// +build !appengine
package tada

import (
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/search"
)

func init() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/getTodo", getTodoHandler)
	http.HandleFunc("/putTodo", putTodoHandler)
}

type TodoItem struct {
	Description string    // Short description of this task -- 1 sentence or less
	DueDate     time.Time // Task due date
	State       string    // "completed" / "incomplete". this is kind of silly but makes it easier to search for completed tasks
}

type TodoID datastore.Key // database key / unique ID for a todo item

type MaybeError interface {
	isMaybeError()
}

type E string
type Ok struct{}
type Matches []TodoItem

func (err E) isMaybeError()           {}
func (ok Ok) isMaybeError()           {}
func (t_item TodoItem) isMaybeError() {}
func (t_id TodoID) isMaybeError()     {}
func (t_id Matches) isMaybeError()    {}

func log(s string) {
	fmt.Printf("%s\n", s)
	// return
}

// TODO: would really be better to statically require that Write returns an item and Read returns an ID

// Takes a task description and a due date, returns a todo item ID
func writeTodoItem(ctx context.Context, description string, dueDate time.Time, state bool) *MaybeError {
	var taskState = "incomplete"
	if state {
		taskState = "completed"
	}
	item := TodoItem{
		Description: description,
		DueDate:     dueDate,
		State:       taskState,
	}
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, "TodoItem", nil), &item)
	var result = new(MaybeError)
	if err != nil {
		log("write error: " + err.Error())
		*result = E(err.Error())
	} else {
		log("write succeeded " + key.String())
		k := TodoID(*key)
		*result = k
		indexResult := indexCommentForSearch(ctx, k)
		switch (*indexResult).(type) {
		case E:
			result = indexResult
		case Ok:
			break
		case TodoID, Matches, TodoItem:
			*result = E("weird answer from indexCommentForSearch")
		}
	}
	return result
}

// Indexes the comment with the specified key for search
func indexCommentForSearch(ctx context.Context, itemID TodoID) *MaybeError {
	index, err := search.Open("tada")
	var result = new(MaybeError)
	if err != nil {
		*result = E(err.Error())
	} else {
		item := readTodoItem(ctx, itemID)
		v := reflect.ValueOf(item)
		switch (*item).(type) {
		case E:
			result = item
		case TodoItem:
			log(fmt.Sprintf("Putting: %s of kind %s and %s", *item, v.Kind(), v.Elem().Kind()))
			titem := (*item).(TodoItem)
			_, err2 := index.Put(ctx, "", &titem)
			if err2 != nil {
				*result = E(err2.Error())
			} else {
				*result = Ok{}
			}
		case TodoID, Matches:
			*result = E("weird response from readTodoItem in indexCommentForSearch")
		}
	}
	return result
}

// Takes a todo item ID, returns a todo item
func readTodoItem(ctx context.Context, itemID TodoID) *MaybeError {
	item := new(TodoItem)
	var err error
	var result = new(MaybeError)
	var key = new(datastore.Key)
	*key = datastore.Key(itemID)
	log("calling Get on: " + (*key).String())
	if err = datastore.Get(ctx, key, item); err != nil {
		log("read failed: " + err.Error())
		*result = E(err.Error())
	} else {
		log("read succeeded with " + item.Description)
		*result = *item
	}
	return (result)
}

// Returns an array of all todo items
func listTodoItems(ctx context.Context, w http.ResponseWriter) *MaybeError {
	var result = new(MaybeError)
	var resultList = make([]TodoItem, 0)
	q := datastore.NewQuery("TodoItem").Order("DueDate")
	_, err := q.GetAll(ctx, &resultList)
	if err != nil {
		//		fmt.Fprintf(w, "listTodoItems got %d keys err = %s", len(keys), err.Error())
		*result = E(err.Error())
	} else {
		//		fmt.Fprintf(w, "got %d items\n", len(keys))
		*result = Matches(resultList) // lookupTodoItems(ctx, keys)
	}
	return result
}

// change a todo item's state from "completed" to "incomplete" or vice versa
func updateTaskState(ctx context.Context, itemID TodoID, completed bool) *MaybeError {
	item := readTodoItem(ctx, itemID)
	var result = new(MaybeError)
	switch (*item).(type) {
	case TodoItem:
		{
			todoItem := (*item).(TodoItem)
			result = writeTodoItem(ctx, todoItem.Description, todoItem.DueDate, completed)
		}
	case E:
		{
			result = item
		}
	case TodoID, Matches, Ok:
		{
			*result = (MaybeError)(E("readTodoItem returned a weird result"))
		}
	}
	return result
}

// Given a list of keys, look up each item
func lookupTodoItems(ctx context.Context, keys []*datastore.Key) *MaybeError {
	var result = new(MaybeError)
	var err error
	var resultList = make([]TodoItem, 0)
	for _, key := range keys {
		var item = new(TodoItem)
		if err = datastore.Get(ctx, key, item); err != nil {
			*result = E(err.Error())
			return result
		}
		resultList = append(resultList, *item)
	}
	*result = Matches(resultList)
	return result
}

// Searches for the string s in *comments* (not short descriptions, for the time being,)
// returns an array of all matching todo item IDs
func searchTodoItems(ctx context.Context, query string) *MaybeError {
	var result = new(MaybeError)
	index, err := search.Open("tada")
	log("Opened the index")
	if err != nil {
		log("Got an error")
		*result = E(err.Error())
	} else {
		log("Got an Index")
		var array = make([]TodoItem, 0, 10)
		log(fmt.Sprintf("array len: %d", len(array)))
		log("About to do search")
		for iter := index.Search(ctx, "Description:"+query, nil); ; {
			log("at head of loop")
			var item TodoItem
			_, err := iter.Next(&item)
			if err == search.Done {
				log("done.")
				break
			}
			if err != nil {
				log("some kind of error")
				*result = E(err.Error())
				return result
			} else {
				array = append(array, item)
				log(fmt.Sprintf("an item! %s", array))

			}
		}
		a := make([]TodoItem, len(array))
		copy(a, array)
		*result = Matches(a)
	}
	return result
}

// Returns true if err != nil
func handleError(w http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// writes the list of existing to-do list arguments
func writeItems(w http.ResponseWriter, r *http.Request) {
	// TODO: use real content
	// and this isn't really right b/c State field in record is now a string
	const todoItem = `<li>
<font color="green">{{.Description}}</font>,
 due on <b><i>{{.DueDate}}</i></b>
<form action="/updateTask" method="post">
  <div><textarea hidden="true" name="description" value="{{.Description}}"></div>
  <div><input hidden="true" type="date" name="dueDate" value="{{.DueDate}}"></div>
  <div><input type="checkbox" name="state" value="{{.State}}"></div>
  <div><input type="submit" value="Save Todo Item"></div>
</form>
</li>
`
	fmt.Fprintf(w, "<!-- in writeItems! -->")

	todoItemT, err := template.New("todoItem").Parse(todoItem)
	if !handleError(w, err) {
		//		fmt.Fprintf(w, "Created template")
		// create AppEngine context
		ctx := appengine.NewContext(r)

		items := listTodoItems(ctx, w)
		//		fmt.Fprintf(w, "Called listTodoItems")
		switch (*items).(type) {
		case Matches:
			{
				itemList := ([]TodoItem)((*items).(Matches))
				//				fmt.Fprintf(w, "Got %d items\n", len(itemList))
				for _, r := range itemList {
					err = todoItemT.Execute(w, r)
					// ignore the return value: if there's an error
					// rendering one item, we still try to render the
					// others
					handleError(w, err)
				}
			}
		case E, TodoItem, TodoID:
			{
				//				fmt.Fprintf(w, "Wrong result from listTodoItems")
				http.Error(w, "Internal error: listTodoItems didn't return a list of items", http.StatusInternalServerError)
			}
		}
	}
}

// writes the form for creating a new todo list item
func makeNewItemForm(w http.ResponseWriter) {
	const form = `
 <form action="/putTodo" method="post">
      <div><textarea name="description" rows="1" cols="100"></textarea></div>
      <div><input type="date" name="dueDate"></div>
      <div><input type="submit" value="Add Todo Item"></div>
    </form>
`
	fmt.Fprint(w, form)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<html><h1>Hi! Welcome to Tada</h1>`)

	fmt.Fprint(w, "<!-- About to call writeItems -->")

	fmt.Fprint(w, `<ol>`)
	writeItems(w, r)
	fmt.Fprint(w, `</ol>`)

	fmt.Fprint(w, "<!-- Called writeItems -->")

	fmt.Fprint(w, `</html>`)

	makeNewItemForm(w)
}

func todoIDFromString(s string) (*int64, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	} else {
		var result = new(int64)
		*result = i
		return result, err
	}

}

func getTodoHandler(w http.ResponseWriter, r *http.Request) {
	// create AppEngine context
	ctx := appengine.NewContext(r)

	// get id from request
	id := r.FormValue("id")
	// read a TodoItem from Datastore
	i, err := todoIDFromString(id)
	if err != nil {
		http.Error(w, "You asked for a todo item that isn't a valid ID: "+id,
			400)
	} else {
		k := datastore.NewKey(ctx, "Entity", "intID", *i, nil)
		item := readTodoItem(ctx, TodoID(*k))
		respondWith(w, *item)
	}
}

func putTodoHandler(w http.ResponseWriter, r *http.Request) {
	// create AppEngine context
	ctx := appengine.NewContext(r)

	// get description from request
	description := r.FormValue("description")
	// get due date from request
	dueDate := r.FormValue("dueDate")
	d, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		http.Error(w, dueDate+" doesn't look like a valid date to me!",
			400)
	} else {
		id := writeTodoItem(ctx, description, d, false)
		respondWith(w, *id)
	}
}

func respondWith(w http.ResponseWriter, result MaybeError) {
	switch result.(type) {
	case E:
		// error
		http.Error(w, string(result.(E)), 500)
	case TodoID:
		// we successfully wrote the item
		fmt.Fprintf(w, "Successfully saved to-do item!")
	case TodoItem:
		// show the looked-up item
		item := result.(TodoItem)
		fmt.Fprintf(w, "item: %s due %d", item.Description, item.DueDate)
	}
}

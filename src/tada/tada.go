// +build !appengine
package tada

import (
	"fmt"
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

// testing only
var defaultTask = TodoItem{Description: "test item", DueDate: time.Now()}

func log(s string) {
	fmt.Printf("%s\n", s)
	// return
}

// TODO: would really be better to statically require that Write returns an item and Read returns an ID

// Takes a task description and a due date, returns a todo item ID
func writeTodoItem(ctx context.Context, description string, dueDate time.Time) *MaybeError {
	item := TodoItem{
		Description: description,
		DueDate:     dueDate,
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "I don't know what you want!")
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
	d, err := time.Parse("2001-01-01", dueDate)
	if err != nil {
		http.Error(w, dueDate+" doesn't look like a valid date to me!",
			400)
	} else {
		id := writeTodoItem(ctx, description, d)
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

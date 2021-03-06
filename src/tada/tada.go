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
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/search"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/user"
)

func init() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/getTodo", getTodoHandler)
	http.HandleFunc("/putTodo", putTodoHandler)
	http.HandleFunc("/updateTask", updateTaskHandler)
}

type TodoItem struct {
	OwnerEmail  string    // email address of the user who created this item
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
type CacheMiss struct{}

// Used for returning stuff from listTodoItems.
// Keep the keys and values separate so as not to add a Key field to the item struct
type Match struct {
	Key   *datastore.Key
	Value TodoItem
}
type Matches []Match
type SearchResults []TodoItem
type Blob []byte

func (err E) isMaybeError()              {}
func (ok Ok) isMaybeError()              {}
func (c CacheMiss) isMaybeError()        {}
func (t_item TodoItem) isMaybeError()    {}
func (t_id TodoID) isMaybeError()        {}
func (t_id Matches) isMaybeError()       {}
func (t_id SearchResults) isMaybeError() {}
func (t_id Blob) isMaybeError()          {}

func log(s string) {
	fmt.Printf("%s\n", s)
	// return
}

// TODO: would really be better to statically require that Write returns an item and Read returns an ID

// Takes a task description and a due date, returns a todo item ID
// user is a separate argument for testing reasons
// Adds a reminder iff remind is true
func writeTodoItem(ctx context.Context, description string, dueDate time.Time, state bool, u *user.User, remind bool) *MaybeError {
	var taskState = "incomplete"
	if state {
		taskState = "completed"
	}
	item := TodoItem{
		Description: description,
		DueDate:     dueDate,
		State:       taskState,
		OwnerEmail:  u.Email,
	}
	key, err := datastore.Put(ctx, datastore.NewIncompleteKey(ctx, "TodoItem", nil), &item)
	log(fmt.Sprintf("WRITE: key = %s", key))
	var result = new(MaybeError)
	if err != nil {
		log("write error: " + err.Error())
		*result = E(err.Error())
	} else {
		log("write succeeded " + key.String())
		k := TodoID(*key)
		// FIXME: should check the results of invalidate calls
		invalidateCache(ctx, *key)
		*result = k
		indexResult := indexCommentForSearch(ctx, k)
		switch (*indexResult).(type) {
		case E:
			return indexResult
		case Ok:
			break
		case TodoID, Matches, TodoItem:
			*result = E("weird answer from indexCommentForSearch")
		}
		if !state && remind {
			queueResult := addReminder(ctx, item)
			switch (*queueResult).(type) {
			case E:
				return queueResult
			case Ok:
				break
			case TodoID, Matches, TodoItem:
				*result = E("weird answer from addReminder")
			}
		}
	}
	return result
}

// Takes a task description and a due date, along with an id, returns OK or an error
func updateTodoItem(ctx context.Context, email string, description string, dueDate time.Time, state bool, id int64) *MaybeError {
	var taskState = "incomplete"
	if state {
		taskState = "completed"
	}
	item := TodoItem{
		OwnerEmail:  email,
		Description: description,
		DueDate:     dueDate,
		State:       taskState,
	}
	k := datastore.NewKey(ctx,
		"TodoItem",
		"",
		id,
		nil)
	key, err := datastore.Put(ctx, k, &item)
	log(fmt.Sprintf("UPDATE: key = %s", key))
	var result = new(MaybeError)
	if err != nil {
		log("update error: " + err.Error())
		*result = E(err.Error())
	} else {
		log("update succeeded " + key.String())
		// n.b. This updateCache call is necessary for consistency
		// because otherwise, a successive call to listTodoItems might not be
		// consistent with the results of this call to update
		updateCache(ctx, *key, item)
		indexResult := indexCommentForSearch(ctx, TodoID(*key))
		switch (*indexResult).(type) {
		case E:
			result = indexResult
		case Ok:
			*result = Ok{}
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
	// n.b. doesn't check the owner
	item := new(TodoItem)
	var err error
	var result = new(MaybeError)
	var key = new(datastore.Key)
	*key = datastore.Key(itemID)
	log("calling Get on: " + (*key).String())
	maybeCached := lookupCache(ctx, *key)
	if (*maybeCached != CacheMiss{}) {
		// item was cached, return it
		return maybeCached
	}

	if err = datastore.Get(ctx, key, item); err != nil {
		log("read failed: " + err.Error())
		*result = E(err.Error())
	} else {
		log("read succeeded with " + item.Description)
		*result = *item
	}
	updateCache(ctx, *key, *item)
	return (result)
}

// Returns an array of all todo items
func listTodoItems(ctx context.Context, u *user.User) *MaybeError {
	var result = new(MaybeError)
	var resultList = make([]TodoItem, 0)
	// filter by user
	log(fmt.Sprintf("Making query, email = %s", u.Email))

	q := datastore.NewQuery("TodoItem").Filter("OwnerEmail=", u.Email).Order("DueDate")
	keys, err := q.GetAll(ctx, &resultList)
	if err != nil {
		log(fmt.Sprintf("listTodoItems got %d keys err = %s", len(keys), err.Error()))
		*result = E(err.Error())
	} else {
		log(fmt.Sprintf("got %d items %s [%s] / [%s]\n", len(keys), u.Email, keys, resultList))
		// *assuming* these are going to be in the same order...
		var matches = make([]Match, 0)
		// this is a bit silly since we already did the database query, but...
		// if we call readTodoItem we get the more-recent item in the cache
		for _, k := range keys {
			anItem := readTodoItem(ctx, TodoID(*k))
			switch (*anItem).(type) {
			case TodoItem:
				matches = append(matches, Match{k, (*anItem).(TodoItem)})
			default:
				*result = (E("readTodoItem returned weird result in listTodoItems"))
				return result
			}
		}
		*result = Matches(matches)
	}
	return result
}

// Searches for the string s in *comments* (not short descriptions, for the time being,)
// returns an array of all matching todo items
// note: it would probably be more useful to be able to go to the actual todo item
// so you can edit it, but ignoring that for now
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
		*result = SearchResults(a)
	}
	return result
}

// Adds a reminder with the given text and due date to the pull queue.
// A reminder will be sent half an hour before the due date
func addReminder(ctx context.Context, item TodoItem) *MaybeError {
	maybeBlob := itemToJson(item)
	switch (*maybeBlob).(type) {
	case Blob:
		{
			item1 := ([]byte)((*maybeBlob).(Blob))
			t := &taskqueue.Task{
				Payload: []byte(item1),
				Method:  "PULL",
			}
			_, err := taskqueue.Add(ctx, t, "reminders")
			if err != nil {
				var result = new(MaybeError)
				*result = E(err.Error())
				return result
			}
		}
	case E:
		{
			return maybeBlob
		}
	case TodoItem, Matches, TodoID:
		{
			var result = new(MaybeError)
			*result = E("strange result from JSON encoder")
			return result
		}
	}
	var result = new(MaybeError)
	*result = Ok{}
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
func writeItems(w http.ResponseWriter, r *http.Request, u *user.User) {
	var (
		funcMap = template.FuncMap{
			"Equal":   func(a, b string) bool { return a == b },
			"FmtDate": func(d time.Time) string { return d.Format("2006-01-02") },
			"FmtKey":  func(k datastore.Key) int64 { return k.IntID() },
		}
	)

	const todoItem = `<li>{{if Equal .Value.State "completed"}}<strike>{{else}}{{end}}
<font color="green">{{.Value.Description}}</font>,
due on <b><i>{{.Value.DueDate}}</i></b>
{{if Equal .Value.State "completed"}}</strike>{{else}}{{end}}
 <form action="/updateTask" method="post">
<p style="border-style:groove;border-width:3px;border-color:pink">
   <textarea name="description">{{.Value.Description}}</textarea>
   <input type="date" name="dueDate" value="{{FmtDate .Value.DueDate}}">
   <input type="checkbox" name="state" {{if Equal .Value.State "completed"}}checked{{else}}{{end}}>
   <input hidden=true name="id" value={{FmtKey .Key}}>
   <input type="submit" value="Save Todo Item">
</p>
 </form>
</li>
` // However, the record has no ItemId field...

	fmt.Fprintf(w, "<!-- in writeItems! -->")

	todoItemT, err := template.New("todoItem").Funcs(funcMap).Parse(todoItem)
	if !handleError(w, err) {
		//		fmt.Fprintf(w, "Created template")
		// create AppEngine context
		ctx := appengine.NewContext(r)

		items := listTodoItems(ctx, user.Current(ctx))
		//		fmt.Fprintf(w, "Called listTodoItems")
		switch (*items).(type) {
		case Matches:
			{
				itemList := ([]Match)((*items).(Matches))
				//				fmt.Fprintf(w, "Got %d items\n", len(itemList))
				for _, r := range itemList {
					//					fmt.Fprintf(w, "Item: %", r)
					err = todoItemT.Execute(w, r)
					// ignore the return value: if there's an error
					// rendering one item, we still try to render the
					// others
					handleError(w, err)
				}
			}
		case TodoItem, TodoID:
			{
				fmt.Fprintf(w, "Wrong result from listTodoItems")
				http.Error(w, "Internal error: listTodoItems didn't return a list of items", http.StatusInternalServerError)
			}
		case E:
			{
				http.Error(w, ((string)((*items).(E))), http.StatusInternalServerError)
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
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		url, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in or register</a>`, url)
		return
	}

	fmt.Fprint(w, `<html><h1>Hi! Welcome to Tada</h1>`)

	fmt.Fprint(w, "<!-- About to call writeItems -->")

	fmt.Fprint(w, `<ol>`)
	writeItems(w, r, u)
	fmt.Fprint(w, `</ol>`)

	fmt.Fprint(w, "<!-- Called writeItems -->")

	url, _ := user.LogoutURL(ctx, "/")
	fmt.Fprintf(w, `Welcome, %s! (<a href="%s">sign out</a>)`, u, url)

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
		id := writeTodoItem(ctx, description, d, false, user.Current(ctx), true)
		respondWith(w, *id)
	}
}

// The same as putTodoHandler, but it expects there to be an "id" parameter.
// It then writes a new record with that id, overwriting the old one.
func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	// create AppEngine context
	ctx := appengine.NewContext(r)

	// get description from request
	description := r.FormValue("description")
	// get due date from request
	dueDate := r.FormValue("dueDate")
	d, err := time.Parse("2006-01-02", dueDate)
	// get item ID from request
	id := r.FormValue("id")
	itemID, err1 := strconv.ParseInt(id, 10, 64)
	// get user from logged-in user
	email := user.Current(ctx).Email
	if err != nil {
		http.Error(w, dueDate+" doesn't look like a valid date to me!",
			400)
	} else if err1 != nil {
		http.Error(w, id+" doesn't look like an item ID to me!",
			400)
	} else {
		state := r.FormValue("state")
		respondWith(w, *(updateTodoItem(ctx,
			email,
			description,
			d,
			state == "on",
			itemID)))
		rootHandler(w, r)
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

/*
originally I was caching the entire todo list for a user as a block so I wouldn't
have to fetch the todo entries individually, but this was awkward since when one
item changes, the cached list would have to be modified
*/

func invalidateCache(ctx context.Context, key datastore.Key) *MaybeError {
	// delete key from memcache
	err := memcache.Delete(ctx, key.String())
	var result = new(MaybeError)
	if err != nil {
		*result = E(err.Error())
	} else {
		*result = Ok{}
	}
	return result
}

func lookupCache(ctx context.Context, key datastore.Key) *MaybeError {
	maybeItem, err := memcache.Get(ctx, key.String())
	var result = new(MaybeError)
	if err != nil { // treat all errors as "cache miss"
		*result = CacheMiss{}
	} else {
		result = jsonToTodoItem(maybeItem.Value)
	}
	return result
}

func updateCache(ctx context.Context, key datastore.Key, item TodoItem) {
	var result = itemToJson(item)
	switch (*result).(type) {
	case Blob:
		blob := ([]byte)((*result).(Blob))
		item := memcache.Item{
			Key:   key.String(),
			Value: blob,
		}
		memcache.Set(ctx, &item) // ignore errors... worst that can happen is we get a cache miss later
	default:
		break
	}
}

/*
Ran into an interesting eventual consistency problem b/c at first, I was caching todo list items and entire todo lists separately, and invalidating the cache for both after an individual item was changed.
This wasn't good enough b/c without updating memcache, the next listTodoItems call wouldn't see the result of the update.
Changed it to only cache individual items and update the memcache manually after every update
*/

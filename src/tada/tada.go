// +build !appengine
package tada

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

func init() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/getTodo", getTodoHandler)
	http.HandleFunc("/putTodo", putTodoHandler)
}

type TodoItem struct {
	Description string    // Description of this task
	DueDate     time.Time // Task due date
}

type TodoID datastore.Key // database key / unique ID for a todo item

type MaybeError interface {
	isMaybeError()
}

type E string

func (err E) isMaybeError()           {}
func (t_item TodoItem) isMaybeError() {}
func (t_id TodoID) isMaybeError()     {}

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
		*result = TodoID(*key)
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "I don't know what you want!")
}

func getTodoHandler(w http.ResponseWriter, r *http.Request) {
	// create AppEngine context
	ctx := appengine.NewContext(r)

	// get id from request
	id := r.FormValue("id")
	// read a TodoItem from Datastore
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		http.Error(w, "You asked for a todo item that isn't a valid ID: "+id,
			400)
	} else {
		k := datastore.NewKey(ctx, "Entity", "intID", i, nil)
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

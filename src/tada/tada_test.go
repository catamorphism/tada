package tada

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func assert(t *testing.T, v bool, error string) {
	if !v {
		t.Errorf("Assertion failed: %s", error)
	}
}

func assertEquals(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Expected %s but was %s", expected, actual)
	}
}

func TestKeyComplete(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	k := writeTodoItem(ctx, "hello", dueDate, false)
	switch (*k).(type) {
	case TodoID:
		k1 := (*k).(TodoID)
		k2 := datastore.Key(k1)
		assert(t, !k2.Incomplete(), "write returned an incomplete key")
	case E, TodoItem:
		t.Fatal(fmt.Sprintf("Expected write to return a todo ID, got something else: %s", *k))
	}

	defer done()
}

func TestReadAfterWrite(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	itemId := writeTodoItem(ctx, "finish writing these tests", dueDate, false)
	switch (*itemId).(type) {
	case TodoID:
		theTodo := readTodoItem(ctx, (*itemId).(TodoID))
		switch (*theTodo).(type) {
		case TodoItem:
			theItem := (*theTodo).(TodoItem)
			assertEquals(t, theItem.Description, "finish writing these tests")
			assertEquals(t, theItem.DueDate.String(), dueDate.Local().String())
		case E, TodoID:
			t.Fatal("Expected read to return a todo item, got ", *theTodo)
		}
	case E, TodoItem:
		t.Fatal("Expected write to return a todo ID, got something else")
	}

	defer done()

}

func TestTextSearch(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	writeTodoItem(ctx, "phone up my friend", dueDate, false)
	writeTodoItem(ctx, "buy a new phone", dueDate, false)
	writeTodoItem(ctx, "feed the fish", dueDate, false)
	queryResults := searchTodoItems(ctx, "phone")
	switch (*queryResults).(type) {
	case SearchResults:
		items := ([]TodoItem)((*queryResults).(SearchResults))
		assert(t, len(items) == 2, "wrong number of search results")
		// I don't know if the order is deterministic, but *shrug*
		assert(t, items[0].Description == "buy a new phone", "wrong first task: found "+items[0].Description)
		assert(t, items[1].Description == "phone up my friend", "wrong second task: found "+items[1].Description)
	case E, TodoID, TodoItem, Matches:
		t.Fatal("Didn't get a SearchResults result from a search")
	}

	defer done()
}

func TestListTodo(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	writeTodoItem(ctx, "phone up my friend", dueDate, false)
	writeTodoItem(ctx, "buy a new phone", dueDate, false)
	writeTodoItem(ctx, "feed the fish", dueDate, false)
	listResults := listTodoItems(ctx)
	switch (*listResults).(type) {
	case Matches:
		items := ([]Match)((*listResults).(Matches))
		assert(t, len(items) == 3, "wrong number of todo items")
		// I don't know if the order is deterministic, but *shrug*
		assert(t, items[0].Value.Description == "buy a new phone", "wrong first task")
		assert(t, items[1].Value.Description == "phone up my friend", "wrong second task")
		assert(t, items[2].Value.Description == "feed the fish", "wrong third task")
	case E, TodoID, TodoItem:
		t.Fatal("Didn't get a Matches result from listTodoItems")
	}
	defer done()
}

func TestUpdate(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	id := writeTodoItem(ctx, "phone up my friend", dueDate, false)
	switch (*id).(type) {
	case TodoID:
		id1 := ((datastore.Key)((*id).(TodoID)))
		result := updateTodoItem(ctx, "phone up my friend", dueDate, true, id1.IntID())
		switch (*result).(type) {
		case Ok:
		case Matches, E, TodoID, TodoItem:
			t.Fatal("Non-OK result from updateTodoItem")
		}
		item := readTodoItem(ctx, TodoID(id1))
		switch (*item).(type) {
		case TodoItem:
			item1 := (*item).(TodoItem)
			assert(t, item1.Description == "phone up my friend", "wrong description")
			assert(t, item1.DueDate == dueDate.Local(), fmt.Sprintf("wrong date: expected %s, found %s", dueDate, item1.DueDate))
			assert(t, item1.State == "completed", "expected to be completed, saw incompleted")
		case E, TodoID, Matches:
			t.Fatal("Didn't get a TodoItem result from readTodoItem")
		}
	case E, TodoItem, Matches:
		t.Fatal("Didn't get a TodoID result from writeTodoItem")
	}
	defer done()
}

package tada

import (
	"testing"
	"time"

	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func assert(t *testing.T, v bool, error string) {
	if !v {
		t.Errorf("Assertion failed: ", error)
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
	k := writeTodoItem(ctx, "hello", dueDate)
	switch (*k).(type) {
	case TodoID:
		k1 := (*k).(TodoID)
		k2 := datastore.Key(k1)
		assert(t, !k2.Incomplete(), "write returned an incomplete key")
	case E, TodoItem:
		t.Fatal("Expected write to return a todo ID, got something else")
	}

	defer done()
}

func TestReadAfterWrite(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	itemId := writeTodoItem(ctx, "finish writing these tests", dueDate)
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

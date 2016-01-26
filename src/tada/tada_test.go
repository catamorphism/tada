package tada

import (
	"testing"
	"time"

	"google.golang.org/appengine/aetest"
)

func assertEquals(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Expected %s but was %s", expected, actual)
	}
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
			assertEquals(t, theItem.description, "finish writing these tests")
			assertEquals(t, theItem.dueDate.String(), dueDate.String())
		case E, TodoID:
			t.Fatal("Expected read to return a todo item, got ", *theTodo)
		}
	case E, TodoItem:
		t.Fatal("Expected write to return a todo ID, got something else")
	}

	defer done()

}

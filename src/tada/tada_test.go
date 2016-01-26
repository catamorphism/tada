package tada

import (
	"testing"
	"time"
)

func TestReadAfterWrite(t *testing.T) {
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, location.UTC)
	itemId := writeTodoItem("finish writing these tests", dueDate)
	theTodo := readTodoItem(itemId)
	assertEquals(theTodo.description, "finish writing these tests")
	assertEquals(theTodo.dueDate, dueDate)
}

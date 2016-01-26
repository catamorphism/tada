package tada

import (
	"fmt"
	"net/http"
	"time"
)

func init() {
	http.HandleFunc("/", handler)
}

type TodoItem struct {
	description string    // Description of this task
	dueDate     time.Time // Task due date
}

type TodoID int // database key / unique ID for a todo item

// testing only
var defaultTask = TodoItem{description: "test item", dueDate: time.Now()}

// Takes a task description and a due date, returns a todo item ID
func writeTodoItem(description string, dueDate time.Time) TodoID {
	return 0 // stub
}

// Takes a todo item ID, returns a todo item
func readTodoItem(itemID TodoID) TodoItem {
	return defaultTask // stub
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Tada!")
}

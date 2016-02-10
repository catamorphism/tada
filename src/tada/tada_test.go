package tada

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
	//	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/user"
)

var testUser = user.User{Email: "alice@example.com"}
var testUser1 = user.User{Email: "bob@example.com"}

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
	k := writeTodoItem(ctx, "hello", dueDate, false, &testUser, false)
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
	itemId := writeTodoItem(ctx, "finish writing these tests", dueDate, false, &testUser, false)
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

	writeTodoItem(ctx, "phone up my friend", dueDate, false, &testUser, false)
	writeTodoItem(ctx, "buy a new phone", dueDate, false, &testUser, false)
	writeTodoItem(ctx, "feed the fish", dueDate, false, &testUser, false)
	queryResults := searchTodoItems(ctx, "phone")
	switch (*queryResults).(type) {
	case SearchResults:
		items := ([]TodoItem)((*queryResults).(SearchResults))
		assert(t, len(items) == 2, "wrong number of search results")
		// order is *not* deterministic
		assert(t, items[0].Description == "buy a new phone" || items[1].Description == "buy a new phone", "neither task had description 'buy a new phone'")
		assert(t, items[0].Description == "phone up my friend" || items[1].Description == "phone up my friend", "neither task had description 'phone up my friend'")
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

	writeTodoItem(ctx, "phone up my friend", dueDate, false, &testUser, false)
	writeTodoItem(ctx, "buy a new phone", dueDate, false, &testUser, false)
	writeTodoItem(ctx, "feed the fish", dueDate, false, &testUser, false)
	listResults := listTodoItems(ctx, &testUser)
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

// write 1 todo item
// list todo items: should be 1 item
// write another todo item
// list todo items again: should be 2 items
func TestListWriteList(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	writeTodoItem(ctx, "phone up my friend", dueDate, false, &testUser, false)
	listResults := listTodoItems(ctx, &testUser)
	switch (*listResults).(type) {
	case Matches:
		items := ([]Match)((*listResults).(Matches))
		assert(t, len(items) == 1, fmt.Sprintf("wrong number of todo items: expected 1, saw %d", len(items)))
		writeTodoItem(ctx, "buy a new phone", dueDate, false, &testUser, false)
		listResults1 := listTodoItems(ctx, &testUser)
		switch (*listResults1).(type) {
		case Matches:
			items1 := ([]Match)((*listResults1).(Matches))
			assert(t, len(items1) == 2, fmt.Sprintf("wrong number of todo items: expected 2, saw %d", len(items1)))
		default:
			t.Fatal(fmt.Sprintf("weird result from second listTodoItems: %s", *listResults1))
		}
	default:
		t.Fatal("weird result from listTodoItems")
	}

	defer done()

}

func TestUpdate(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	id := writeTodoItem(ctx, "phone up my friend", dueDate, false, &testUser, false)
	switch (*id).(type) {
	case TodoID:
		id1 := ((datastore.Key)((*id).(TodoID)))
		result := updateTodoItem(ctx, testUser.Email, "phone up my friend", dueDate, true, id1.IntID())
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
			assert(t, item1.DueDate == dueDate, fmt.Sprintf("wrong date: expected %s, found %s [%s] {%s}", dueDate, item1.DueDate, dueDate.Sub(item1.DueDate), dueDate == item1.DueDate))
			assert(t, item1.State == "completed", "expected to be completed, saw incompleted")
		case E, TodoID, Matches:
			t.Fatal("Didn't get a TodoItem result from readTodoItem")
		}
	case E, TodoItem, Matches:
		t.Fatal("Didn't get a TodoID result from writeTodoItem")
	}
	defer done()
}

// Not sure how to test this one or if there's a way to test task queues
/*
// actually want to do auth first
func TestEmailReminder(t *testing.T) {
	// Add a new todo item. Check that there's an item in the pull queue
	// with the correct due date.
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	id := writeTodoItem(ctx, "water my cactus", dueDate, false, &testUser, false)
	switch (*id).(type) {
	case TodoID:
		// ((datastore.Key)((*id).(TodoID)))
		tasks, err := taskqueue.Lease(ctx, 1, "reminders", 3600)
		assert(t, err == nil, "error pulling tasks from queue")
		assert(t, len(tasks) == 1, "wrong number of tasks in queue")
	case E, TodoItem, Matches:
		t.Fatal("Didn't get a TodoID result from writeTodoItem")
	}
	defer done()
}
*/

func assertList(t *testing.T, x MaybeError) []Match {
	switch x.(type) {
	case Matches:
		{
			return ([]Match)(x.(Matches))
		}
	default:
		{
			t.Fail()
			return nil
		}
	}
}

func TestNoInterference(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	writeTodoItem(ctx, "Brush my teeth", dueDate, false, &testUser, false)
	writeTodoItem(ctx, "Brush my dog", dueDate, false, &testUser1, false)
	l := listTodoItems(ctx, &testUser)
	l1 := listTodoItems(ctx, &testUser1)
	aliceItems := assertList(t, *l)
	bobItems := assertList(t, *l1)
	assert(t, len(aliceItems) == 1, fmt.Sprintf("Alice's todolist has the wrong length: %d", len(aliceItems)))
	assert(t, len(bobItems) == 1, fmt.Sprintf("Bob's todolist has the wrong length: %d", len(bobItems)))
	if len(aliceItems) == 1 && len(bobItems) == 1 {
		assert(t, aliceItems[0].Value.Description == "Brush my teeth", "Wrong item in Alice's todo list")
		assert(t, aliceItems[0].Value.OwnerEmail == testUser.Email, "Wrong item owner in Alice's todo list")
		assert(t, bobItems[0].Value.Description == "Brush my dog", "Wrong item in Bob's todo list")
		assert(t, bobItems[0].Value.OwnerEmail == testUser1.Email, "Wrong item owner in Bob's todo list")
	}
	defer done()
}

// Apparently there's no way to test task queues? https://code.google.com/p/googleappengine/issues/detail?id=10771

// No, it does seem to work: //depot/google3/third_party/golang/appengine/taskqueue/taskqueue_test.go
// no, that's only for internal AppEngine testing. "taskqueue execution is not supported in the test environment." Whatever.

// read an item; check that the item is in the cache
func TestMemcacheRead(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)

	id := writeTodoItem(ctx, "Brush my teeth", dueDate, false, &testUser, false)
	switch (*id).(type) {
	case TodoID:
		id1 := (*id).(TodoID)
		item := readTodoItem(ctx, id1)
		switch (*item).(type) {
		case TodoItem:
			read_item := (*item).(TodoItem)
			k := datastore.Key(id1)
			cache_item, err := memcache.Get(ctx, k.String())
			if err != nil {
				t.Fatal("memcache error")
			}
			cache_value := jsonToTodoItem(cache_item.Value)
			switch (*cache_value).(type) {
			case TodoItem:
				cached_item := (*cache_value).(TodoItem)
				assert(t, read_item == cached_item, "Cached todo item differs from the original item")
			default:
				t.Fatal("memcache.Get returned a weird result")
			}
		default:
			t.Fatal("readTodoItem returned a weird result")
		}
	default:
		t.Fatal("writeTodoItem returned a weird result")
	}
	defer done()
}

// write a new item; read it back; modify the due date; check that we see the change
func TestMemcacheUpdate(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	dueDate := time.Date(2016, 2, 29, 13, 0, 0, 0, time.UTC)
	dueDate1 := time.Date(2016, 3, 12, 13, 0, 0, 0, time.UTC)

	id := writeTodoItem(ctx, "Brush my teeth", dueDate, false, &testUser, false)
	switch (*id).(type) {
	case TodoID:
		id1 := (*id).(TodoID)
		item := readTodoItem(ctx, id1)
		t.Log("item = ", *item)
		switch (*item).(type) {
		case TodoItem:
			int_id := datastore.Key(id1)
			updateTodoItem(ctx, testUser.Email, "Brush my teeth", dueDate1, false, int_id.IntID())
			cached_value, err := memcache.Get(ctx, int_id.String())
			if err != nil {
				t.Fatal("memcache.Get returned a weird result")
			}
			parsed_value := jsonToTodoItem(cached_value.Value)
			switch (*parsed_value).(type) {
			case TodoItem:
				cached_item := (*parsed_value).(TodoItem)
				assert(t,
					cached_item.DueDate == dueDate1,
					"cached item due date doesn't reflect update")
			default:
				t.Fatal("error encoding memcached item")
			}
		default:
			t.Fatal("readTodoItem returned a weird result: ", *item)
		}
	default:
		t.Fatal("writeTodoItem returned a weird result")
	}

	defer done()
}

// +build !appengine
package tada

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func itemToJson(item TodoItem) *MaybeError {
	b := new(bytes.Buffer)
	e := json.NewEncoder(b)
	err := e.Encode(item)
	if err != nil {
		var result = new(MaybeError)
		*result = E("error trying to encode item")
		return result
	}
	var result = new(MaybeError)
	*result = Blob(b.Bytes())
	return result
}

func matchesToJson(items Matches) *MaybeError {
	b := new(bytes.Buffer)
	e := json.NewEncoder(b)
	err := e.Encode(items)
	if err != nil {
		var result = new(MaybeError)
		*result = E("error trying to encode matches")
		return result
	}
	var result = new(MaybeError)
	*result = Blob(b.Bytes())
	return result
}

func jsonToTodoItem(blob []byte) *MaybeError {
	d := json.NewDecoder(bytes.NewReader(blob))
	var item = new(TodoItem)
	var result = new(MaybeError)
	err := d.Decode(&item)
	if err != nil {
		log(fmt.Sprintf("jsonToTodoItem: returning ", err.Error()))
		*result = E(err.Error())
	} else {
		log(fmt.Sprintf("jsonToDoItem: returning item owner=(%s) desc=(%s) due=(%s) ", item.OwnerEmail, item.Description, item.DueDate))
		*result = *item
	}
	return result
}

func jsonToMatches(blob []byte) *MaybeError {
	d := json.NewDecoder(bytes.NewReader(blob))
	var items = new(Matches)
	var result = new(MaybeError)
	err := d.Decode(&items)
	if err != nil {
		*result = E(err.Error())
	} else {
		*result = *items
	}
	return result
}

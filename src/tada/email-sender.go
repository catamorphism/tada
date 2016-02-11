// +build !appengine
package tada

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/mail"
	"google.golang.org/appengine/runtime"
	"google.golang.org/appengine/taskqueue"
)

func init() {
	http.HandleFunc("/_ah/start", startPoller)
}

func startPoller(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	err := runtime.RunInBackground(ctx, poller)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// Checks the task queue every half-hour
func poller(ctx context.Context) {
	for true {
		tasks, err := taskqueue.Lease(ctx, 1, "reminders", 3600)
		if err != nil {
			// not sure what to do with errors
		} else {
			if len(tasks) > 0 {
				sendOneReminder(ctx, tasks[0])
			}
		}
		time.Sleep(time.Minute * 30)
	}
}

func sendOneReminder(ctx context.Context, t *taskqueue.Task) {
	// decode todo item
	todoItem_ := jsonToTodoItem(t.Payload)
	switch (*todoItem_).(type) {
	case TodoItem:
		{
			todoItem := (*todoItem_).(TodoItem)
			if reminderDue(todoItem) {
				// send email reminder
				// note: this doesn't handle the case where a task gets complete in between
				// when it's enqueued and when the reminder is due to be sent
				if sendReminderEmail(ctx,
					todoItem.OwnerEmail,
					todoItem.Description,
					todoItem.DueDate) {
					err := taskqueue.Delete(ctx, t, "reminders")
					if err != nil {
						// ???
					}
				}
				// if sending the reminder failed, we just leave it in the queue
			} else {
				err := taskqueue.ModifyLease(ctx, t, "reminders", 0)
				if err != nil {
					// ??
				}
			}
		}
	default:
		// ignore errors, for now
		break
	}
}

func reminderDue(todoItem TodoItem) bool {
	now := time.Now()
	// returns true if it's less than an hour before the due date
	return (todoItem.DueDate.Local().Sub(now).Minutes() <= 60)
}

func sendReminderEmail(ctx context.Context,
	email string,
	description string,
	dueDate time.Time) bool {
	msg := &mail.Message{
		Sender:  "Tada <tada@tada-1202.appspotmail.com>",
		To:      []string{email},
		Subject: fmt.Sprintf("[Tada reminder] %s", description),
		Body: fmt.Sprintf(`This is a friendly reminder about your task: \n
                       %s \n
                       (due %s)\n`, description, dueDate),
	}
	return (mail.Send(ctx, msg) == nil)
}

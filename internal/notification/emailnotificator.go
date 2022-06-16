package notification

import "fmt"

type EmailNotificator struct {
	// business stuff
}

func (en *EmailNotificator) Send(notification ServerNotification) {
	// business logic regarding email notifications
	fmt.Println("Sent an Email Notification")
}

package notification

import "fmt"

type SMSNotificator struct {
	// business stuff
}

func (smsn *SMSNotificator) Send(notification ServerNotification) {
	// business logic regarding sms notifications
	fmt.Println("Sent an SMS Notification")
}

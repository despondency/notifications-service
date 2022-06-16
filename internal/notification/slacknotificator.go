package notification

import "fmt"

type SlackNotificator struct {
	// business stuff
}

func (sn *SlackNotificator) Send(notification ServerNotification) {
	// business logic regarding slack notifications
	fmt.Println("Sent an Slack Notification")
}

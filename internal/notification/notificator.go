package notification

type Sender interface {
	Send(notification ServerNotification) error
}

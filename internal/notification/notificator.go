package notification

import (
	"github.com/google/uuid"
)

type Notificator interface {
	Send(txt string) error
	Destination() Destination
}

type DelegatingNotification struct {
	serverUUID uuid.UUID
	txt        string
	dest       Destination
}

type DelegatingNotificator struct {
	Notificators []Notificator
}

func (dn *DelegatingNotificator) DelegateNotification(notification *DelegatingNotification) error {
	for _, n := range dn.Notificators {
		if n.Destination() == notification.dest {
			err := n.Send(notification.txt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

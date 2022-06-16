package notification

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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
			log.Debug().Msg(fmt.Sprintf("successfully sent notification with uuid %s, to %d", notification.serverUUID, notification.dest))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

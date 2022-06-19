package notification

import (
	"fmt"
	"github.com/rs/zerolog/log"
)

type SlackNotificator struct {
	// business stuff
}

func (sn *SlackNotificator) Send(txt string) error {
	// business logic regarding slack notifications
	log.Info().Msg(fmt.Sprintf("Sent an Slack NotificationRequest with txt %s", txt))
	return nil
}

func (sn *SlackNotificator) Destination() Destination {
	return Slack
}

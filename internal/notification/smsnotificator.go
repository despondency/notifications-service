package notification

import (
	"fmt"
	"github.com/rs/zerolog/log"
)

type SMSNotificator struct {
	// business stuff
}

func (smsn *SMSNotificator) Send(txt string) error {
	// business logic regarding sms notifications
	log.Info().Msg(fmt.Sprintf("Sent an SMS NotificationRequest with txt %s", txt))
	return nil
}

func (sn *SMSNotificator) Destination() Destination {
	return SMS
}

package notification

type EmailNotificator struct {
	// business stuff
}

func (en *EmailNotificator) Send(txt string) error {
	// business logic regarding email notifications
	//log.Info().Msg(fmt.Sprintf("Sent an Email Notification with txt %s", txt))
	return nil
}

func (sn *EmailNotificator) Destination() Destination {
	return Email
}

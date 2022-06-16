package notification

import (
	"fmt"
	"github.com/google/uuid"
)

type Destination int8

const (
	SMS Destination = iota
	Email
	Slack
)

var ErrNoSuchDestination = fmt.Errorf("no such destination exists")

type Status int8

const (
	NOT_PROCESSED Status = iota
	PROCESSED
)

func toServerNotificationDestination(destination string) (int, error) {
	switch destination {
	case "SMS":
		return 0, nil
	case "EMAIL":
		return 1, nil
	case "SLACK":
		return 2, nil
	default:
		return -1, ErrNoSuchDestination
	}
}

type ServerNotification struct {
	serverUUID      uuid.UUID
	serverTimestamp int64
	notificationTxt string
	dest            Destination
}

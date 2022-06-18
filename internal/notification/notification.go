package notification

import (
	"fmt"
	"github.com/google/uuid"
	"time"
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

func toServerNotificationDestination(destination string) (Destination, error) {
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
	UUID                    uuid.UUID   `json:"uuid"`
	ServerReceivedTimestamp time.Time   `json:"server_received_timestamp"`
	NotificationTxt         string      `json:"txt"`
	Dest                    Destination `json:"destination"`
}

type OutstandingNotification struct {
	UUID uuid.UUID `json:"uuid"`
}

type Notification struct {
	UUID            string `json:"uuid"`
	NotificationTxt string `json:"txt"`
	Destination     string `json:"destination"`
}

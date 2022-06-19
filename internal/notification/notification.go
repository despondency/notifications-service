package notification

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

type Destination int8

const (
	SMS Destination = iota
	Email
	Slack
)

var ErrNoSuchDestination = fmt.Errorf("no such destination exists")

func toServerNotificationDestination(destination string) (Destination, error) {
	destinationUpper := strings.ToUpper(destination)
	switch destinationUpper {
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

type Notification struct {
	UUID                    uuid.UUID   `json:"uuid"`
	ServerReceivedTimestamp time.Time   `json:"server_received_timestamp"`
	NotificationTxt         string      `json:"txt"`
	Dest                    Destination `json:"destination"`
}

type OutstandingNotification struct {
	UUID uuid.UUID `json:"uuid"`
}

type Request struct {
	UUID            string `json:"uuid"`
	NotificationTxt string `json:"txt"`
	Destination     string `json:"destination"`
}

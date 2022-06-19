package notification

import (
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/despondency/notifications-service/internal/messaging"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
	"time"
)

type InternalService struct {
	consumer                         *kafka.Consumer
	outstandingNotificationsProducer *messaging.KafkaProducer
	persistence                      *CRDBPersistence
	stopped                          *atomic.Bool
}

func NewInternalService(outstandingNotificationsProducer *messaging.KafkaProducer,
	persistence *CRDBPersistence) (*InternalService, error) {
	is := &InternalService{
		persistence:                      persistence,
		outstandingNotificationsProducer: outstandingNotificationsProducer,
		stopped:                          atomic.NewBool(false),
	}
	return is, nil
}

func (s *InternalService) PushNotificationInternal(n *Request) error {
	dest, err := toServerNotificationDestination(n.Destination)
	if err != nil {
		log.Err(err).Msg(fmt.Sprintf("destination conversion to server notification destination failed, original destination is %s", n.Destination))
	}
	nUUID, err := uuid.Parse(n.UUID)
	if err != nil {
		return err
	}
	serverNotification := &Notification{
		UUID:                    nUUID,
		ServerReceivedTimestamp: time.Now().UTC(),
		NotificationTxt:         n.NotificationTxt,
		Dest:                    dest,
	}
	b, err := json.Marshal(serverNotification)
	if err != nil {
		return err
	}
	return s.outstandingNotificationsProducer.Produce(b)
}

package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/despondency/notifications-service/internal/messaging"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
	"time"
)

type InternalService struct {
	consumer                         *kafka.Consumer
	kp                               *messaging.KafkaProducer
	outstandingNotificationsProducer *messaging.KafkaProducer
	persistence                      *storage.CRDBPersistence
	stopped                          *atomic.Bool
	maxRoutines                      int
}

func NewInternalService(bootstrapServers, receivedNotificationsGroupID, resetOffset, receivedNotificationsTopic, enableAutoCommit string,
	kp *messaging.KafkaProducer, outstandingNotificationsProducer *messaging.KafkaProducer, persistence *storage.CRDBPersistence, maxRoutines int) (*InternalService, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           receivedNotificationsGroupID,
		"auto.offset.reset":  resetOffset,
		"enable.auto.commit": enableAutoCommit})
	if err != nil {
		return nil, err
	}
	err = c.Subscribe(receivedNotificationsTopic, nil)
	if err != nil {
		return nil, err
	}
	is := &InternalService{
		kp:                               kp,
		consumer:                         c,
		persistence:                      persistence,
		outstandingNotificationsProducer: outstandingNotificationsProducer,
		stopped:                          atomic.NewBool(false),
		maxRoutines:                      maxRoutines,
	}
	is.consumeNotificationInternal()
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
	serverNotification := &ServerNotification{
		UUID:                    nUUID,
		ServerReceivedTimestamp: time.Now().UTC(),
		NotificationTxt:         n.NotificationTxt,
		Dest:                    dest,
	}
	b, err := json.Marshal(serverNotification)
	if err != nil {
		return err
	}
	return s.kp.Produce(b)
}

func (s *InternalService) Stop() {
	s.stopped.Store(true)
	_, err := s.consumer.Commit()
	if err != nil {
		log.Err(err).Msg("could not commit consumer while stopping in internal notifications")
	}
	err = s.consumer.Close()
	if err != nil {
		log.Err(err).Msg("could not close consumer while stopping in internal notifications")
	}
}

func (s *InternalService) consumeNotificationInternal() {
	maxReq := make(chan struct{}, s.maxRoutines)
	go func() {
		for s.stopped.Load() == false {
			ev := s.consumer.Poll(100) // release CPU quota 100ms
			switch e := ev.(type) {
			case *kafka.Message:
				maxReq <- struct{}{}
				go func() {
					ctx := context.Background()
					defer func() {
						<-maxReq
					}()
					err := s.createOutstandingNotification(ctx, e)
					if err != nil {
						log.Err(err).Msg("could not insert new server notification")
						return
					}
					_, err = s.consumer.CommitMessage(e)
					if err != nil {
						log.Err(err).Msg("could not commit message")
						return
					}
				}()
			case kafka.Error:
				log.Err(e).Msg("kafka produced an error in outstanding service")
			default:
			}
		}
	}()
}

func (s *InternalService) createOutstandingNotification(ctx context.Context, msg *kafka.Message) error {
	var serverNotification *ServerNotification
	err := json.Unmarshal(msg.Value, &serverNotification)
	if err != nil {
		return err
	}
	var affected int64
	err = crdbpgx.ExecuteTx(ctx, s.persistence.GetPool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		affected, err = s.persistence.InsertOnConflictNothing(ctx, ToUnprocessedNotification(serverNotification), tx)
		return err
	})
	if err != nil {
		return err
	}
	// this signifies that we actually have a new "unique" notification.
	// if it's a duplicate it will be 0
	if affected == 1 {
		outStandingNotification := &OutstandingNotification{
			UUID: serverNotification.UUID,
		}
		b, errMarshal := json.Marshal(outStandingNotification)
		if errMarshal != nil {
			return err
		}
		errMarshal = s.outstandingNotificationsProducer.Produce(b)
		if errMarshal != nil {
			return errMarshal
		}
		return errMarshal
	}
	return nil
}

func ToUnprocessedNotification(n *ServerNotification) *storage.Notification {
	return &storage.Notification{
		UUID: n.UUID,
		Txt:  n.NotificationTxt,
		Status: pgtype.Int2{
			Int:    int16(NOT_PROCESSED),
			Status: pgtype.Present,
		},
		Dest: pgtype.Int2{
			Int:    int16(n.Dest),
			Status: pgtype.Present,
		},
		ServerTimestamp: pgtype.Timestamp{
			Time:   n.ServerReceivedTimestamp.UTC(),
			Status: pgtype.Present,
		},
		LastUpdated: pgtype.Timestamp{
			Time:   n.ServerReceivedTimestamp.UTC(),
			Status: pgtype.Present,
		},
	}
}

package notification

import (
	"context"
	"encoding/json"
	"fmt"
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
	txCreator                        *storage.WrappedTxCreator
	persistence                      storage.Persistence
	stopped                          *atomic.Bool
}

const (
	receivedNotificationsMaxGoRoutines = 10
)

func NewInternalService(bootstrapServers, receivedNotificationsGroupID, resetOffset, receivedNotificationsTopic, enableAutoCommit string,
	kp *messaging.KafkaProducer, outstandingNotificationsProducer *messaging.KafkaProducer, txCreator *storage.WrappedTxCreator, persistence storage.Persistence) *InternalService {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           receivedNotificationsGroupID,
		"auto.offset.reset":  resetOffset,
		"enable.auto.commit": enableAutoCommit})
	if err != nil {
		panic(err)
	}
	err = c.Subscribe(receivedNotificationsTopic, nil)

	is := &InternalService{
		kp:                               kp,
		consumer:                         c,
		txCreator:                        txCreator,
		persistence:                      persistence,
		outstandingNotificationsProducer: outstandingNotificationsProducer,
		stopped:                          atomic.NewBool(false),
	}
	is.consumeNotificationInternal()
	return is
}

func (s *InternalService) PushNotificationInternal(n *Notification) error {
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
		log.Err(err).Msg("could not commit consumer while stopping in internal service")
	}
	err = s.consumer.Close()
	if err != nil {
		log.Err(err).Msg("could not close consumer while stopping in internal service")
	}
}

func (s *InternalService) consumeNotificationInternal() {
	maxReq := make(chan struct{}, receivedNotificationsMaxGoRoutines)
	go func() {
		for s.stopped.Load() == false {
			ev := s.consumer.Poll(1000)
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
				}()
			case kafka.Error:
				// maybe fatal here?
				// its unrecoverable
			default:
				//fmt.Printf("Ignored %v\n", e)
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
	tx, err := s.txCreator.NewTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	affected, err := s.persistence.InsertOnConflictNothing(ctx, ToUnprocessedNotification(serverNotification), tx)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		log.Err(err).Msg("error committing server notification")
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

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

func NewInternalService(kp *messaging.KafkaProducer, outstandingNotificationsProducer *messaging.KafkaProducer, txCreator *storage.WrappedTxCreator, persistence storage.Persistence) *InternalService {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  "localhost:9092",
		"group.id":           "received-notifications-group-id",
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false"})
	if err != nil {
		panic(err)
	}
	err = c.Subscribe("received-notifications", nil)

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
		log.Err(err).Msg(fmt.Sprintf("destination is %d", dest))
	}
	serverNotification := &ServerNotification{
		ServerUUID:              uuid.New(),
		ServerReceivedTimestamp: time.Now().UTC(),
		NotificationTxt:         n.NotificationTxt,
		Dest:                    dest,
	}
	b, err := json.Marshal(serverNotification)
	if err != nil {
		return err
	}
	err = s.kp.Produce(b)
	if err != nil {
		return err
	}
	return nil
}

func (s *InternalService) Stop() {
	s.stopped.Store(true)
	s.consumer.Commit()
	s.consumer.Close()
}

func (s *InternalService) consumeNotificationInternal() {
	maxReq := make(chan struct{}, 1000)
	go func() {
		for s.stopped.Load() == false {
			ev := s.consumer.Poll(0)
			switch e := ev.(type) {
			case *kafka.Message:
				maxReq <- struct{}{}
				go func() {
					ctx := context.Background()
					defer func() {
						<-maxReq
					}()
					var serverNotification ServerNotification
					err := json.Unmarshal(e.Value, &serverNotification)
					if err != nil {
						log.Err(err).Msg("cannot consume internal notification")
					}
					tx, err := s.txCreator.NewTx(ctx, pgx.TxOptions{})
					if err != nil {
						log.Err(err).Msg("could not create tx")
						return
					}
					err = s.persistence.InsertOnConflictNothing(context.Background(), ToUnprocessedNotification(&serverNotification), tx)
					if err != nil {
						log.Err(err).Msg("could not upsert notification")
						return
					}
					err = tx.Commit(ctx)
					if err != nil {
						log.Err(err).Msg("could not commit tx")
						return
					}
					on := OutstandingNotification{
						ServerUUID: serverNotification.ServerUUID,
					}
					b, err := json.Marshal(on)
					if err != nil {
						log.Err(err).Msg("could not marshal outstanding notification")
					}
					err = s.outstandingNotificationsProducer.Produce(b)
					if err != nil {
						log.Err(err).Msg("failed to produce outstanding notification")
						return
					}
					_, err = s.consumer.CommitMessage(e)
				}()
			case kafka.Error:
				// maybe fatal here?
			default:
				//	fmt.Printf("Ignored %v\n", e)
			}
		}
	}()
}

func ToUnprocessedNotification(n *ServerNotification) *storage.Notification {
	return &storage.Notification{
		ServerUUID: n.ServerUUID,
		Txt:        n.NotificationTxt,
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
			Time:   time.Now().UTC(),
			Status: pgtype.Present,
		},
	}
}

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
	receivedNotificationsMax = 1000
)

func NewInternalService(
	bootstrapServers, receivedNotificationsGroupID, resetOffset, receivedNotificationsTopic, enableAutoCommit string,
	kp *messaging.KafkaProducer, outstandingNotificationsProducer *messaging.KafkaProducer,
	txCreator *storage.WrappedTxCreator, persistence storage.Persistence) *InternalService {
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
	maxReq := make(chan struct{}, receivedNotificationsMax)
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
					serverNotification, err := s.insertNewServerNotification(ctx, e)
					if err != nil {
						log.Err(err).Msg("could not insert new server notification")
						return
					}
					err = s.pushOutstandingNotification(serverNotification)
					if err != nil {
						log.Err(err).Msg("could not push outstanding notification")
						return
					}
					_, err = s.consumer.CommitMessage(e)
				}()
			case kafka.Error:
				//maybe fatal here?
			default:
				//fmt.Printf("Ignored %v\n", e)
			}
		}
	}()
}

func (s *InternalService) pushOutstandingNotification(serverNotification *ServerNotification) error {
	on := OutstandingNotification{
		ServerUUID: serverNotification.ServerUUID,
	}
	b, err := json.Marshal(on)
	if err != nil {
		return err
	}
	err = s.outstandingNotificationsProducer.Produce(b)
	if err != nil {
		return err
	}
	return nil
}

func (s *InternalService) insertNewServerNotification(ctx context.Context, e *kafka.Message) (*ServerNotification, error) {
	var serverNotification *ServerNotification
	err := json.Unmarshal(e.Value, &serverNotification)
	if err != nil {
		return nil, err
	}
	tx, err := s.txCreator.NewTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Commit(ctx)
		if err != nil {
			log.Err(err).Msg("error committing server notification")
		}
	}()
	err = s.persistence.InsertOnConflictNothing(context.Background(), ToUnprocessedNotification(serverNotification), tx)
	if err != nil {
		return nil, err
	}
	return serverNotification, err
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

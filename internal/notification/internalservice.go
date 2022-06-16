package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/despondency/notifications-service/internal/messaging"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"time"
)

type InternalService struct {
	kp                               *messaging.KafkaProducer
	kc                               *messaging.KafkaConsumer
	outstandingNotificationsProducer *messaging.KafkaProducer
	txCreator                        *storage.WrappedTxCreator
	persistence                      storage.Persistence
}

func NewInternalService(kp *messaging.KafkaProducer,
	kc *messaging.KafkaConsumer, outstandingNotificationsProducer *messaging.KafkaProducer, txCreator *storage.WrappedTxCreator, persistence storage.Persistence) *InternalService {
	is := &InternalService{
		kp:                               kp,
		kc:                               kc,
		txCreator:                        txCreator,
		persistence:                      persistence,
		outstandingNotificationsProducer: outstandingNotificationsProducer,
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

func (s *InternalService) consumeNotificationInternal() {
	ch := s.kc.GetRelayChan()
	ctx := context.Background()
	go func() {
		for b := range ch {
			var serverNotification ServerNotification
			err := json.Unmarshal(b, &serverNotification)
			if err != nil {
				log.Err(err).Msg("cannot consume internal notification")
			}
			tx, err := s.txCreator.NewTx(ctx, pgx.TxOptions{})
			if err != nil {
				log.Err(err).Msg("could not create tx")
			}
			err = s.persistence.InsertOnConflictNothing(context.Background(), ToUnprocessedNotification(&serverNotification), tx)
			if err != nil {
				log.Err(err).Msg("could not upsert notification")
			}
			err = tx.Commit(ctx)
			if err != nil {
				log.Err(err).Msg("could not commit tx")
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

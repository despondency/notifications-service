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
	"go.uber.org/atomic"
	"time"
)

type InternalService struct {
	outstandingNotificationsProducer *messaging.KafkaProducer
	txCreator                        *storage.WrappedTxCreator
	persistence                      storage.Persistence
	stopped                          *atomic.Bool
}

func NewInternalService(outstandingNotificationsProducer *messaging.KafkaProducer,
	txCreator *storage.WrappedTxCreator, persistence storage.Persistence) *InternalService {
	is := &InternalService{
		txCreator:                        txCreator,
		persistence:                      persistence,
		outstandingNotificationsProducer: outstandingNotificationsProducer,
		stopped:                          atomic.NewBool(false),
	}
	return is
}

func (s *InternalService) HandleNotification(ctx context.Context, n *Notification) error {
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
	return s.createOutstandingNotification(ctx, serverNotification)
}

func (s *InternalService) createOutstandingNotification(ctx context.Context, serverNotification *ServerNotification) error {
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
		b, err := json.Marshal(outStandingNotification)
		if err != nil {
			return err
		}
		err = s.outstandingNotificationsProducer.Produce(b)
		if err != nil {
			return err
		}
		return err
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

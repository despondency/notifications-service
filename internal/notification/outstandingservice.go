package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

type OutstandingService struct {
	notificator *DelegatingNotificator
	kc          *kafka.Consumer
	txCreator   *storage.WrappedTxCreator
	persistence storage.Persistence
	stopped     *atomic.Bool
}

func NewOutstandingService(txCreator *storage.WrappedTxCreator, persistence storage.Persistence,
	notificator *DelegatingNotificator) *OutstandingService {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  "localhost:9092",
		"group.id":           "outstanding-notifications-group-id",
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false"})
	if err != nil {
		panic(err)
	}
	err = c.Subscribe("outstanding-notifications", nil)
	ous := &OutstandingService{
		kc:          c,
		txCreator:   txCreator,
		persistence: persistence,
		notificator: notificator,
		stopped:     atomic.NewBool(false),
	}
	ous.consumeNotificationOutstanding()
	return ous
}

func (ous *OutstandingService) Stop() {
	ous.stopped.Store(true)
	ous.kc.Commit()
	ous.kc.Close()
}

func (ous *OutstandingService) consumeNotificationOutstanding() {
	maxReq := make(chan struct{}, 1000)
	go func() {
		for ous.stopped.Load() == false {
			ev := ous.kc.Poll(0)
			switch e := ev.(type) {
			case *kafka.Message:
				maxReq <- struct{}{}
				go func() {
					ctx := context.Background()
					defer func() {
						<-maxReq
					}()
					var outstandingNotification OutstandingNotification
					err := json.Unmarshal(e.Value, &outstandingNotification)
					if err != nil {
						log.Err(err).Msg("cannot consume internal notification")
					}
					tx, err := ous.txCreator.NewTx(ctx, pgx.TxOptions{})
					if err != nil {
						log.Err(err).Msg("could not create tx")
						return
					} else {
						err := ous.handleMsg(err, ctx, outstandingNotification, tx)
						if err != nil {
							tx.Rollback(ctx)
						} else {
							_, err = ous.kc.CommitMessage(e)
							tx.Commit(ctx)
						}
					}
				}()
			case kafka.Error:
				// maybe fatal here?
			default:
				// fmt.Printf("Ignored %v\n", e)
			}
		}
	}()
}

func (ous *OutstandingService) handleMsg(err error, ctx context.Context, outstandingNotification OutstandingNotification, tx *storage.WrappedTx) error {
	sv, err := ous.persistence.GetForUpdate(ctx, outstandingNotification.ServerUUID, tx)
	if err != nil {
		log.Err(err).Msg(fmt.Sprintf("could not get for update server uuid %s", outstandingNotification.ServerUUID))
		return err
	}
	if sv.Status.Int == int16(NOT_PROCESSED) {
		errSend := ous.notificator.DelegateNotification(&DelegatingNotification{
			serverUUID: sv.ServerUUID,
			txt:        sv.Txt,
			dest:       Destination(sv.Dest.Int),
		})
		if errSend != nil {
			log.Err(errSend).Msg(fmt.Sprintf("could not send notification for server uuid %s", outstandingNotification.ServerUUID))
			return err
		} else {
			err = ous.persistence.UpdateStatus(ctx, outstandingNotification.ServerUUID, pgtype.Int2{
				Int:    int16(PROCESSED),
				Status: pgtype.Present,
			}, tx)
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("could not update status for uuid %s", outstandingNotification.ServerUUID))
				return err
			}
			if err != nil {
				log.Err(err).Msg(fmt.Sprintf("could not commit tx successful notification send for %s", outstandingNotification.ServerUUID))
				return err
			}
		}
	} else {
		// skip since its already processed
		log.Info().Msg(fmt.Sprintf("skipping already processed notification %s", sv.ServerUUID))
		if err != nil {
			log.Err(err).Msg(fmt.Sprintf("could not commit tx successful notification send for %s", outstandingNotification.ServerUUID))
		}
	}
	return err
}

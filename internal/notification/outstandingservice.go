package notification

import (
	"context"
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

type OutstandingService struct {
	notificator *DelegatingNotificator
	consumer    *kafka.Consumer
	txCreator   *storage.WrappedTxCreator
	persistence storage.Persistence
	stopped     *atomic.Bool
}

const (
	outstandingNotificationsMax = 1000
)

func NewOutstandingService(
	bootstrapServers, outstandingGroupID, autoResetOffset, enableAutoCommit, outstandingNotificationsTopic string,
	txCreator *storage.WrappedTxCreator, persistence storage.Persistence,
	notificator *DelegatingNotificator) *OutstandingService {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           outstandingGroupID,
		"auto.offset.reset":  autoResetOffset,
		"enable.auto.commit": enableAutoCommit})
	if err != nil {
		panic(err)
	}
	err = consumer.Subscribe(outstandingNotificationsTopic, nil)
	ous := &OutstandingService{
		consumer:    consumer,
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
	_, err := ous.consumer.Commit()
	if err != nil {
		log.Err(err).Msg("could not commit consumer while stopping in outstanding service")
	}
	err = ous.consumer.Close()
	if err != nil {
		log.Err(err).Msg("could not close consumer while stopping in outstanding service")
	}
}
func (ous *OutstandingService) consumeNotificationOutstanding() {
	maxReq := make(chan struct{}, outstandingNotificationsMax)
	go func() {
		for ous.stopped.Load() == false {
			ev := ous.consumer.Poll(1000)
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
						return
					}
					tx, err := ous.txCreator.NewTx(ctx, pgx.TxOptions{})
					if err != nil {
						log.Err(err).Msg("could not create tx")
						return
					} else {
						err := ous.handleMsg(err, ctx, outstandingNotification, tx)
						if err != nil {
							log.Err(err).Msg("error handling message for outstanding notification")
							errRollback := tx.Rollback(ctx)
							if errRollback != nil {
								log.Err(errRollback).Msg("could not rollback outstanding notification tx")
							}
						} else {
							errCommit := tx.Commit(ctx)
							if errCommit != nil {
								log.Err(errCommit).Msg("could not commit outstanding notification tx")
								return
							}
							_, errCommitMsg := ous.consumer.CommitMessage(e)
							if errCommitMsg != nil {
								log.Err(errCommitMsg).Msg("could not commit outstanding notification msg")
								return
							}
						}
					}
				}()
			case kafka.Error:
				// maybe fatal here?
				// usually this indicates unrecoverable error
			default:
			}
		}
	}()
}

func (ous *OutstandingService) handleMsg(err error, ctx context.Context, outstandingNotification OutstandingNotification, tx *storage.WrappedTx) error {
	sv, err := ous.persistence.GetForUpdate(ctx, outstandingNotification.UUID, tx)
	if err != nil {
		return err
	}
	if sv.Status.Int == int16(NOT_PROCESSED) {
		errSend := ous.notificator.DelegateNotification(&DelegatingNotification{
			uuid: sv.UUID,
			txt:  sv.Txt,
			dest: Destination(sv.Dest.Int),
		})
		if errSend != nil {
			return err
		} else {
			err = ous.persistence.UpdateStatus(ctx, outstandingNotification.UUID, pgtype.Int2{
				Int:    int16(PROCESSED),
				Status: pgtype.Present,
			}, tx)
			if err != nil {
				return err
			}
		}
	}
	return err
}

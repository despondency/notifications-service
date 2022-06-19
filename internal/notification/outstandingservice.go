package notification

import (
	"context"
	"encoding/json"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
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
	persistence *storage.CRDBPersistence
	stopped     *atomic.Bool
	maxRoutines int
}

func NewOutstandingService(
	bootstrapServers, outstandingGroupID, autoResetOffset, enableAutoCommit, outstandingNotificationsTopic string, persistence *storage.CRDBPersistence,
	notificator *DelegatingNotificator, maxRoutines int) (*OutstandingService, error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           outstandingGroupID,
		"auto.offset.reset":  autoResetOffset,
		"enable.auto.commit": enableAutoCommit})
	if err != nil {
		return nil, err
	}
	err = consumer.Subscribe(outstandingNotificationsTopic, nil)
	if err != nil {
		return nil, err
	}
	ous := &OutstandingService{
		consumer:    consumer,
		persistence: persistence,
		notificator: notificator,
		maxRoutines: maxRoutines,
		stopped:     atomic.NewBool(false),
	}
	ous.consumeNotificationOutstanding()
	return ous, nil
}

func (ous *OutstandingService) Stop() {
	ous.stopped.Store(true)
	_, err := ous.consumer.Commit()
	if err != nil {
		log.Err(err).Msg("could not commit consumer while stopping in outstanding notifications")
	}
	err = ous.consumer.Close()
	if err != nil {
		log.Err(err).Msg("could not close consumer while stopping in outstanding notifications")
	}
}
func (ous *OutstandingService) consumeNotificationOutstanding() {
	maxReq := make(chan struct{}, ous.maxRoutines)
	go func() {
		for ous.stopped.Load() == false {
			ev := ous.consumer.Poll(100) // release CPU quota 100ms
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
					} else {
						err = ous.handleMsg(ctx, outstandingNotification)
						if err != nil {
							return
						}
						_, errCommitMsg := ous.consumer.CommitMessage(e)
						if errCommitMsg != nil {
							log.Err(errCommitMsg).Msg("could not commit outstanding notification msg")
							return
						}
					}
				}()
			case kafka.Error:
				log.Err(e).Msg("kafka produced an error in outstanding service")
			default:
			}
		}
	}()
}

func (ous *OutstandingService) handleMsg(ctx context.Context, outstandingNotification OutstandingNotification) error {
	err := crdbpgx.ExecuteTx(ctx, ous.persistence.GetPool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
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
			}
			return ous.persistence.UpdateStatus(ctx, outstandingNotification.UUID, pgtype.Int2{
				Int:    int16(PROCESSED),
				Status: pgtype.Present,
			}, tx)
		}
		return nil
	})
	return err
}

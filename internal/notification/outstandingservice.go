package notification

import (
	"context"
	"encoding/json"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

type OutstandingService struct {
	notificator *DelegatingNotificator
	consumer    *kafka.Consumer
	persistence *CRDBPersistence
	stopped     *atomic.Bool
	maxRoutines int
}

func NewOutstandingService(
	bootstrapServers, outstandingGroupID, autoResetOffset, enableAutoCommit, outstandingNotificationsTopic string, persistence *CRDBPersistence,
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
					err := ous.handleMsg(ctx, e)
					if err != nil {
						log.Err(err).Msg("could not handle msg in outstanding service")
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

func (ous *OutstandingService) handleMsg(ctx context.Context, msg *kafka.Message) error {
	var serverNotification *Notification
	errUnmarshall := json.Unmarshal(msg.Value, &serverNotification)
	if errUnmarshall != nil {
		return errUnmarshall
	}
	err := crdbpgx.ExecuteTx(ctx, ous.persistence.GetPool(), pgx.TxOptions{}, func(tx pgx.Tx) error {
		affected, err := ous.persistence.InsertOnConflictNothing(ctx, serverNotification, tx)
		if err != nil {
			return err
		}
		if affected == 1 {
			// it's a new notification
			// delegate it
			// on err the ExecuteTx will rollback, and retry later from the log because we won't commit the msg
			err = ous.notificator.DelegateNotification(&DelegatingNotification{
				uuid: serverNotification.UUID,
				txt:  serverNotification.NotificationTxt,
				dest: serverNotification.Dest,
			})
			if err != nil {
				return err // this is non retryable so we will rollback
			}
		}
		return nil
	})
	// if there is no error, commit the msg
	if err == nil {
		_, err = ous.consumer.CommitMessage(msg)
		if err != nil {
			log.Err(err).Msg("failed to commit outstanding message")
		}
	}
	return err
}

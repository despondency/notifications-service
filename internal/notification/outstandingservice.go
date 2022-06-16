package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/despondency/notifications-service/internal/messaging"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/rs/zerolog/log"
)

type OutstandingService struct {
	notificator *DelegatingNotificator
	kc          *kafka.Consumer
	txCreator   *storage.WrappedTxCreator
	persistence storage.Persistence
}

func NewOutstandingService(kc *messaging.KafkaConsumer, txCreator *storage.WrappedTxCreator, persistence storage.Persistence,
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
	}
	ous.consumeNotificationOutstanding()
	return ous
}

func (ous *OutstandingService) consumeNotificationOutstanding() {
	ctx := context.Background()
	go func() {
		for {
			ev := ous.kc.Poll(0)
			switch e := ev.(type) {
			case *kafka.Message:
				var outstandingNotification OutstandingNotification
				err := json.Unmarshal(e.Value, &outstandingNotification)
				if err != nil {
					log.Err(err).Msg("cannot consume internal notification")
				}
				log.Info().Msg(fmt.Sprintf("%s event", outstandingNotification))
				tx, err := ous.txCreator.NewTx(ctx, pgx.TxOptions{})
				if err != nil {
					log.Err(err).Msg("could not create tx")
				}
				sv, err := ous.persistence.GetForUpdate(ctx, outstandingNotification.ServerUUID, tx)
				if err != nil {
					log.Err(err).Msg(fmt.Sprintf("could not get for update server uuid %s", outstandingNotification.ServerUUID))
				}
				// simulate error
				if true {
					err := ous.kc.Seek(e.TopicPartition, 0)
					if err != nil {
						panic(err)
					}
					err = tx.Commit(ctx)
					if err != nil {
						log.Err(err).Msg(fmt.Sprintf("could not commit tx for server uuid %s", outstandingNotification.ServerUUID))
					}
					continue
				} else {
					_, err := ous.kc.Commit()
					if err != nil {
						log.Err(err).Msg("cannot commit kafka")
					}
				}
				if sv.Status.Int == int16(NOT_PROCESSED) {
					err = ous.notificator.DelegateNotification(&DelegatingNotification{
						serverUUID: sv.ServerUUID,
						txt:        sv.Txt,
						dest:       Destination(sv.Dest.Int),
					})
					if err != nil {
						log.Err(err).Msg(fmt.Sprintf("could not send notification for server uuid %s", outstandingNotification.ServerUUID))
						// commit
						err = tx.Commit(ctx)
						if err != nil {
							log.Err(err).Msg(fmt.Sprintf("could not commit tx for server uuid %s", outstandingNotification.ServerUUID))
						}
					} else {
						err = ous.persistence.UpdateStatus(ctx, outstandingNotification.ServerUUID, pgtype.Int2{
							Int:    int16(PROCESSED),
							Status: pgtype.Present,
						}, tx)
						if err != nil {
							log.Err(err).Msg(fmt.Sprintf("could not update status for uuid %s", outstandingNotification.ServerUUID))
						}
						err = tx.Commit(ctx)
						if err != nil {
							log.Err(err).Msg(fmt.Sprintf("could not commit tx successful notification send for %s", outstandingNotification.ServerUUID))
						}
					}
				}
			case kafka.Error:
				// maybe fatal here?
			default:
				//	fmt.Printf("Ignored %v\n", e)
			}
		}
	}()
}

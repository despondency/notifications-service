package database_test

import (
	"context"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/despondency/notifications-service/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Database", func() {

	Context("Test DB", func() {

		var (
			affected int64
			err      error
			toInsert *storage.Notification
		)

		var uuid = uuid.New()

		BeforeEach(func() {
			toInsert = &storage.Notification{
				UUID: uuid,
				Txt:  "txt",
				Status: pgtype.Int2{
					Int:    int16(notification.NOT_PROCESSED),
					Status: pgtype.Present,
				},
				Dest: pgtype.Int2{
					Int:    int16(notification.Email),
					Status: pgtype.Present,
				},
				ServerTimestamp: pgtype.Timestamp{
					Time:   time.Now().UTC(),
					Status: pgtype.Present,
				},
				LastUpdated: pgtype.Timestamp{
					Time:   time.Now().UTC(),
					Status: pgtype.Present,
				},
			}
		})

		JustBeforeEach(func() {
			err = crdbpgx.ExecuteTx(context.Background(), connPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
				var errInsert error
				affected, errInsert = store.InsertOnConflictNothing(context.Background(), toInsert, tx)
				return errInsert
			})
		})

		Context("Insert a new notification", func() {
			It("should insert successfully", func() {
				Expect(affected).To(Equal(int64(1)))
				Expect(err).To(BeNil())
			})
			Context("Insert same notification", func() {
				It("should do nothing", func() {
					Expect(affected).To(Equal(int64(0)))
					Expect(err).To(BeNil())
				})
			})
			Context("Get and Update notification", func() {
				BeforeEach(func() {
					ctx := context.Background()
					err = crdbpgx.ExecuteTx(ctx, connPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
						n, err := store.GetForUpdate(ctx, toInsert.UUID, tx)
						if err != nil {
							return err
						}
						var errUpdate error
						errUpdate = store.UpdateStatus(context.Background(), n.UUID, pgtype.Int2{
							Int:    int16(notification.PROCESSED),
							Status: pgtype.Present,
						}, tx)
						return errUpdate
					})
				})
				It("should have updated the status", func() {
					Expect(err).To(BeNil())
				})
				Context("Get the updated notification", func() {
					var (
						updatedNotification *storage.Notification
					)
					BeforeEach(func() {
						err = crdbpgx.ExecuteTx(context.Background(), connPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
							var errGet error
							updatedNotification, errGet = store.Get(context.Background(), toInsert.UUID, tx)
							return errGet
						})
					})
					It("should get the notification", func() {
						Expect(err).To(BeNil())
						Expect(updatedNotification).To(Not(BeNil()))
						Expect(updatedNotification.Status.Int).To(Equal(int16(notification.PROCESSED)))
					})
				})
			})
		})
	})
})

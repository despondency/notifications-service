package database_test

import (
	"context"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Database", func() {

	Context("Test DB", func() {

		var (
			affected int64
			err      error
			toInsert *notification.Notification
		)

		var uuid = uuid.New()

		BeforeEach(func() {
			toInsert = &notification.Notification{
				UUID:            uuid,
				NotificationTxt: "txt",
				Dest:            notification.Email,
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
				Context("Get the notification", func() {
					var (
						notificationInserted *notification.Notification
					)
					BeforeEach(func() {
						err = crdbpgx.ExecuteTx(context.Background(), connPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
							var errGet error
							notificationInserted, errGet = store.Get(context.Background(), toInsert.UUID, tx)
							return errGet
						})
					})
					It("should get the notification", func() {
						Expect(err).To(BeNil())
						Expect(notificationInserted).To(Not(BeNil()))
					})
				})
			})
		})
	})
})

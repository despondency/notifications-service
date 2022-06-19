package notifications_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/despondency/notifications-service/internal/notification"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"net/http"
)

var (
	notifications []*notification.Request
)

var _ = Describe("Push NotificationRequest Test", func() {

	JustBeforeEach(func() {
		for _, n := range notifications {
			sendNotification(n)
		}
	})

	Context("Create Email NotificationRequest and push it", func() {
		BeforeEach(func() {
			notifications = make([]*notification.Request, 0)
			notifications = append(notifications, &notification.Request{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "EMAIL",
			})
		})

		It("should push the notification successfully", func() {
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create SMS NotificationRequest and push it", func() {
		BeforeEach(func() {
			notifications = make([]*notification.Request, 0)
			notifications = append(notifications, &notification.Request{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "SMS",
			})
		})

		It("should push the notification successfully", func() {
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create Slack NotificationRequest and push it", func() {
		BeforeEach(func() {
			notifications = make([]*notification.Request, 0)
			notifications = append(notifications, &notification.Request{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "SLACK",
			})
		})

		It("should push the notification successfully", func() {
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create 100 notification and push them", func() {
		BeforeEach(func() {
			notifications = make([]*notification.Request, 100)
			for i := 0; i < 100; i++ {
				notifications[i] = &notification.Request{
					UUID:            uuid.New().String(),
					NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
					Destination:     "EMAIL",
				}
			}
		})

		It("should push the notifications successfully", func() {
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})
})

func CheckNotifications() bool {
	storedNotifications := make([]*notification.Notification, len(notifications))
	for i, n := range notifications {
		ctx := context.Background()
		var storedNotification *notification.Notification
		err := crdbpgx.ExecuteTx(ctx, connPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			var errGet error
			storedNotification, errGet = store.Get(ctx, uuid.MustParse(n.UUID), tx)
			return errGet
		})
		if err != nil && err.Error() == "no rows in result set" {
			return false
		} else if err != nil && err.Error() != "no rows in result set" {
			// this deserves a panic
			panic(err)
		}
		storedNotifications[i] = storedNotification
	}
	if len(storedNotifications) == len(notifications) {
		return true
	}
	return false
}

func sendNotification(n *notification.Request) {
	b, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("%s/notification", cfg.NodeHost), bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() {
		resp.Body.Close()
	}()
}

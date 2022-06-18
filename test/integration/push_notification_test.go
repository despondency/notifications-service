package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/despondency/notifications-service/internal/storage"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"net/http"
)

var (
	notifs []*notification.Notification
	err    error
)

var _ = Describe("Push Notification Test", func() {

	JustBeforeEach(func() {
		for _, n := range notifs {
			sendNotification(n)
		}
	})

	Context("Create Email Notification and push it", func() {
		BeforeEach(func() {
			notifs = make([]*notification.Notification, 0)
			notifs = append(notifs, &notification.Notification{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "EMAIL",
			})
		})

		It("should push the notification successfully", func() {
			Expect(err).To(BeNil())
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create SMS Notification and push it", func() {
		BeforeEach(func() {
			notifs = make([]*notification.Notification, 0)
			notifs = append(notifs, &notification.Notification{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "SMS",
			})
		})

		It("should push the notification successfully", func() {
			Expect(err).To(BeNil())
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create Slack Notification and push it", func() {
		BeforeEach(func() {
			notifs = make([]*notification.Notification, 0)
			notifs = append(notifs, &notification.Notification{
				UUID:            uuid.New().String(),
				NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
				Destination:     "SLACK",
			})
		})

		It("should push the notification successfully", func() {
			Expect(err).To(BeNil())
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

	Context("Create 100 notification and push it", func() {
		BeforeEach(func() {
			notifs = make([]*notification.Notification, 100)
			for i := 0; i < 100; i++ {
				notifs[i] = &notification.Notification{
					UUID:            uuid.New().String(),
					NotificationTxt: fmt.Sprintf("%s-%s", "TEST", uuid.New().String()),
					Destination:     "EMAIL",
				}
			}
		})

		It("should push the notification successfully", func() {
			Expect(err).To(BeNil())
			Eventually(CheckNotifications).Should(Equal(true))
		})
	})

})

func CheckNotifications() bool {
	successfulNotifs := map[uuid.UUID]struct{}{}
	for _, n := range notifs {
		ctx := context.Background()
		var tx *storage.WrappedTx
		tx, err = txCreator.NewTx(ctx, pgx.TxOptions{})
		if err != nil {
			panic(err)
		}
		var storedNotif *storage.Notification
		storedNotif, err = store.Get(ctx, uuid.MustParse(n.UUID), tx)
		if err != nil {
			panic(err)
		}
		if storedNotif.Status.Int == int16(notification.PROCESSED) {
			successfulNotifs[storedNotif.UUID] = struct{}{}
		}
		err = tx.Commit(ctx)
		if err != nil {
			panic(err)
		}
	}
	if len(successfulNotifs) == len(notifs) {
		return true
	}
	return false
}

func sendNotification(n *notification.Notification) {
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

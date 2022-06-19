package notification_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/despondency/notifications-service/internal/notification/notificationmocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Endpoint", func() {

	Context("Test notifications endpoint", func() {
		var (
			endpoint              *notification.Endpoint
			mockedInternalManager *notificationmocks.MockInternalManager
			ctrl                  *gomock.Controller
			router                *httprouter.Router
			body                  io.Reader
			rr                    *httptest.ResponseRecorder
		)
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockedInternalManager = notificationmocks.NewMockInternalManager(ctrl)
			endpoint = notification.NewEndpoint(mockedInternalManager)

			router = httprouter.New()
			router.POST("/notifications", endpoint.CreateNotification)
		})

		JustBeforeEach(func() {
			req, _ := http.NewRequest("POST", "/notifications", body)
			rr = httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		})

		Context("Test happy path, 201", func() {
			BeforeEach(func() {
				mockedInternalManager.EXPECT().PushNotificationInternal(gomock.Any()).Return(nil).Times(1)
				n := &notification.Request{
					UUID:            uuid.New().String(),
					NotificationTxt: "TXT",
					Destination:     "EMAIL",
				}
				b, err := json.Marshal(n)
				if err != nil {
					panic(err)
				}
				body = bytes.NewBuffer(b)
			})

			It("should be return 201", func() {
				Expect(rr.Code).To(Equal(http.StatusCreated))
			})
		})

		Context("Test bad path, 400", func() {
			BeforeEach(func() {
				mockedInternalManager.EXPECT().PushNotificationInternal(gomock.Any()).Times(0)
				body = bytes.NewBufferString("{invalid json}")
			})

			It("should be return 400", func() {
				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("Test bad path, 500 because push failed", func() {
			BeforeEach(func() {
				mockedInternalManager.EXPECT().PushNotificationInternal(gomock.Any()).Return(fmt.Errorf("some-error-occurred")).Times(1)
				n := &notification.Request{
					UUID:            uuid.New().String(),
					NotificationTxt: "TXT",
					Destination:     "EMAIL",
				}
				b, err := json.Marshal(n)
				if err != nil {
					panic(err)
				}
				body = bytes.NewBuffer(b)
			})

			It("should be return 500", func() {
				Expect(rr.Code).To(Equal(http.StatusInternalServerError))
			})
		})

	})

})

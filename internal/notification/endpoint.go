package notification

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
)

type InternalManager interface {
	PushNotificationInternal(notification *Notification) error
}

type Endpoint struct {
	svc InternalManager
}

type Notification struct {
	notificationTxt string
	destination     string
}

func (e *Endpoint) CreateNotification(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	reader := r.Body
	length := r.ContentLength
	b := make([]byte, 0, length)
	_, err := io.ReadFull(reader, b)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	n := &Notification{}
	err = json.Unmarshal(b, n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	err = e.svc.PushNotificationInternal(n)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

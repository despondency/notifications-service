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

func NewEndpoint(svc InternalManager) *Endpoint {
	return &Endpoint{
		svc: svc,
	}
}

func (e *Endpoint) CreateNotification(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	reader := r.Body
	length := r.ContentLength
	b := make([]byte, length)
	_, err := io.ReadFull(reader, b)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	n := &Notification{}
	err = json.Unmarshal(b, n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = e.svc.PushNotificationInternal(n)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

package notification

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

//go:generate mockgen -source endpoint.go -destination ./notificationmocks/internalservice_mock.go -package notificationmocks
type InternalManager interface {
	PushNotificationInternal(notification *Request) error
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
		log.Err(err).Msg("error while reading req")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	n := &Request{}
	err = json.Unmarshal(b, n)
	if err != nil {
		log.Err(err).Msg("error while unmarshalling")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = e.svc.PushNotificationInternal(n)
	if err != nil {
		log.Err(err).Msg("internal error")
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

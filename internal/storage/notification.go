package storage

import (
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
)

type Notification struct {
	ServerUUID      uuid.UUID
	Txt             string
	Status          pgtype.Int2
	Dest            pgtype.Int2
	ServerTimestamp pgtype.Timestamp
	LastUpdated     pgtype.Timestamp
}

package storage

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
)

type Persistence interface {
	InsertOnConflictNothing(ctx context.Context, notification *Notification, tx *WrappedTx) error
	UpdateStatus(ctx context.Context, serverUUID uuid.UUID, status pgtype.Int2, tx *WrappedTx) error
	GetForUpdate(ctx context.Context, serverUUID uuid.UUID, tx *WrappedTx) (*Notification, error)
}

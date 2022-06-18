package storage

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

type CRDBPersistence struct {
	connPool *pgxpool.Pool
}

func NewCRDBPersistence(pool *pgxpool.Pool) *CRDBPersistence {
	return &CRDBPersistence{
		connPool: pool,
	}
}

func (crdbp *CRDBPersistence) InsertOnConflictNothing(ctx context.Context, notification *Notification, tx *WrappedTx) (int64, error) {
	v, errExec := (*tx.Tx).Exec(ctx,
		"INSERT into notifications(uuid, txt, status, destination, server_timestamp, last_updated) "+
			"VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT(uuid) DO NOTHING",
		notification.UUID,
		notification.Txt,
		notification.Status,
		notification.Dest,
		notification.ServerTimestamp,
		notification.LastUpdated,
	)
	if errExec != nil {
		return -1, errExec
	}
	return v.RowsAffected(), nil
}

func (crdbp *CRDBPersistence) UpdateStatus(ctx context.Context, serverUUID uuid.UUID, status pgtype.Int2, tx *WrappedTx) error {
	_, errExec := (*tx.Tx).Exec(ctx,
		"UPDATE notifications SET status=$1, last_updated=$2 "+
			"WHERE uuid = $3",
		status,
		pgtype.Timestamp{
			Time:   time.Now().UTC(),
			Status: pgtype.Present,
		},
		serverUUID,
	)
	if errExec != nil {
		return errExec
	}
	return nil
}

func (crdbp *CRDBPersistence) GetForUpdate(ctx context.Context, serverUUID uuid.UUID, txx *WrappedTx) (*Notification, error) {
	notification := &Notification{}
	r := (*txx.Tx).QueryRow(ctx, "SELECT uuid, txt, status, destination, server_timestamp, last_updated FROM notifications WHERE uuid = $1 FOR UPDATE", serverUUID)
	errScan := r.Scan(&notification.UUID, &notification.Txt, &notification.Status, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return notification, nil
}

func (crdbp *CRDBPersistence) Get(ctx context.Context, serverUUID uuid.UUID, txx *WrappedTx) (*Notification, error) {
	notification := &Notification{}
	r := (*txx.Tx).QueryRow(ctx, "SELECT uuid, txt, status, destination, server_timestamp, last_updated FROM notifications WHERE uuid = $1", serverUUID)
	errScan := r.Scan(&notification.UUID, &notification.Txt, &notification.Status, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return notification, nil
}

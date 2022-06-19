package storage

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
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

func (crdbp *CRDBPersistence) GetPool() *pgxpool.Pool {
	return crdbp.connPool
}

func (crdbp *CRDBPersistence) InsertOnConflictNothing(ctx context.Context, notification *Notification, tx pgx.Tx) (int64, error) {
	v, errExec := tx.Exec(ctx,
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

func (crdbp *CRDBPersistence) UpdateStatus(ctx context.Context, serverUUID uuid.UUID, status pgtype.Int2, tx pgx.Tx) error {
	_, errExec := tx.Exec(ctx,
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

func (crdbp *CRDBPersistence) GetForUpdate(ctx context.Context, serverUUID uuid.UUID, tx pgx.Tx) (*Notification, error) {
	notification := &Notification{}
	r := tx.QueryRow(ctx, "SELECT uuid, txt, status, destination, server_timestamp, last_updated FROM notifications WHERE uuid = $1 FOR UPDATE", serverUUID)
	errScan := r.Scan(&notification.UUID, &notification.Txt, &notification.Status, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return notification, nil
}

func (crdbp *CRDBPersistence) Get(ctx context.Context, notifUUID uuid.UUID, tx pgx.Tx) (*Notification, error) {
	r := tx.QueryRow(ctx, "SELECT uuid, txt, status, destination, server_timestamp, last_updated FROM notifications WHERE uuid = $1", notifUUID)
	notification := &Notification{}
	errScan := r.Scan(&notification.UUID, &notification.Txt, &notification.Status, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return notification, nil
}

package storage

import (
	"context"
	"fmt"
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

func (crdbp *CRDBPersistence) InsertOnConflictNothing(ctx context.Context, notification *Notification, tx *WrappedTx) error {
	cmd, errExec := (*tx.Tx).Exec(ctx,
		"INSERT into notifications(server_uuid, txt, status, destination, server_timestamp, last_updated) "+
			"VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT(server_uuid) DO NOTHING",
		notification.ServerUUID,
		notification.Txt,
		notification.Status,
		notification.Dest,
		notification.ServerTimestamp,
		notification.LastUpdated,
	)
	if errExec != nil {
		return errExec
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("expected to affect 1 row, but affected %d", cmd.RowsAffected())
	}
	return nil
}

func (crdbp *CRDBPersistence) UpdateStatus(ctx context.Context, serverUUID uuid.UUID, status pgtype.Int2, tx *WrappedTx) error {
	cmd, errExec := (*tx.Tx).Exec(ctx,
		"UPDATE notifications SET status=$1, last_updated=$2 "+
			"WHERE server_uuid = $3",
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
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("expected to affect 1 row, but affected %d", cmd.RowsAffected())
	}
	return nil
}

func (crdbp *CRDBPersistence) GetForUpdate(ctx context.Context, serverUUID uuid.UUID, txx *WrappedTx) (*Notification, error) {
	notification := &Notification{}
	r := (*txx.Tx).QueryRow(ctx, "SELECT server_uuid, txt, status, destination, server_timestamp, last_updated FROM notifications WHERE server_uuid = $1 FOR UPDATE", serverUUID)
	errScan := r.Scan(&notification.ServerUUID, &notification.Txt, &notification.Status, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return notification, nil
}

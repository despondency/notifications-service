package notification

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type storageModel struct {
	UUID            uuid.UUID
	Txt             string
	Dest            pgtype.Int2
	ServerTimestamp pgtype.Timestamp
	LastUpdated     pgtype.Timestamp
}

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

func (crdbp *CRDBPersistence) InsertIfNotExists(ctx context.Context, notification *Notification, tx pgx.Tx) (int64, error) {
	dbNotificationModel := toDBNotification(notification)
	v, errExec := tx.Exec(ctx,
		"INSERT into notifications(uuid, txt, destination, server_timestamp, last_updated) "+
			"VALUES ($1, $2, $3, $4, $5) ON CONFLICT(uuid) DO NOTHING",
		dbNotificationModel.UUID,
		dbNotificationModel.Txt,
		dbNotificationModel.Dest,
		dbNotificationModel.ServerTimestamp,
		dbNotificationModel.LastUpdated,
	)
	if errExec != nil {
		return -1, errExec
	}
	return v.RowsAffected(), nil
}

func (crdbp *CRDBPersistence) Get(ctx context.Context, notifUUID uuid.UUID, tx pgx.Tx) (*Notification, error) {
	r := tx.QueryRow(ctx, "SELECT uuid, txt, destination, server_timestamp, last_updated FROM notifications WHERE uuid = $1", notifUUID)
	notification := &storageModel{}
	errScan := r.Scan(&notification.UUID, &notification.Txt, &notification.Dest, &notification.ServerTimestamp, &notification.LastUpdated)
	if errScan != nil {
		return nil, errScan
	}
	return &Notification{
		UUID:                    notification.UUID,
		ServerReceivedTimestamp: notification.ServerTimestamp.Time,
		NotificationTxt:         notification.Txt,
		Dest:                    Destination(notification.Dest.Int),
	}, nil
}

func toDBNotification(n *Notification) *storageModel {
	return &storageModel{
		UUID: n.UUID,
		Txt:  n.NotificationTxt,
		Dest: pgtype.Int2{
			Int:    int16(n.Dest),
			Status: pgtype.Present,
		},
		ServerTimestamp: pgtype.Timestamp{
			Time:   n.ServerReceivedTimestamp.UTC(),
			Status: pgtype.Present,
		},
		LastUpdated: pgtype.Timestamp{
			Time:   n.ServerReceivedTimestamp.UTC(),
			Status: pgtype.Present,
		},
	}
}

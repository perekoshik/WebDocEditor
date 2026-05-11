package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Logger {
	return &Logger{pool: pool, log: log}
}

func (l *Logger) Record(ctx context.Context, action string, docID *uuid.UUID, metadata map[string]any) {
	meta, err := json.Marshal(metadata)
	if err != nil {
		meta = []byte("{}")
	}
	if _, err := l.pool.Exec(ctx, `
        insert into audit_log (action, document_id, metadata)
        values ($1, $2, $3)
    `, action, docID, meta); err != nil {
		l.log.Warn("audit insert failed", "err", err, "action", action)
	}
}

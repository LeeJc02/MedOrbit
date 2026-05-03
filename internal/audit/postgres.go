package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrForbidden = errors.New("forbidden")

type PostgresLogger struct {
	pool *pgxpool.Pool
}

func NewPostgresLogger(pool *pgxpool.Pool) *PostgresLogger {
	return &PostgresLogger{pool: pool}
}

func (l *PostgresLogger) Log(ctx context.Context, event Event) error {
	_, err := l.pool.Exec(ctx,
		`INSERT INTO audit_logs(session_id, tenant_id, user_id, event_type, payload) VALUES ($1, $2, $3, $4, $5)`,
		event.SessionID, event.TenantID, event.UserID, event.Type, event.Payload,
	)
	return err
}

func (l *PostgresLogger) Replay(ctx context.Context, sessionID, tenantID string) ([]string, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT event_type, payload FROM audit_logs WHERE session_id = $1 AND tenant_id = $2 ORDER BY created_at ASC, id ASC`,
		sessionID, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []string
	for rows.Next() {
		var eventType string
		var payload string
		if err := rows.Scan(&eventType, &payload); err != nil {
			return nil, err
		}
		events = append(events, formatEvent(eventType, payload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrForbidden
	}
	return events, nil
}

func formatEvent(eventType, payload string) string {
	return fmt.Sprintf("%s:%s", eventType, payload)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  session_id TEXT NOT NULL,
  tenant_id TEXT,
  user_id TEXT,
  event_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_id TEXT;
CREATE INDEX IF NOT EXISTS idx_audit_logs_session_id ON audit_logs (session_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_session_tenant ON audit_logs (session_id, tenant_id);
`)
	return err
}

package audit

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFormatEvent(t *testing.T) {
	got := formatEvent("gateway.request", `{"ok":true}`)
	want := `gateway.request:{"ok":true}`
	if got != want {
		t.Fatalf("formatEvent() = %q, want %q", got, want)
	}
}

func TestPostgresLoggerWriteReplayIntegration(t *testing.T) {
	dsn := os.Getenv("DDI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set DDI_TEST_PG_DSN to run Postgres audit integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pg connect: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  session_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
)`)
	if err != nil {
		t.Fatalf("create audit table: %v", err)
	}
	if err := EnsureSchema(ctx, pool); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sessionID := "test-audit-replay"
	_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE session_id = $1`, sessionID)
	logger := NewPostgresLogger(pool)

	if err := logger.Log(ctx, Event{SessionID: sessionID, TenantID: "tenant-1", UserID: "u1", Type: "gateway.request", Payload: `{"step":1}`}); err != nil {
		t.Fatalf("log request: %v", err)
	}
	if err := logger.Log(ctx, Event{SessionID: sessionID, TenantID: "tenant-1", UserID: "u1", Type: "gateway.response", Payload: `{"step":2}`}); err != nil {
		t.Fatalf("log response: %v", err)
	}

	var tenantID, userID string
	if err := pool.QueryRow(ctx, `SELECT tenant_id, user_id FROM audit_logs WHERE session_id = $1 ORDER BY id ASC LIMIT 1`, sessionID).Scan(&tenantID, &userID); err != nil {
		t.Fatalf("read tenant/user columns: %v", err)
	}
	if tenantID != "tenant-1" || userID != "u1" {
		t.Fatalf("tenant/user = %q/%q, want tenant-1/u1", tenantID, userID)
	}

	got, err := logger.Replay(ctx, sessionID, "tenant-1")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	want := []string{`gateway.request:{"step":1}`, `gateway.response:{"step":2}`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, want %#v", got, want)
	}

	if _, err := logger.Replay(ctx, sessionID, "tenant-2"); err != ErrForbidden {
		t.Fatalf("cross-tenant replay error = %v, want %v", err, ErrForbidden)
	}
}

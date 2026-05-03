package audit

import "context"

type Event struct {
	SessionID string
	TenantID  string
	UserID    string
	Type      string
	Payload   string
}

type Logger interface {
	Log(ctx context.Context, event Event) error
	Replay(ctx context.Context, sessionID, tenantID string) ([]string, error)
}

type NoopLogger struct{}

func (NoopLogger) Log(_ context.Context, _ Event) error { return nil }
func (NoopLogger) Replay(_ context.Context, _, _ string) ([]string, error) {
	return []string{}, nil
}

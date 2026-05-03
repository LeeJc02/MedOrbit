package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ddi/gen"
	"ddi/internal/audit"
	"ddi/internal/authctx"
)

type fakeRuntimeClient struct {
	replayCalls int
}

func (c *fakeRuntimeClient) RunSession(_ context.Context, req *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error) {
	return &orchestratorpb.RunSessionResponse{SessionId: req.SessionId, RiskLevel: "LOW"}, nil
}

func (c *fakeRuntimeClient) Replay(_ context.Context, req *orchestratorpb.ReplayRequest) (*orchestratorpb.ReplayResponse, error) {
	c.replayCalls++
	return &orchestratorpb.ReplayResponse{SessionId: req.SessionId, Events: []string{"runtime.request:{}"}}, nil
}

type fakeAuditLogger struct {
	events []audit.Event
}

func (l *fakeAuditLogger) Log(_ context.Context, event audit.Event) error {
	l.events = append(l.events, event)
	return nil
}

func (l *fakeAuditLogger) Replay(_ context.Context, sessionID, tenantID string) ([]string, error) {
	for _, event := range l.events {
		if event.SessionID == sessionID && event.TenantID == tenantID {
			return []string{"gateway.request:{}"}, nil
		}
	}
	return nil, audit.ErrForbidden
}

func TestRunSessionAuditIncludesTenantAndUser(t *testing.T) {
	logger := &fakeAuditLogger{}
	service := New(&fakeRuntimeClient{}, logger)
	ctx := authctx.WithClaims(context.Background(), authctx.Claims{UserID: "u1", TenantID: "tenant-1", Roles: []string{"doctor"}})

	_, err := service.RunSession(ctx, &orchestratorpb.RunSessionRequest{SessionId: "s1", UserId: "u1", InputText: "aspirin"})
	if err != nil {
		t.Fatalf("run session: %v", err)
	}
	if len(logger.events) != 2 {
		t.Fatalf("events = %d, want 2", len(logger.events))
	}
	for _, event := range logger.events {
		if event.TenantID != "tenant-1" || event.UserID != "u1" || event.SessionID != "s1" {
			t.Fatalf("event identity = %#v", event)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(logger.events[0].Payload), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["session_id"] != "s1" || payload["user_id"] != "u1" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestReplayAllowsSameTenantAndAppendsRuntimeEvents(t *testing.T) {
	logger := &fakeAuditLogger{events: []audit.Event{{SessionID: "s1", TenantID: "tenant-1"}}}
	client := &fakeRuntimeClient{}
	service := New(client, logger)
	ctx := authctx.WithClaims(context.Background(), authctx.Claims{UserID: "u1", TenantID: "tenant-1", Roles: []string{"doctor"}})

	events, err := service.Replay(ctx, "s1")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(events) != 2 || events[0] != "gateway.request:{}" || events[1] != "runtime.request:{}" {
		t.Fatalf("events = %#v", events)
	}
	if client.replayCalls != 1 {
		t.Fatalf("runtime replay calls = %d, want 1", client.replayCalls)
	}
}

func TestReplayRejectsCrossTenantBeforeRuntimeReplay(t *testing.T) {
	logger := &fakeAuditLogger{events: []audit.Event{{SessionID: "s1", TenantID: "tenant-1"}}}
	client := &fakeRuntimeClient{}
	service := New(client, logger)
	ctx := authctx.WithClaims(context.Background(), authctx.Claims{UserID: "u2", TenantID: "tenant-2", Roles: []string{"doctor"}})

	_, err := service.Replay(ctx, "s1")
	if !errors.Is(err, audit.ErrForbidden) {
		t.Fatalf("replay error = %v, want %v", err, audit.ErrForbidden)
	}
	if client.replayCalls != 0 {
		t.Fatalf("runtime replay calls = %d, want 0", client.replayCalls)
	}
}

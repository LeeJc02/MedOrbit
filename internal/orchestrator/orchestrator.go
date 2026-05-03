package orchestrator

import (
	"context"
	"encoding/json"
	"log"

	"ddi/gen"
	"ddi/internal/audit"
	"ddi/internal/authctx"
)

type Service struct {
	client runtimeClient
	logger audit.Logger
}

type runtimeClient interface {
	RunSession(context.Context, *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error)
	Replay(context.Context, *orchestratorpb.ReplayRequest) (*orchestratorpb.ReplayResponse, error)
}

func New(client runtimeClient, logger audit.Logger) *Service {
	return &Service{client: client, logger: logger}
}

func (s *Service) RunSession(ctx context.Context, req *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error) {
	claims, _ := authctx.FromContext(ctx)

	payload, _ := json.Marshal(req)
	if err := s.logger.Log(ctx, audit.Event{
		SessionID: req.SessionId,
		TenantID:  claims.TenantID,
		UserID:    claims.UserID,
		Type:      "gateway.request",
		Payload:   string(payload),
	}); err != nil {
		log.Printf("audit log gateway.request failed session_id=%s: %v", req.SessionId, err)
	}

	resp, err := s.client.RunSession(ctx, req)
	if err != nil {
		return nil, err
	}

	payload, _ = json.Marshal(resp)
	if err := s.logger.Log(ctx, audit.Event{
		SessionID: req.SessionId,
		TenantID:  claims.TenantID,
		UserID:    claims.UserID,
		Type:      "gateway.response",
		Payload:   string(payload),
	}); err != nil {
		log.Printf("audit log gateway.response failed session_id=%s: %v", req.SessionId, err)
	}
	return resp, nil
}

func (s *Service) Replay(ctx context.Context, sessionID string) ([]string, error) {
	claims, _ := authctx.FromContext(ctx)
	events, err := s.logger.Replay(ctx, sessionID, claims.TenantID)
	if err != nil {
		return nil, err
	}

	runtimeResp, err := s.client.Replay(ctx, &orchestratorpb.ReplayRequest{SessionId: sessionID})
	if err != nil {
		log.Printf("runtime replay failed session_id=%s: %v", sessionID, err)
		return events, nil
	}
	events = append(events, runtimeResp.Events...)
	return events, nil
}

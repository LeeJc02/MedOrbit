package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"ddi/gen"
	"ddi/internal/audit"
	"ddi/internal/authctx"
	"github.com/gin-gonic/gin"
)

type stubService struct {
	runResp   *orchestratorpb.RunSessionResponse
	runErr    error
	runReq    *orchestratorpb.RunSessionRequest
	replayErr error
}

func (s *stubService) RunSession(_ context.Context, req *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error) {
	s.runReq = req
	return s.runResp, s.runErr
}

func (s *stubService) Replay(_ context.Context, _ string) ([]string, error) {
	return []string{"gateway.request:{}"}, s.replayErr
}

func TestRunSessionBadRequest(t *testing.T) {
	w := postRunSession(t, &stubService{}, `{}`)
	if w.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, stdhttp.StatusBadRequest)
	}
}

func TestRunSessionServiceError(t *testing.T) {
	w := postRunSession(t, &stubService{runErr: errors.New("runtime failed")}, validRunSessionBody())
	if w.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, stdhttp.StatusInternalServerError)
	}
}

func TestRunSessionSuccess(t *testing.T) {
	svc := &stubService{
		runResp: &orchestratorpb.RunSessionResponse{
			SessionId: "s1",
			RiskLevel: "MEDIUM",
			Draft:     "draft",
		},
	}

	w := postRunSession(t, svc, validRunSessionBody())
	if w.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, stdhttp.StatusOK, w.Body.String())
	}
	if svc.runReq == nil || svc.runReq.SessionId != "s1" || svc.runReq.UserId != "token-user" {
		t.Fatalf("service received request = %#v", svc.runReq)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["session_id"] != "s1" {
		t.Fatalf("session_id = %v, want s1", body["session_id"])
	}
}

func TestReplayForbidden(t *testing.T) {
	w := postReplay(t, &stubService{replayErr: audit.ErrForbidden}, `{"session_id":"s1"}`)
	if w.Code != stdhttp.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, stdhttp.StatusForbidden)
	}
}

func TestReplaySuccess(t *testing.T) {
	w := postReplay(t, &stubService{}, `{"session_id":"s1"}`)
	if w.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, stdhttp.StatusOK, w.Body.String())
	}
}

func postRunSession(t *testing.T, svc Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := New(svc)
	router.POST("/v1/session/run", handler.RunSession)

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/session/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(authctx.WithClaims(req.Context(), authctx.Claims{UserID: "token-user", TenantID: "tenant-1", Roles: []string{"doctor"}}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func postReplay(t *testing.T, svc Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := New(svc)
	router.POST("/v1/session/replay", handler.Replay)

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/session/replay", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(authctx.WithClaims(req.Context(), authctx.Claims{UserID: "token-user", TenantID: "tenant-1", Roles: []string{"doctor"}}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func validRunSessionBody() string {
	return `{"session_id":"s1","user_id":"u1","locale":"zh-CN","input_text":"aspirin and warfarin","metadata":{"source":"test"}}`
}

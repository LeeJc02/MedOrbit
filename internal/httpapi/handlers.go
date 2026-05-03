package httpapi

import (
	"context"
	"errors"
	"net/http"

	"ddi/gen"
	"ddi/internal/audit"
	"ddi/internal/authctx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

type Service interface {
	RunSession(context.Context, *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error)
	Replay(context.Context, string) ([]string, error)
}

func New(service Service) *Handler {
	return &Handler{service: service}
}

type RunSessionRequest struct {
	SessionID string            `json:"session_id" binding:"required"`
	UserID    string            `json:"user_id" binding:"required"`
	Locale    string            `json:"locale"`
	InputText string            `json:"input_text" binding:"required"`
	Metadata  map[string]string `json:"metadata"`
}

type ReplayRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

type RunSessionResponse struct {
	SessionID string          `json:"session_id"`
	Claims    []ClaimResponse `json:"claims"`
	Draft     string          `json:"draft"`
	RiskLevel string          `json:"risk_level"`
	Followups []string        `json:"followups"`
}

type ClaimResponse struct {
	Text     string             `json:"text"`
	Evidence []EvidenceResponse `json:"evidence"`
	Degraded bool               `json:"degraded"`
}

type EvidenceResponse struct {
	Source      string `json:"source"`
	URI         string `json:"uri"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	Region      string `json:"region"`
	Version     string `json:"version"`
	PublishedAt string `json:"published_at"`
}

func (h *Handler) RunSession(c *gin.Context) {
	var req RunSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims, hasClaims := authctx.FromContext(c.Request.Context())
	userID := req.UserID
	if hasClaims {
		userID = claims.UserID
	}

	resp, err := h.service.RunSession(c.Request.Context(), &orchestratorpb.RunSessionRequest{
		SessionId: req.SessionID,
		UserId:    userID,
		Locale:    req.Locale,
		InputText: req.InputText,
		Metadata:  req.Metadata,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, newRunSessionResponse(resp))
}

func (h *Handler) Replay(c *gin.Context) {
	var req ReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events, err := h.service.Replay(c.Request.Context(), req.SessionID)
	if err != nil {
		if errors.Is(err, audit.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": req.SessionID, "events": events})
}

func newRunSessionResponse(resp *orchestratorpb.RunSessionResponse) RunSessionResponse {
	followups := resp.Followups
	if followups == nil {
		followups = []string{}
	}
	claims := make([]ClaimResponse, 0, len(resp.Claims))
	for _, claim := range resp.Claims {
		evidence := make([]EvidenceResponse, 0, len(claim.Evidence))
		for _, ev := range claim.Evidence {
			evidence = append(evidence, EvidenceResponse{
				Source:      ev.Source,
				URI:         ev.Uri,
				Title:       ev.Title,
				Snippet:     ev.Snippet,
				Region:      ev.Region,
				Version:     ev.Version,
				PublishedAt: ev.PublishedAt,
			})
		}
		claims = append(claims, ClaimResponse{
			Text:     claim.Text,
			Evidence: evidence,
			Degraded: claim.Degraded,
		})
	}
	return RunSessionResponse{
		SessionID: resp.SessionId,
		Claims:    claims,
		Draft:     resp.Draft,
		RiskLevel: resp.RiskLevel,
		Followups: followups,
	}
}

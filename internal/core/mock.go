package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PCStatus struct {
	ExternalPCID     string     `json:"external_pc_id"`
	Status           string     `json:"status"`
	CurrentSessionID string     `json:"current_session_id,omitempty"`
	CurrentGrantID   string     `json:"current_grant_id,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndsAt           *time.Time `json:"ends_at,omitempty"`
	RemainingSeconds int        `json:"remaining_seconds"`
	AgentOnline      bool       `json:"agent_online"`
	ControllerOnline bool       `json:"controller_online"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
}

type StartSessionCommand struct {
	RequestID       string `json:"request_id"`
	GrantID         string `json:"grant_id"`
	ClubID          string `json:"club_id"`
	PCID            string `json:"pc_id"`
	PCExternalID    string `json:"external_pc_id"`
	DurationSeconds int    `json:"duration_seconds"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	Source          string `json:"source"`
	PaymentOrderID  string `json:"payment_order_id,omitempty"`
	InvoiceID       string `json:"invoice_id,omitempty"`
	ExtendURL       string `json:"extend_url,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type StartSessionResult struct {
	Status        string     `json:"status"`
	CoreSessionID string     `json:"core_session_id"`
	ExternalPCID  string     `json:"external_pc_id"`
	GrantID       string     `json:"grant_id"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	Message       string     `json:"message,omitempty"`
}

type ExtendSessionCommand struct {
	RequestID      string `json:"request_id"`
	GrantID        string `json:"grant_id"`
	ClubID         string `json:"club_id"`
	ExternalPCID   string `json:"external_pc_id"`
	AddSeconds     int    `json:"add_seconds"`
	Source         string `json:"source"`
	PaymentOrderID string `json:"payment_order_id,omitempty"`
	InvoiceID      string `json:"invoice_id,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type ExtendSessionResult struct {
	Status           string     `json:"status"`
	CoreSessionID    string     `json:"core_session_id"`
	GrantID          string     `json:"grant_id"`
	OldEndsAt        *time.Time `json:"old_ends_at,omitempty"`
	NewEndsAt        *time.Time `json:"new_ends_at,omitempty"`
	RemainingSeconds int        `json:"remaining_seconds"`
	Reason           string     `json:"reason,omitempty"`
	Message          string     `json:"message,omitempty"`
}

type EndSessionCommand struct {
	RequestID    string            `json:"request_id"`
	ExternalPCID string            `json:"external_pc_id,omitempty"`
	Reason       string            `json:"reason"`
	EndedBy      map[string]string `json:"ended_by,omitempty"`
	CreatedAt    string            `json:"created_at"`
}

type EndSessionResult struct {
	Status           string     `json:"status"`
	CoreSessionID    string     `json:"core_session_id"`
	ExternalPCID     string     `json:"external_pc_id"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	PlannedEndsAt    *time.Time `json:"planned_ends_at,omitempty"`
	RemainingSeconds int        `json:"remaining_seconds"`
	Reason           string     `json:"reason,omitempty"`
	Message          string     `json:"message,omitempty"`
}

type Adapter interface {
	GetPCStatus(ctx context.Context, externalPCID string) (PCStatus, error)
	StartSession(ctx context.Context, cmd StartSessionCommand) (StartSessionResult, error)
	ExtendSession(ctx context.Context, coreSessionID string, cmd ExtendSessionCommand) (ExtendSessionResult, error)
	EndSession(ctx context.Context, coreSessionID string, cmd EndSessionCommand) (EndSessionResult, error)
	Lock(ctx context.Context, externalPCID, reason string) error
	Unlock(ctx context.Context, externalPCID, reason string) error
	Wake(ctx context.Context, externalPCID string) error
	Sleep(ctx context.Context, externalPCID string) error
	SetRepair(ctx context.Context, externalPCID string, on bool) error
}

type MockAdapter struct{}

func (MockAdapter) GetPCStatus(ctx context.Context, externalPCID string) (PCStatus, error) {
	now := time.Now().UTC()
	return PCStatus{
		ExternalPCID:     externalPCID,
		Status:           "available",
		AgentOnline:      true,
		ControllerOnline: true,
		LastSeenAt:       &now,
	}, nil
}

func (MockAdapter) StartSession(ctx context.Context, cmd StartSessionCommand) (StartSessionResult, error) {
	if cmd.PCExternalID == "" {
		return StartSessionResult{}, fmt.Errorf("pc external id is required")
	}
	startedAt := time.Now().UTC()
	endsAt := startedAt.Add(time.Duration(cmd.DurationSeconds) * time.Second)
	return StartSessionResult{
		Status:        "accepted",
		CoreSessionID: fmt.Sprintf("mock-session-%s-%d", cmd.PCExternalID, startedAt.Unix()),
		ExternalPCID:  cmd.PCExternalID,
		GrantID:       cmd.GrantID,
		StartedAt:     &startedAt,
		EndsAt:        &endsAt,
	}, nil
}

func (MockAdapter) ExtendSession(ctx context.Context, coreSessionID string, cmd ExtendSessionCommand) (ExtendSessionResult, error) {
	now := time.Now().UTC()
	newEndsAt := now.Add(time.Duration(cmd.AddSeconds) * time.Second)
	return ExtendSessionResult{
		Status:           "accepted",
		CoreSessionID:    coreSessionID,
		GrantID:          cmd.GrantID,
		NewEndsAt:        &newEndsAt,
		RemainingSeconds: cmd.AddSeconds,
	}, nil
}

func (MockAdapter) EndSession(ctx context.Context, coreSessionID string, cmd EndSessionCommand) (EndSessionResult, error) {
	now := time.Now().UTC()
	return EndSessionResult{
		Status:        "ended",
		CoreSessionID: coreSessionID,
		EndedAt:       &now,
	}, nil
}

func (MockAdapter) Lock(ctx context.Context, externalPCID, reason string) error {
	return nil
}

func (MockAdapter) Unlock(ctx context.Context, externalPCID, reason string) error {
	return nil
}

func (MockAdapter) Wake(ctx context.Context, externalPCID string) error {
	return nil
}

func (MockAdapter) Sleep(ctx context.Context, externalPCID string) error {
	return nil
}

func (MockAdapter) SetRepair(ctx context.Context, externalPCID string, on bool) error {
	return nil
}

type HTTPAdapter struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewHTTPAdapter(baseURL, token string, timeout time.Duration) *HTTPAdapter {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (a *HTTPAdapter) GetPCStatus(ctx context.Context, externalPCID string) (PCStatus, error) {
	var result PCStatus
	err := a.doJSON(ctx, http.MethodGet, "/core/v1/pcs/"+externalPCID+"/status", nil, &result)
	return result, err
}

func (a *HTTPAdapter) StartSession(ctx context.Context, cmd StartSessionCommand) (StartSessionResult, error) {
	var result StartSessionResult
	if err := a.doJSON(ctx, http.MethodPost, "/core/v1/sessions/start", cmd, &result); err != nil {
		return StartSessionResult{}, err
	}
	if result.Status == "rejected" || result.Status == "failed" {
		return result, fmt.Errorf("core start rejected: %s %s", result.Reason, result.Message)
	}
	return result, nil
}

func (a *HTTPAdapter) ExtendSession(ctx context.Context, coreSessionID string, cmd ExtendSessionCommand) (ExtendSessionResult, error) {
	var result ExtendSessionResult
	if err := a.doJSON(ctx, http.MethodPost, "/core/v1/sessions/"+coreSessionID+"/extend", cmd, &result); err != nil {
		return ExtendSessionResult{}, err
	}
	if result.Status == "rejected" || result.Status == "failed" {
		return result, fmt.Errorf("core extend rejected: %s %s", result.Reason, result.Message)
	}
	return result, nil
}

func (a *HTTPAdapter) EndSession(ctx context.Context, coreSessionID string, cmd EndSessionCommand) (EndSessionResult, error) {
	var result EndSessionResult
	if err := a.doJSON(ctx, http.MethodPost, "/core/v1/sessions/"+coreSessionID+"/end", cmd, &result); err != nil {
		return EndSessionResult{}, err
	}
	if result.Status == "rejected" || result.Status == "failed" {
		return result, fmt.Errorf("core end rejected: %s %s", result.Reason, result.Message)
	}
	return result, nil
}

func (a *HTTPAdapter) Lock(ctx context.Context, externalPCID, reason string) error {
	return a.doJSON(ctx, http.MethodPost, "/core/v1/pcs/"+externalPCID+"/lock", map[string]string{"reason": reason}, &struct{}{})
}

func (a *HTTPAdapter) Unlock(ctx context.Context, externalPCID, reason string) error {
	return a.doJSON(ctx, http.MethodPost, "/core/v1/pcs/"+externalPCID+"/unlock", map[string]string{"reason": reason}, &struct{}{})
}

func (a *HTTPAdapter) Wake(ctx context.Context, externalPCID string) error {
	return a.doJSON(ctx, http.MethodPost, "/core/v1/pcs/"+externalPCID+"/wake", nil, &struct{}{})
}

func (a *HTTPAdapter) Sleep(ctx context.Context, externalPCID string) error {
	return a.doJSON(ctx, http.MethodPost, "/core/v1/pcs/"+externalPCID+"/sleep", nil, &struct{}{})
}

func (a *HTTPAdapter) SetRepair(ctx context.Context, externalPCID string, on bool) error {
	return a.doJSON(ctx, http.MethodPost, "/core/v1/pcs/"+externalPCID+"/repair", map[string]bool{"on": on}, &struct{}{})
}

func (a *HTTPAdapter) doJSON(ctx context.Context, method, path string, body any, target any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("core http %d: %v", resp.StatusCode, errBody)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

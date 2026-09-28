package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"clubpay/internal/config"
	"clubpay/internal/core"
)

func TestNodeSyncRequiresCoreTokenOnBothLocalNodeModes(t *testing.T) {
	for _, mode := range []string{"edge", "manager"} {
		t.Run(mode, func(t *testing.T) {
			s := NewServer(config.Config{NodeMode: mode, CoreToken: "test-secret"}, nil, core.NewMockAdapter())
			recorder := httptest.NewRecorder()
			s.handleNodeSync(recorder, httptest.NewRequest(http.MethodPost, "/api/node/sync", nil))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("node mode %s returned %d without a Core token, want %d", mode, recorder.Code, http.StatusUnauthorized)
			}
		})
	}

	s := NewServer(config.Config{NodeMode: "cloud", CoreToken: "test-secret"}, nil, core.NewMockAdapter())
	recorder := httptest.NewRecorder()
	s.handleNodeSync(recorder, httptest.NewRequest(http.MethodPost, "/api/node/sync", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cloud node returned %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

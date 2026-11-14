package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRelayLightModeRequiresConfirmationAndPreservesSettings(t *testing.T) {
	newHostTestDB(t)
	a := &SettingController{}
	restarts := 0
	a.scheduleRelayModeReload = func(delay time.Duration) error {
		if delay <= 0 {
			t.Fatal("panel reload must be delayed until after the response")
		}
		restarts++
		return nil
	}
	if err := a.settingService.SetTgbotEnabled(true); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	a.initRouter(engine.Group("/"))
	for _, tt := range []struct {
		body        string
		contentType string
		query       string
		code        int
		enabled     bool
		restarts    int
	}{
		{`{"enabled":true}`, "application/json", "", http.StatusBadRequest, false, 0},
		{`{"enabled":true,"confirmed":false}`, "application/json", "", http.StatusBadRequest, false, 0},
		{`{"confirmed":true}`, "application/json", "", http.StatusBadRequest, false, 0},
		{"enabled=true", "application/x-www-form-urlencoded; charset=UTF-8", "?confirmed=true", http.StatusBadRequest, false, 0},
		{"enabled=true&confirmed=true", "application/x-www-form-urlencoded; charset=UTF-8", "", http.StatusOK, true, 1},
		{`{"enabled":true,"confirmed":true}`, "application/json", "", http.StatusOK, true, 1},
		{`{"enabled":false,"confirmed":true}`, "application/json", "", http.StatusOK, false, 2},
		{"enabled=true&confirmed=true", "text/plain", "", http.StatusUnsupportedMediaType, false, 2},
	} {
		req := httptest.NewRequest(http.MethodPost, "/setting/relayLightMode"+tt.query, strings.NewReader(tt.body))
		req.Header.Set("Content-Type", tt.contentType)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != tt.code || restarts != tt.restarts {
			t.Fatalf("%s: code=%d, reloads=%d, body=%s", tt.body, w.Code, restarts, w.Body.String())
		}
		if enabled, err := a.settingService.GetRelayLightMode(); err != nil || enabled != tt.enabled {
			t.Fatalf("saved setting: %t, %v", enabled, err)
		}
		if telegram, err := a.settingService.GetTgbotEnabled(); err != nil || !telegram {
			t.Fatalf("Telegram configuration changed: %t, %v", telegram, err)
		}
		if tt.code == http.StatusOK {
			var response hostEnvelope
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || !response.Success {
				t.Fatalf("unexpected response: %s (%v)", w.Body.String(), err)
			}
		}
	}
}

package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRemoteResetClientTrafficsPostsEmailsToBulkEndpoint(t *testing.T) {
	var path string
	var body struct {
		Emails []string `json:"emails"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	if err := r.ResetClientTraffics(context.Background(), []string{"a@x", "b@x"}); err != nil {
		t.Fatalf("ResetClientTraffics: %v", err)
	}
	if path != "/panel/api/clients/bulkResetTraffic" || !slices.Equal(body.Emails, []string{"a@x", "b@x"}) {
		t.Fatalf("node got %s %v, want /panel/api/clients/bulkResetTraffic [a@x b@x]", path, body.Emails)
	}
}

func TestRemoteResetInboundTrafficUsesNodeInboundID(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		method, path = req.Method, req.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"msg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	r.cacheSet("n1-in-443", 7)
	ib := &model.Inbound{Id: 42, Tag: "n1-in-443"}
	if err := r.ResetInboundTraffic(context.Background(), ib); err != nil {
		t.Fatalf("ResetInboundTraffic: %v", err)
	}
	if method != http.MethodPost || path != "/panel/api/inbounds/7/resetTraffic" {
		t.Fatalf("node got %s %s, want POST /panel/api/inbounds/7/resetTraffic", method, path)
	}
}

func TestRemoteResetInboundTrafficUnknownTagErrors(t *testing.T) {
	var resetPosted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/panel/api/inbounds/list" {
			_, _ = w.Write([]byte(`{"success":true,"msg":"ok","obj":[]}`))
			return
		}
		resetPosted = true
		_, _ = w.Write([]byte(`{"success":true,"msg":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	r := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ib := &model.Inbound{Id: 42, Tag: "n1-in-443"}
	if err := r.ResetInboundTraffic(context.Background(), ib); err == nil {
		t.Fatal("ResetInboundTraffic error = nil, want unknown-tag error")
	}
	if resetPosted {
		t.Fatal("reset request posted to node despite an unresolved tag")
	}
}

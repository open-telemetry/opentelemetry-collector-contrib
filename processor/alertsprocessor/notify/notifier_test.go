package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/statestore"
)

func TestNotifier_SendTransitions(t *testing.T) {
	var got []AMAlert
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := New(Config{
		URL:            srv.URL,
		Timeout:        2 * time.Second,
		MaxBatchSize:   100,
		DisableSending: false,
	}, zap.NewNop())

	now := time.Now()
	trans := []statestore.Transition{
		{From: "inactive", To: "firing", Labels: map[string]string{"rule_id": "r1"}, At: now},
		{From: "firing", To: "resolved", Labels: map[string]string{"rule_id": "r1"}, At: now},
	}

	n.Notify(context.Background(), trans)

	if len(got) != 2 {
		t.Fatalf("expected 2 alerts posted, got %d", len(got))
	}
	if got[0].Status != "firing" || got[1].Status != "resolved" {
		t.Fatalf("unexpected statuses: %+v", got)
	}
}

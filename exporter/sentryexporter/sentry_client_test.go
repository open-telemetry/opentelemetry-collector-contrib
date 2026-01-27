// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGetAllProjectsPagination(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	var callCount int32

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)

		if r.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/api/0/organizations/test-org/projects/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		cursor := r.URL.Query().Get("cursor")
		switch cursor {
		case "":
			w.Header().Set("Link", `<`+server.URL+`/api/0/organizations/test-org/projects/?cursor=0:100:0>; rel="next"; results="true"; cursor="0:100:0"`)
			if err := json.NewEncoder(w).Encode([]projectInfo{
				{ID: "1", Slug: "proj-1", Name: "Project 1", Team: teamInfo{ID: "t1", Slug: "team", Name: "Team"}},
			}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		case "0:100:0":
			w.Header().Set("Link", `<`+server.URL+`/api/0/organizations/test-org/projects/?cursor=0:200:0>; rel="next"; results="false"; cursor="0:200:0"`)
			if err := json.NewEncoder(w).Encode([]projectInfo{
				{ID: "2", Slug: "proj-2", Name: "Project 2", Team: teamInfo{ID: "t1", Slug: "team", Name: "Team"}},
			}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer server.Close()

	client := newSentryClientImpl(server.URL, "token", server.Client())

	projects, err := client.GetAllProjects(ctx, "test-org")
	if err != nil {
		t.Fatalf("GetAllProjects returned error: %v", err)
	}

	if got, want := len(projects), 2; got != want {
		t.Fatalf("expected %d projects, got %d", want, got)
	}

	if projects[0].Slug != "proj-1" || projects[1].Slug != "proj-2" {
		t.Fatalf("unexpected project order %+v", projects)
	}

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

func TestGetProjectKeysPagination(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	var callCount int32

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)

		if r.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/api/0/projects/test-org/test-project/keys/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		cursor := r.URL.Query().Get("cursor")
		switch cursor {
		case "":
			w.Header().Set("Link", `<`+server.URL+`/api/0/projects/test-org/test-project/keys/?cursor=0:100:0>; rel="next"; results="true"; cursor="0:100:0"`)
			if err := json.NewEncoder(w).Encode([]projectKey{
				{ID: "1", Name: "Key 1", Public: "public1", Secret: "secret1", ProjectID: 1, IsActive: true, DSN: dsnField{Public: "https://public1@example/1"}},
			}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		case "0:100:0":
			w.Header().Set("Link", `<`+server.URL+`/api/0/projects/test-org/test-project/keys/?cursor=0:200:0>; rel="next"; results="false"; cursor="0:200:0"`)
			if err := json.NewEncoder(w).Encode([]projectKey{
				{ID: "2", Name: "Key 2", Public: "public2", Secret: "secret2", ProjectID: 1, IsActive: false, DSN: dsnField{Public: "https://public2@example/1"}},
			}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		default:
			t.Fatalf("unexpected cursor %q", cursor)
		}
	}))
	defer server.Close()

	client := newSentryClientImpl(server.URL, "token", server.Client())

	keys, err := client.GetProjectKeys(ctx, "test-org", "test-project")
	if err != nil {
		t.Fatalf("GetProjectKeys returned error: %v", err)
	}

	if got, want := len(keys), 2; got != want {
		t.Fatalf("expected %d keys, got %d", want, got)
	}

	if keys[0].ID != "1" || keys[1].ID != "2" {
		t.Fatalf("unexpected key order %+v", keys)
	}

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBuildPolicyBodyDefaults(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{}}
	body := m.buildPolicyBody(otelV1SpanIndexAlias)

	require.Len(t, body.Policy.States, 1)
	require.Len(t, body.Policy.States[0].Actions, 1)
	rollover := body.Policy.States[0].Actions[0].Rollover
	require.NotNil(t, rollover)
	assert.Equal(t, defaultRolloverMinSize, rollover.MinSize)
	assert.Equal(t, defaultRolloverMinIndexAge, rollover.MinIndexAge)

	require.Len(t, body.Policy.Template, 1)
	assert.Equal(t, []string{otelV1SpanIndexAlias + "-*"}, body.Policy.Template[0].IndexPatterns)
	assert.Equal(t, defaultRolloverPriority, body.Policy.Template[0].Priority)
}

func TestBuildPolicyBodyCustomRollover(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{RolloverMinSize: "10gb", RolloverMinIndexAge: "1h"}}
	body := m.buildPolicyBody(otelV1LogsIndexAlias)

	rollover := body.Policy.States[0].Actions[0].Rollover
	require.NotNil(t, rollover)
	assert.Equal(t, "10gb", rollover.MinSize)
	assert.Equal(t, "1h", rollover.MinIndexAge)
}

func TestBuildPolicyBodyCustomPriority(t *testing.T) {
	m := &ismManager{cfg: ISMConfig{RolloverPriority: 200}}
	body := m.buildPolicyBody(otelV1SpanIndexAlias)
	require.Len(t, body.Policy.Template, 1)
	assert.Equal(t, 200, body.Policy.Template[0].Priority)
}

func TestLoadPolicyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	content := `{"policy":{"description":"custom","default_state":"hot","states":[{"name":"hot","actions":[{"rollover":{"min_size":"5gb"}}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	body, err := loadPolicyFile(path)
	require.NoError(t, err)
	assert.Equal(t, "custom", body.Policy.Description)
	assert.Equal(t, "hot", body.Policy.DefaultState)
	require.Len(t, body.Policy.States, 1)
	assert.Equal(t, "5gb", body.Policy.States[0].Actions[0].Rollover.MinSize)
}

func TestLoadPolicyFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json}"), 0o600))

	_, err := loadPolicyFile(path)
	assert.ErrorContains(t, err, "parsing ISM policy file")
}

// newISMManager pre-loads the custom policy file eagerly and fails fast on a bad path.
func TestNewISMManagerBadPolicyFile(t *testing.T) {
	_, err := newISMManager("http://localhost:9200", nil, nil, ISMConfig{PolicyFile: "/does/not/exist.json"}, zap.NewNop())
	assert.ErrorContains(t, err, "reading ISM policy file")
}

// A valid custom policy file is used verbatim by buildPolicyBody, overriding the built-in policy.
func TestNewISMManagerCustomPolicyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	content := `{"policy":{"description":"custom","default_state":"hot","states":[{"name":"hot","actions":[{"rollover":{"min_size":"5gb"}}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	m, err := newISMManager("http://localhost:9200", nil, nil, ISMConfig{PolicyFile: path}, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, m.customPolicy)
	assert.Equal(t, "custom", m.buildPolicyBody(otelV1SpanIndexAlias).Policy.Description)
}

func TestEnsureInitialIndex(t *testing.T) {
	tests := []struct {
		name         string
		existsStatus int
		wantCreate   bool
	}{
		{name: "alias missing creates initial index", existsStatus: http.StatusNotFound, wantCreate: true},
		{name: "alias exists skips creation", existsStatus: http.StatusOK, wantCreate: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var createPath, createBody string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					w.WriteHeader(tt.existsStatus)
				case http.MethodPut:
					body, _ := io.ReadAll(r.Body)
					createPath, createBody = r.URL.Path, string(body)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"acknowledged":true,"shards_acknowledged":true}`))
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer ts.Close()

			client, err := opensearchapi.NewClient(opensearchapi.Config{Client: opensearch.Config{Addresses: []string{ts.URL}}})
			require.NoError(t, err)
			m := &ismManager{client: client, logger: zap.NewNop()}

			m.ensureInitialIndex(t.Context(), "otel-v1-apm-span")

			if tt.wantCreate {
				assert.Equal(t, "/otel-v1-apm-span-000001", createPath)
				assert.JSONEq(t, `{"aliases":{"otel-v1-apm-span":{"is_write_index":true}}}`, createBody)
			} else {
				assert.Empty(t, createPath)
			}
		})
	}
}

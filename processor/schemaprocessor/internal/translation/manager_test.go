// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
)

//go:embed testdata/schema.yaml
var exampleTranslation []byte

func TranslationHandler(t *testing.T) http.Handler {
	assert.NotEmpty(t, exampleTranslation, "SchemaContent MUST not be empty")
	return http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
		_, err := wr.Write(exampleTranslation)
		assert.NoError(t, err, "Must not have issues writing schema content")
	})
}

func TestRequestTranslation(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(TranslationHandler(t))
	t.Cleanup(s.Close)

	schemaURL := fmt.Sprintf("%s/1.1.0", s.URL)

	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	m, err := NewManager(
		[]string{schemaURL},
		zaptest.NewLogger(t),
		5*time.Minute, 5,
		telemetryBuilder,
		NewHTTPProvider(s.Client()),
	)
	require.NoError(t, err, "Must not error when created manager")

	tr, err := m.RequestTranslation(t.Context(), "/not/a/valid/schema/URL")
	assert.Error(t, err, "Must error when requesting an invalid schema URL")
	assert.Nil(t, tr, "Must not return a translation")

	tn, err := m.RequestTranslation(t.Context(), schemaURL)
	require.NoError(t, err, "Must not error when requesting a valid schema URL")
	require.NotNil(t, tn, "Must return a translation")

	assert.True(t, tn.SupportedVersion(&Version{1, 0, 0}), "Must have the version listed as supported")
	trs, ok := tn.(*translator)
	require.True(t, ok, "Can cast to the concrete type")

	count := 0
	prevRev := &Version{1, 0, 0}
	it, status := trs.iterator(prevRev)
	assert.Equal(t, Update, status, "Must return a status of update")
	for currRev, more := it(); more; currRev, more = it() {
		assert.True(t, prevRev.LessThan(currRev.Version()))
		prevRev = currRev.Version()
		count++
	}
}

// TestRequestTranslationUpgrade verifies that the upgrade path fetches the target schema
// URL (not the incoming signal's URL). Real OTel schema files only contain history up to
// their own version, so an incoming 1.0.0 schema file has no knowledge of changes made
// in 1.1.0–1.9.0. The processor must fetch the target schema file to get the full history.
func TestRequestTranslationUpgrade(t *testing.T) {
	t.Parallel()

	// fullSchema contains history for 1.0.0–1.9.0, including a rename in 1.8.0.
	// stubSchema simulates what a real 1.0.0 schema file looks like: it only knows
	// about itself and has no forward history.
	fullSchema := LoadTranslationVersion(t, TranslationVersion190)
	stubSchema := `file_format: 1.0.0
schema_url: https://example.com/1.0.0
versions:
  1.0.0:
`

	// URL-aware server: serve the full schema only at the target URL.
	// All other URLs (simulating older signal schema files) return the stub.
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var content string
		if strings.HasSuffix(r.URL.Path, TranslationVersion190) {
			content = fullSchema
		} else {
			content = stubSchema
		}
		_, err := w.Write([]byte(content))
		assert.NoError(t, err)
	}))
	t.Cleanup(s.Close)

	targetURL := fmt.Sprintf("%s/%s", s.URL, TranslationVersion190)
	signalURL := fmt.Sprintf("%s/1.0.0", s.URL)

	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	m, err := NewManager(
		[]string{targetURL},
		zaptest.NewLogger(t),
		5*time.Minute, 5,
		telemetryBuilder,
		NewHTTPProvider(s.Client()),
	)
	require.NoError(t, err)

	tr, err := m.RequestTranslation(t.Context(), signalURL)
	require.NoError(t, err, "Must not error on upgrade path")
	require.NotNil(t, tr)

	// The translator must know about 1.0.0 (loaded from the target schema file).
	assert.True(t, tr.SupportedVersion(&Version{1, 0, 0}), "Must support the incoming signal version")

	// Apply the upgrade: 1.8.0 in the target schema renames db.cassandra.keyspace → db.name.
	scopeSpans := ptrace.NewScopeSpans()
	scopeSpans.SetSchemaUrl(signalURL)
	span := scopeSpans.Spans().AppendEmpty()
	span.Attributes().PutStr("db.cassandra.keyspace", "my_keyspace")

	require.NoError(t, tr.ApplyScopeSpanChanges(scopeSpans, signalURL))

	val, ok := span.Attributes().Get("db.name")
	require.True(t, ok, "db.name must exist after upgrade — rename was not applied")
	assert.Equal(t, "my_keyspace", val.Str())

	_, oldExists := span.Attributes().Get("db.cassandra.keyspace")
	assert.False(t, oldExists, "old attribute must be removed after upgrade")
}

type errorProvider struct{}

func (*errorProvider) Retrieve(_ context.Context, _ string) (string, error) {
	return "", errors.New("error")
}

func TestManagerError(t *testing.T) {
	t.Parallel()

	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	m, err := NewManager(
		[]string{"http://localhost/1.1.0"},
		zaptest.NewLogger(t),
		5*time.Minute, 5,
		telemetryBuilder,
		&errorProvider{},
	)
	require.NoError(t, err, "Must not error when created manager")

	tr, err := m.RequestTranslation(t.Context(), "http://localhost/1.1.0")
	assert.Error(t, err, "Must error when provider errors")
	assert.Nil(t, tr, "Must not return a translation")
}

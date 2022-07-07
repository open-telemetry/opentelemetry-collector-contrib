// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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

	var (
		schemaURL = fmt.Sprintf("%s/1.1.0", s.URL)
	)

	m, err := NewManager(
		[]string{schemaURL},
		zaptest.NewLogger(t),
	)
	require.NoError(t, err, "Must not error when created manager")
	require.NoError(t, m.SetProviders(NewHTTPProvider(s.Client())), "Must have no issues trying to set providers")

	nop, ok := m.RequestTranslation(context.Background(), "/not/a/valid/schema/URL").(nopTranslation)
	require.True(t, ok, "Must return a NoopTranslation if no valid schema URL is provided")
	require.NotNil(t, nop, "Must have a valid translation")

	tn, ok := m.RequestTranslation(context.Background(), schemaURL).(*translator)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, tn, "Must have a valid translation")

	assert.True(t, tn.SupportedVersion(&Version{1, 0, 0}), "Must have the version listed as supported")

	count := 0
	prevRev := &Version{1, 0, 0}
	it, status := tn.iterator(context.Background(), prevRev)
	assert.Equal(t, Update, status, "Must return a status of update")
	for currRev, more := it(); more; currRev, more = it() {
		assert.True(t, prevRev.LessThan(currRev.Version()))
		prevRev = currRev.Version()
		count++
	}

	tn, ok = m.RequestTranslation(context.Background(), schemaURL).(*translator)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, tn, "Must have a valid translation")

}

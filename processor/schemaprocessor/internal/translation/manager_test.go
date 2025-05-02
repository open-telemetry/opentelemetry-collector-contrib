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

	schemaURL := fmt.Sprintf("%s/1.1.0", s.URL)

	m, err := NewManager(
		[]string{schemaURL},
		zaptest.NewLogger(t),
		NewHTTPProvider(s.Client()),
	)
	require.NoError(t, err, "Must not error when created manager")

	tr, err := m.RequestTranslation(context.Background(), "/not/a/valid/schema/URL")
	assert.Error(t, err, "Must error when requesting an invalid schema URL")
	assert.Nil(t, tr, "Must not return a translation")

	tn, err := m.RequestTranslation(context.Background(), schemaURL)
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

type errorProvider struct{}

func (p *errorProvider) Retrieve(_ context.Context, _ string) (string, error) {
	return "", errors.New("error")
}

func TestManagerError(t *testing.T) {
	t.Parallel()

	m, err := NewManager(
		[]string{"http://localhost/1.1.0"},
		zaptest.NewLogger(t),
		&errorProvider{},
	)
	require.NoError(t, err, "Must not error when created manager")

	tr, err := m.RequestTranslation(context.Background(), "http://localhost/1.1.0")
	assert.Error(t, err, "Must error when provider errors")
	assert.Nil(t, tr, "Must not return a translation")
}

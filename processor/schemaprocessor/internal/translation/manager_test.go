// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

//go:embed testdata/schema.yml
var exampleTranslation []byte

func TranslationHandler(t *testing.T) http.Handler {
	assert.NotEmpty(t, exampleTranslation, "SchemaContent MUST not be empty")
	return http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
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
	require.NotNil(t, m, "Must have a valid client")

	var wg sync.WaitGroup
	ctx, done := context.WithCancel(context.Background())

	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()

		assert.NoError(t, m.Run(ctx, NewHTTPProvider(s.Client())), "Must not error when shutdown correctly")
	}(ctx)

	nop, ok := m.RequestTranslation(context.Background(), "/not/a/valid/schema/URL").(nopTranslation)
	require.True(t, ok, "Must return a NoopTranslation if no valid schema URL is provided")
	require.NotNil(t, nop, "Must have a valid translation")

	tn, ok := m.RequestTranslation(ctx, schemaURL).(*translator)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, tn, "Must have a valid translation")

	assert.True(t, tn.SupportedVersion(&Version{1, 0, 0}), "Must have the version listed as supported")

	count := 0
	ver := &Version{1, 0, 0}
	it, status := tn.iterator(ctx, ver)
	assert.Equal(t, Update, status, "Must return a status of update")
	for rev, more := it(); more; rev, more = it() {
		switch count {
		case 0:
			assert.True(t, ver.Equal(rev.Version()))
		default:
			assert.True(t, ver.LessThan(rev.Version()))
		}
		ver = rev.Version()
		count++
	}

	tn, ok = m.RequestTranslation(ctx, schemaURL).(*translator)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, tn, "Must have a valid translation")

	done()
	wg.Wait()
}

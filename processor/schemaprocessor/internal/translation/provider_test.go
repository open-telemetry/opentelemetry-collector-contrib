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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidHTTPProviderTests(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/1.7.0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		io.Copy(w, LoadTranslationVersion(t, "complex_changeset.yml"))
	}))
	t.Cleanup(s.Close)

	tests := []struct {
		scenario string
		url      string
	}{

		{
			scenario: "A failed request happens",
			url:      fmt.Sprint(s.URL, "/not/a/valid/path/1.7.0"),
		},
		{
			scenario: "invalid url",
			url:      "unix:///localhost",
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			p := NewHTTPProvider(s.Client())
			content, err := p.Lookup(context.Background(), tc.url)
			assert.Nil(t, content, "Expected to be nil")
			assert.Error(t, err, "Must have errored processing request")
		})
	}
}

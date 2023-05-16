// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sanitize

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	u := url.URL{
		Scheme: "https",
		Host:   "opentelemetry.io",
		Path:   "/path?q=go",
	}
	assert.Equal(t, "https://opentelemetry.io/path%3Fq=go", URL(&u))

	u = url.URL{
		Scheme: "https\r\n",
		Host:   "opentelemetry.io\r\n", // net/url String() escapes control characters for Host
		Path:   "/\r\npath?q=go\r\n",   // net/url String() escapes control characters for Path
	}
	assert.Equal(t, "https://opentelemetry.io%0D%0A/%0D%0Apath%3Fq=go%0D%0A", URL(&u))
}

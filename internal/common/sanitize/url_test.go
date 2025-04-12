// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

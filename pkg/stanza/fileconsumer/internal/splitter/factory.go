// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/splitter"

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

// Func builds a bufio.SplitFunc based on the configuration
// First apply the base splitFunc.
// If a token is emitted for any reason, we must then apply trim rules.
// We must trim to max length _before_ trimming whitespace because otherwise we
// cannot properly keep track of the number of bytes to advance.
// For instance, if we have advance: 5, token: []byte(" foo "):
//
//	Trimming whitespace first would result in advance: 5, token: []byte("foo")
//	Then if we trim to max length of 2, we don't know whether or not to reduce advance.
func Func(splitFunc bufio.SplitFunc, maxLength int, trimFunc trim.Func) bufio.SplitFunc {
	return trim.WithFunc(trim.ToLength(splitFunc, maxLength), trimFunc)
}

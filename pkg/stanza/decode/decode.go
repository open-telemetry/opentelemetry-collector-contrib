// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package decode // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"

import (
	"errors"
	"fmt"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

// Deprecated: [v0.120.0] Use directly the encoding.Decoder().
type Decoder struct {
	decoder      *encoding.Decoder
	decodeBuffer []byte
}

// Deprecated: [v0.120.0] Use directly the encoding.Decoder().
func New(enc encoding.Encoding) *Decoder {
	return &Decoder{
		decoder:      enc.NewDecoder(),
		decodeBuffer: make([]byte, 1<<12),
	}
}

// Decode converts the bytes in msgBuf to UTF-8 from the configured encoding.
func (d *Decoder) Decode(msgBuf []byte) ([]byte, error) {
	for {
		d.decoder.Reset()
		nDst, _, err := d.decoder.Transform(d.decodeBuffer, msgBuf, true)
		if err == nil {
			return d.decodeBuffer[:nDst], nil
		}
		if errors.Is(err, transform.ErrShortDst) {
			d.decodeBuffer = make([]byte, len(d.decodeBuffer)*2)
			continue
		}
		return nil, fmt.Errorf("transform encoding: %w", err)
	}
}

// Deprecated: [v0.120.0] no public replacement.
func LookupEncoding(enc string) (encoding.Encoding, error) {
	return textutils.LookupEncoding(enc)
}

// Deprecated: [v0.120.0] no public replacement.
func IsNop(enc string) bool {
	return textutils.IsNop(enc)
}

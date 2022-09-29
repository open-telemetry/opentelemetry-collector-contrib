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

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// NewBasicConfig creates a new Encoding config
func NewEncodingConfig() EncodingConfig {
	return EncodingConfig{
		Encoding: "utf-8",
	}
}

// EncodingConfig is the configuration of a Encoding helper
type EncodingConfig struct {
	Encoding string `mapstructure:"encoding,omitempty"`
}

// Build will build an Encoding operator.
func (c EncodingConfig) Build() (Encoding, error) {
	enc, err := lookupEncoding(c.Encoding)
	if err != nil {
		return Encoding{}, err
	}

	return Encoding{
		Encoding: enc,
	}, nil
}

type Encoding struct {
	Encoding encoding.Encoding
}

// decode converts the bytes in msgBuf to utf-8 from the configured encoding
func (e *Encoding) Decode(msgBuf []byte) ([]byte, error) {
	decodeBuffer := make([]byte, 1<<12)
	decoder := e.Encoding.NewDecoder()

	for {
		decoder.Reset()
		nDst, _, err := decoder.Transform(decodeBuffer, msgBuf, true)
		if err == nil {
			return decodeBuffer[:nDst], nil
		}
		if errors.Is(err, transform.ErrShortDst) {
			decodeBuffer = make([]byte, len(decodeBuffer)*2)
			continue
		}
		return nil, fmt.Errorf("transform encoding: %w", err)
	}
}

var encodingOverrides = map[string]encoding.Encoding{
	"utf-16":   unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf16":    unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf-8":    unicode.UTF8,
	"utf8":     unicode.UTF8,
	"ascii":    unicode.UTF8,
	"us-ascii": unicode.UTF8,
	"nop":      encoding.Nop,
	"":         unicode.UTF8,
}

func lookupEncoding(enc string) (encoding.Encoding, error) {
	if e, ok := encodingOverrides[strings.ToLower(enc)]; ok {
		return e, nil
	}
	e, err := ianaindex.IANA.Encoding(enc)
	if err != nil {
		return nil, fmt.Errorf("unsupported encoding '%s'", enc)
	}
	if e == nil {
		return nil, fmt.Errorf("no charmap defined for encoding '%s'", enc)
	}
	return e, nil
}

func IsNop(enc string) bool {
	e, err := lookupEncoding(enc)
	if err != nil {
		return false
	}
	return e == encoding.Nop
}

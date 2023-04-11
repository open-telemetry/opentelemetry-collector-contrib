// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package textutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestUTF8Encoding(t *testing.T) {
	tests := []struct {
		name         string
		encoding     encoding.Encoding
		encodingName string
	}{
		{
			name:         "UTF8 encoding",
			encoding:     unicode.UTF8,
			encodingName: "utf8",
		},
		{
			name:         "GBK encoding",
			encoding:     simplifiedchinese.GBK,
			encodingName: "gbk",
		},
		{
			name:         "SHIFT_JIS encoding",
			encoding:     japanese.ShiftJIS,
			encodingName: "shift_jis",
		},
		{
			name:         "EUC-KR encoding",
			encoding:     korean.EUCKR,
			encodingName: "euc-kr",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encCfg := NewEncodingConfig()
			encCfg.Encoding = test.encodingName
			enc, err := encCfg.Build()
			assert.NoError(t, err)
			assert.Equal(t, test.encoding, enc.Encoding)
		})
	}
}

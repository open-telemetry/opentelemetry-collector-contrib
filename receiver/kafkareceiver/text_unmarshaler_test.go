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

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTextUnmarshaler(t *testing.T) {
	t.Parallel()
	um := newTextLogsUnmarshaler()
	assert.Equal(t, "text", um.Encoding())
}

func TestTextUnmarshalerWithEnc(t *testing.T) {
	t.Parallel()
	um := newTextLogsUnmarshaler()
	um2 := um
	assert.EqualValues(t, um, um2)

	um, err := um.WithEnc("utf8")
	require.NoError(t, err)
	um2, err = um2.WithEnc("gbk")
	require.NoError(t, err)
	assert.NotEqualValues(t, um, um2)

	um2, err = um2.WithEnc("utf8")
	require.NoError(t, err)
	assert.EqualValues(t, um, um2)
}

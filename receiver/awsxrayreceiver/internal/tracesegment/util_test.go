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
package tracesegment

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	recvErr "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"
)

func TestSplitHeaderBodyWithSeparatorExists(t *testing.T) {
	buf := []byte(`{"format":"json", "version":1}` + "\nBody")

	header, body, err := SplitHeaderBody(buf)
	assert.NoError(t, err, "should split correctly")

	assert.Equal(t, &Header{
		Format:  "json",
		Version: 1,
	}, header, "actual header is different from the expected")
	assert.Equal(t, "Body", string(body), "actual body is different from the expected")
}

func TestSplitHeaderBodyWithSeparatorDoesNotExist(t *testing.T) {
	buf := []byte(`{"format":"json", "version":1}`)

	_, _, err := SplitHeaderBody(buf)

	var errRecv *recvErr.ErrRecoverable
	assert.True(t, errors.As(err, &errRecv), "should return recoverable error")
	assert.EqualError(t, err,
		fmt.Sprintf("unable to split incoming data as header and segment, incoming bytes: %v", buf),
		"expected error messages")
}

func TestSplitHeaderBodyNilBuf(t *testing.T) {
	_, _, err := SplitHeaderBody(nil)

	var errRecv *recvErr.ErrRecoverable
	assert.True(t, errors.As(err, &errRecv), "should return recoverable error")
	assert.EqualError(t, err, "buffer to split is nil",
		"expected error messages")
}

func TestSplitHeaderBodyNonJsonHeader(t *testing.T) {
	buf := []byte(`nonJson` + "\nBody")

	_, _, err := SplitHeaderBody(buf)

	var errRecv *recvErr.ErrRecoverable
	assert.True(t, errors.As(err, &errRecv), "should return recoverable error")
	assert.Contains(t, err.Error(), "invalid character 'o'")
}

func TestSplitHeaderBodyEmptyBody(t *testing.T) {
	buf := []byte(`{"format":"json", "version":1}` + "\n")

	header, body, err := SplitHeaderBody(buf)
	assert.NoError(t, err, "should split correctly")

	assert.Equal(t, &Header{
		Format:  "json",
		Version: 1,
	}, header, "actual header is different from the expected")
	assert.Len(t, body, 0, "body should be empty")
}

func TestSplitHeaderBodyInvalidJsonHeader(t *testing.T) {
	buf := []byte(`{"format":"json", "version":20}` + "\n")

	_, _, err := SplitHeaderBody(buf)
	assert.Error(t, err, "should fail because version is invalid")

	var errRecv *recvErr.ErrRecoverable
	assert.True(t, errors.As(err, &errRecv), "should return recoverable error")
	assert.Contains(t, err.Error(),
		fmt.Sprintf("invalid header %+v", Header{
			Format:  "json",
			Version: 20,
		}),
	)
}

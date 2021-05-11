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

package dotnet

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestEventHeader(t *testing.T) {
	h := eventHeader{}
	rw := network.NewDefaultFakeRW("", "", "")
	mr := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseEventHeader(mr, &h)
	require.NoError(t, err)
}

func TestEventHeaderErrors(t *testing.T) {
	testEventHeaderError(t, 0, 0)
	testEventHeaderError(t, headerFlagMetadataID, 1)
	testEventHeaderError(t, headerFlagCaptureThreadAndSequence, 1)
	testEventHeaderError(t, headerFlagCaptureThreadAndSequence, 2)
	testEventHeaderError(t, headerFlagCaptureThreadAndSequence, 3)
	testEventHeaderError(t, headerFlagThreadID, 1)
	testEventHeaderError(t, headerFlagStackID, 1)
	testEventHeaderError(t, 0, 1)
	testEventHeaderError(t, headerFlagActivityID, 2)
	testEventHeaderError(t, headerFlagRelatedActivityID, 2)
	testEventHeaderError(t, headerFlagDataLength, 2)
}

func testEventHeaderError(t *testing.T, flags headerFlags, errPos int) {
	pr := &network.FakeRW{
		ReadErrIdx: errPos,
		Responses: map[int][]byte{
			0: {byte(flags)},
		},
	}
	mr := network.NewMultiReader(pr, &network.NopBlobWriter{})
	err := parseEventHeader(mr, &eventHeader{})
	require.Error(t, err)
}

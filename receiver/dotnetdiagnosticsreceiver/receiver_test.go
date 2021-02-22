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

package dotnetdiagnosticsreceiver

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestReceiver(t *testing.T) {
	ctx := context.Background()
	rw := &fakeRW{writeErrOn: -1}
	r, err := NewReceiver(
		ctx,
		consumertest.NewMetricsNop(),
		func() (io.ReadWriter, error) {
			return rw, nil
		},
		nil,
		1,
		zap.NewNop(),
	)
	require.NoError(t, err)
	err = r.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	err = r.Shutdown(ctx)
	require.NoError(t, err)
	const magic = "DOTNET_IPC_V1"
	magicBytes := rw.writeBuf[:len(magic)]
	require.Equal(t, magic, string(magicBytes))
}

func TestReceiver_ConnectError(t *testing.T) {
	connect := func() (io.ReadWriter, error) {
		return nil, errors.New("foo")
	}
	testErrOnReceiverStart(t, connect, "foo")
}

func TestReceiver_WriteRequestError(t *testing.T) {
	connect := func() (io.ReadWriter, error) {
		return &fakeRW{}, nil
	}
	testErrOnReceiverStart(t, connect, "")
}

func testErrOnReceiverStart(t *testing.T, connect func() (io.ReadWriter, error), errStr string) {
	ctx := context.Background()
	r, err := NewReceiver(
		ctx,
		consumertest.NewMetricsNop(),
		connect,
		nil,
		1,
		zap.NewNop(),
	)
	require.NoError(t, err)
	err = r.Start(ctx, componenttest.NewNopHost())
	require.Error(t, err, errStr)
}

type fakeRW struct {
	writeErrOn int
	writeBuf   []byte
	writeCount int
}

var _ io.ReadWriter = (*fakeRW)(nil)

func (rw *fakeRW) Write(p []byte) (n int, err error) {
	defer func() {
		rw.writeCount++
	}()
	if rw.writeCount == rw.writeErrOn {
		return 0, errors.New("")
	}
	rw.writeBuf = append(rw.writeBuf, p...)
	return len(p), nil
}

func (rw *fakeRW) Read(p []byte) (n int, err error) {
	return len(p), nil
}

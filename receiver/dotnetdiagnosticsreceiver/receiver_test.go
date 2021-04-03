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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestReceiverBlobData(t *testing.T) {
	data, err := network.ReadBlobData("testdata", 16)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	ctx := context.Background()
	r, err := NewReceiver(
		ctx,
		consumertest.NewNop(),
		func() (io.ReadWriter, error) {
			return rw, nil
		},
		nil,
		1,
		zap.NewNop(),
		&network.NopBlobWriter{},
	)
	require.NoError(t, err)
	err = r.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	<-rw.Gate()
	err = r.Shutdown(ctx)
	require.NoError(t, err)
}

func TestReceiverBlobData_ParsingError(t *testing.T) {
	data, err := network.ReadBlobData("testdata", 16)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	ctx := context.Background()
	// cause an error in the goroutine (ParseAll)
	rw.ErrOnRead(9)
	obs, logs := observer.New(zap.WarnLevel)
	r, err := NewReceiver(
		ctx,
		consumertest.NewNop(),
		func() (io.ReadWriter, error) {
			return rw, nil
		},
		nil,
		1,
		zap.New(obs),
		&network.NopBlobWriter{},
	)
	require.NoError(t, err)
	err = r.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return logs.Len() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestReceiver_ConnectError(t *testing.T) {
	connect := func() (io.ReadWriter, error) {
		return nil, errors.New("foo")
	}
	testErrOnReceiverStart(t, connect, "foo")
}

func TestReceiver_WriteRequestError(t *testing.T) {
	connect := func() (io.ReadWriter, error) {
		return &network.FakeRW{}, nil
	}
	testErrOnReceiverStart(t, connect, "")
}

func testErrOnReceiverStart(t *testing.T, connect func() (io.ReadWriter, error), errStr string) {
	ctx := context.Background()
	r, err := NewReceiver(
		ctx,
		consumertest.NewNop(),
		connect,
		nil,
		1,
		zap.NewNop(),
		&network.NopBlobWriter{},
	)
	require.NoError(t, err)
	err = r.Start(ctx, componenttest.NewNopHost())
	require.Error(t, err, errStr)
}

func TestRecevier_ReadErr(t *testing.T) {
	err := testReceiverReadErr(3)
	assert.Error(t, err)
	err = testReceiverReadErr(6)
	assert.Error(t, err)
}

func testReceiverReadErr(i int) error {
	rw := fakeRW()
	rw.ReadErrIdx = i
	ctx := context.Background()
	r, _ := NewReceiver(
		ctx,
		consumertest.NewNop(),
		func() (io.ReadWriter, error) {
			return rw, nil
		},
		nil,
		1,
		zap.NewNop(),
		&network.NopBlobWriter{},
	)
	return r.Start(ctx, componenttest.NewNopHost())
}

func fakeRW() *network.FakeRW {
	rw := network.NewDefaultFakeRW(
		"DOTNET_IPC_V1\000",
		"Nettrace",
		"!FastSerialization.1",
	)
	return rw
}

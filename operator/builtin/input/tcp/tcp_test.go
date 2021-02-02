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

package tcp

import (
	"net"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func tcpInputTest(input []byte, expected []string) func(t *testing.T) {
	return func(t *testing.T) {
		cfg := NewTCPInputConfig("test_id")
		cfg.ListenAddress = ":0"

		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		op := ops[0]

		mockOutput := testutil.Operator{}
		tcpInput := op.(*TCPInput)
		tcpInput.InputOperator.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = tcpInput.Start()
		require.NoError(t, err)
		defer tcpInput.Stop()

		conn, err := net.Dial("tcp", tcpInput.listener.Addr().String())
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write(input)
		require.NoError(t, err)

		for _, expectedMessage := range expected {
			select {
			case entry := <-entryChan:
				require.Equal(t, expectedMessage, entry.Record)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for message to be written")
			}
		}

		select {
		case entry := <-entryChan:
			require.FailNow(t, "Unexpected entry: %s", entry)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func TestTcpInput(t *testing.T) {
	t.Run("Simple", tcpInputTest([]byte("message\n"), []string{"message"}))
	t.Run("CarriageReturn", tcpInputTest([]byte("message\r\n"), []string{"message"}))
}

func BenchmarkTcpInput(b *testing.B) {
	cfg := NewTCPInputConfig("test_id")
	cfg.ListenAddress = ":0"

	ops, err := cfg.Build(testutil.NewBuildContext(b))
	require.NoError(b, err)
	op := ops[0]

	fakeOutput := testutil.NewFakeOutput(b)
	tcpInput := op.(*TCPInput)
	tcpInput.InputOperator.OutputOperators = []operator.Operator{fakeOutput}

	err = tcpInput.Start()
	require.NoError(b, err)

	done := make(chan struct{})
	go func() {
		conn, err := net.Dial("tcp", tcpInput.listener.Addr().String())
		require.NoError(b, err)
		defer tcpInput.Stop()
		defer conn.Close()
		message := []byte("message\n")
		for {
			select {
			case <-done:
				return
			default:
				_, err := conn.Write(message)
				require.NoError(b, err)
			}
		}
	}()

	for i := 0; i < b.N; i++ {
		<-fakeOutput.Received
	}

	defer close(done)
}

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

package udp

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func udpInputTest(input []byte, expected []string) func(t *testing.T) {
	return func(t *testing.T) {
		cfg := NewUDPInputConfig("test_input")
		cfg.ListenAddress = ":0"

		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		op := ops[0]

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*UDPInput)
		require.True(t, ok)

		udpInput.InputOperator.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start()
		require.NoError(t, err)
		defer udpInput.Stop()

		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write(input)
		require.NoError(t, err)

		for _, expectedRecord := range expected {
			select {
			case entry := <-entryChan:
				require.Equal(t, expectedRecord, entry.Record)
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

func TestUDPInput(t *testing.T) {
	t.Run("Simple", udpInputTest([]byte("message1"), []string{"message1"}))
	t.Run("TrailingNewlines", udpInputTest([]byte("message1\n"), []string{"message1"}))
	t.Run("TrailingCRNewlines", udpInputTest([]byte("message1\r\n"), []string{"message1"}))
	t.Run("NewlineInMessage", udpInputTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}))
}

func BenchmarkUdpInput(b *testing.B) {
	cfg := NewUDPInputConfig("test_id")
	cfg.ListenAddress = ":0"

	ops, err := cfg.Build(testutil.NewBuildContext(b))
	require.NoError(b, err)
	op := ops[0]

	fakeOutput := testutil.NewFakeOutput(b)
	udpInput := op.(*UDPInput)
	udpInput.InputOperator.OutputOperators = []operator.Operator{fakeOutput}

	err = udpInput.Start()
	require.NoError(b, err)

	done := make(chan struct{})
	go func() {
		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		require.NoError(b, err)
		defer udpInput.Stop()
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

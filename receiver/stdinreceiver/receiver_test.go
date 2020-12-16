// Copyright 2020, OpenTelemetry Authors
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

package stdinreceiver

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestConsumeLine(t *testing.T) {
	sink := new(consumertest.LogsSink)
	config := createDefaultConfig()
	r := stdinReceiver{logsConsumer: sink, config: config.(*Config)}
	r.consumeLine("foo")
	lds := sink.AllLogs()
	assert.Equal(t, 1, len(lds))
	log := lds[0].ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().StringVal()
	assert.Equal(t, "foo", log)
}

func TestReceiveLinesTwoReceivers(t *testing.T) {
	read, write, err := os.Pipe()
	assert.NoError(t, err)
	stdin = read
	sink := new(consumertest.LogsSink)
	config := createDefaultConfig()
	r := stdinReceiver{logsConsumer: sink, config: config.(*Config)}
	sink2 := new(consumertest.LogsSink)
	config2 := createDefaultConfig()
	r2 := stdinReceiver{logsConsumer: sink2, config: config2.(*Config)}
	err = r.Start(context.Background(), nil)
	assert.NoError(t, err)
	err = r2.Start(context.Background(), nil)
	assert.NoError(t, err)
	write.WriteString("foo\nbar\nfoobar\n")
	write.WriteString("foo\r\nbar\nfoobar")
	time.Sleep(time.Second * 1)
	lds := sink.AllLogs()
	assert.Equal(t, 6, len(lds))
	lds2 := sink2.AllLogs()
	assert.Equal(t, 6, len(lds2))

	write.Close()

	err = r.Shutdown(context.Background())
	assert.NoError(t, err)
	err = r2.Shutdown(context.Background())
	assert.NoError(t, err)
}

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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestClientExpectError(t *testing.T) {
	client, err := newClient(icingaClientConfig{})
	if err != nil {
		t.Fail()
	}

	ctx := context.Background()
	parser := icingaParser{ctx: ctx, config: Config{}}
	err = client.Listen(ctx, parser, nil)
	require.EqualError(t, err, "nil next Consumer")
}

func TestClientListen(t *testing.T) {
	// mock server
	c := make(chan interface{})
	port, close := createMockServer(c)
	defer close()

	// create client
	client, _ := newClient(icingaClientConfig{
		Host:                   fmt.Sprintf("localhost:%d", port),
		DisableSslVerification: true,
		logger:                 zap.NewNop().Sugar(),
	})
	ctx := context.Background()
	parser := icingaParser{ctx: ctx, config: Config{}}
	parser.initialize()
	err := client.Listen(ctx, parser, consumertest.NewNop())
	if err != nil {
		fmt.Print(err)
		t.Fail()
	}

	// wait until request received
	<-c
}

func createMockServer(c chan interface{}) (port int, close func()) {
	svr := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "\n")
		c <- true
	}))
	svr.StartTLS()

	return svr.Listener.Addr().(*net.TCPAddr).Port, func() {
		svr.Close()
	}
}

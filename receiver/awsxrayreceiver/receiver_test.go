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

package awsxrayreceiver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

const (
	segmentHeader        = "{\"format\": \"json\", \"version\": 1}\n"
	defaultRegionEnvName = "AWS_DEFAULT_REGION"
	mockRegion           = "us-west-2"
)

func TestConsumerCantBeNil(t *testing.T) {
	addr, err := net.ResolveUDPAddr(udppoller.Transport, "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP(udppoller.Transport, addr)
	assert.NoError(t, err, "should be able to listen")
	defer sock.Close()
	address := sock.LocalAddr().String()

	_, err = newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  address,
				Transport: udppoller.Transport,
			},
		},
		nil,
		zap.NewNop(),
	)
	assert.True(t, errors.Is(err, componenterror.ErrNilNextConsumer), "consumer is nil should be detected")
}

func TestProxyCreationFailed(t *testing.T) {
	addr, err := findAvailableUDPAddress()
	assert.NoError(t, err, "there should be address available")

	sink := new(consumertest.TracesSink)
	_, err = newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: udppoller.Transport,
			},
			ProxyServer: &proxy.Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: "invalidEndpoint",
				},
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.Error(t, err, "receiver creation should fail due to failure to create TCP proxy")
}

func TestPollerCreationFailed(t *testing.T) {
	sink := new(consumertest.TracesSink)
	_, err := newReceiver(
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  "dontCare",
				Transport: "tcp",
			},
		},
		sink,
		zap.NewNop(),
	)
	assert.Error(t, err, "receiver creation should fail due to failure to create UCP poller")
}

// TODO: Update this test to assert on the format of traces
// once the transformation from X-Ray segments -> OTLP is done.
func TestSegmentsPassedToConsumer(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping test on darwin")
	}
	doneFn, err := obsreporttest.SetupRecordedMetricsTest()
	assert.NoError(t, err, "SetupRecordedMetricsTest should succeed")
	defer doneFn()

	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	receiverID := config.NewID("TestSegmentsPassedToConsumer")

	addr, rcvr, _ := createAndOptionallyStartReceiver(t, receiverID, nil, true)
	defer rcvr.Shutdown(context.Background())

	content, err := ioutil.ReadFile(path.Join("../../internal/aws/xray", "testdata", "ddbSample.txt"))
	assert.NoError(t, err, "can not read raw segment")

	err = writePacket(t, addr, segmentHeader+string(content))
	assert.NoError(t, err, "can not write packet in the happy case")

	sink := rcvr.(*xrayReceiver).consumer.(*consumertest.TracesSink)

	assert.Eventuallyf(t, func() bool {
		got := sink.AllTraces()
		return len(got) == 1
	}, 10*time.Second, 5*time.Millisecond, "consumer should eventually get the X-Ray span")

	obsreporttest.CheckReceiverTraces(t, receiverID, udppoller.Transport, 18, 0)
}

func TestTranslatorErrorsOut(t *testing.T) {
	doneFn, err := obsreporttest.SetupRecordedMetricsTest()
	assert.NoError(t, err, "SetupRecordedMetricsTest should succeed")
	defer doneFn()

	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	receiverID := config.NewID("TestTranslatorErrorsOut")

	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, receiverID, nil, true)
	defer rcvr.Shutdown(context.Background())

	err = writePacket(t, addr, segmentHeader+"invalidSegment")
	assert.NoError(t, err, "can not write packet in the "+receiverID.String()+" case")

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		fmt.Println(logs)
		return len(logs) > 0 && strings.Contains(logs[len(logs)-1].Message,
			"X-Ray segment to OT traces conversion failed")
	}, 10*time.Second, 5*time.Millisecond, "poller should log warning because consumer errored out")

	obsreporttest.CheckReceiverTraces(t, receiverID, udppoller.Transport, 0, 1)
}

func TestSegmentsConsumerErrorsOut(t *testing.T) {
	doneFn, err := obsreporttest.SetupRecordedMetricsTest()
	assert.NoError(t, err, "SetupRecordedMetricsTest should succeed")
	defer doneFn()

	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	receiverID := config.NewID("TestSegmentsConsumerErrorsOut")

	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, receiverID,
		consumertest.NewErr(errors.New("can't consume traces")), true)
	defer rcvr.Shutdown(context.Background())

	content, err := ioutil.ReadFile(path.Join("../../internal/aws/xray", "testdata", "serverSample.txt"))
	assert.NoError(t, err, "can not read raw segment")

	err = writePacket(t, addr, segmentHeader+string(content))
	assert.NoError(t, err, "can not write packet")

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		return len(logs) > 0 && strings.Contains(logs[len(logs)-1].Message,
			"Trace consumer errored out")
	}, 10*time.Second, 5*time.Millisecond, "poller should log warning because consumer errored out")

	obsreporttest.CheckReceiverTraces(t, receiverID, udppoller.Transport, 0, 1)
}

func TestPollerCloseError(t *testing.T) {
	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, config.NewID("TestPollerCloseError"), nil, false)
	mPoller := &mockPoller{closeErr: errors.New("mockPollerCloseErr")}
	rcvr.(*xrayReceiver).poller = mPoller
	rcvr.(*xrayReceiver).server = &mockProxy{}
	err := rcvr.Shutdown(context.Background())
	assert.EqualError(t, err, mPoller.closeErr.Error(), "expected error")
}

func TestProxyCloseError(t *testing.T) {
	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, config.NewID("TestPollerCloseError"), nil, false)
	mProxy := &mockProxy{closeErr: errors.New("mockProxyCloseErr")}
	rcvr.(*xrayReceiver).poller = &mockPoller{}
	rcvr.(*xrayReceiver).server = mProxy
	err := rcvr.Shutdown(context.Background())
	assert.EqualError(t, err, mProxy.closeErr.Error(), "expected error")
}

func TestBothPollerAndProxyCloseError(t *testing.T) {
	env := stashEnv()
	defer restoreEnv(env)
	os.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, config.NewID("TestBothPollerAndProxyCloseError"), nil, false)
	mPoller := &mockPoller{closeErr: errors.New("mockPollerCloseErr")}
	mProxy := &mockProxy{closeErr: errors.New("mockProxyCloseErr")}
	rcvr.(*xrayReceiver).poller = mPoller
	rcvr.(*xrayReceiver).server = mProxy
	err := rcvr.Shutdown(context.Background())
	assert.EqualError(t, err,
		fmt.Sprintf("failed to close proxy: %s: failed to close poller: %s",
			mProxy.closeErr.Error(), mPoller.closeErr.Error()),
		"expected error")
}

type mockPoller struct {
	closeErr error
}

func (m *mockPoller) SegmentsChan() <-chan udppoller.RawSegment {
	return make(chan udppoller.RawSegment, 1)
}

func (m *mockPoller) Start(ctx context.Context) {}

func (m *mockPoller) Close() error {
	if m.closeErr != nil {
		return m.closeErr
	}
	return nil
}

type mockProxy struct {
	closeErr error
}

func (m *mockProxy) ListenAndServe() error {
	return errors.New("returning from ListenAndServe() always errors out")
}

func (m *mockProxy) Close() error {
	if m.closeErr != nil {
		return m.closeErr
	}
	return nil
}

func createAndOptionallyStartReceiver(
	t *testing.T,
	receiverID config.ComponentID,
	csu consumer.Traces,
	start bool) (string, component.TracesReceiver, *observer.ObservedLogs) {
	addr, err := findAvailableUDPAddress()
	assert.NoError(t, err, "there should be address available")
	tcpAddr := testutil.GetAvailableLocalAddress(t)

	var sink consumer.Traces
	if csu == nil {
		sink = new(consumertest.TracesSink)
	} else {
		sink = csu
	}

	logger, recorded := logSetup()
	rcvr, err := newReceiver(
		&Config{
			ReceiverSettings: config.NewReceiverSettings(receiverID),
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: udppoller.Transport,
			},
			ProxyServer: &proxy.Config{
				TCPAddr: confignet.TCPAddr{
					Endpoint: tcpAddr,
				},
			},
		},
		sink,
		logger,
	)
	assert.NoError(t, err, "receiver should be created")

	if start {
		err = rcvr.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err, "receiver should be started")
	}
	return addr, rcvr, recorded
}

// findAvailableUDPAddress finds an available local address+port and returns it.
// There might be race condition on the address returned by this function if
// there's some other code that grab the address before we can listen on it.
func findAvailableUDPAddress() (string, error) {
	addr, err := net.ResolveUDPAddr(udppoller.Transport, "localhost:0")
	if err != nil {
		return "", err
	}

	sock, err := net.ListenUDP(udppoller.Transport, addr)
	if err != nil {
		return "", err
	}
	defer sock.Close()
	return sock.LocalAddr().String(), nil
}

func writePacket(t *testing.T, addr, toWrite string) error {
	conn, err := net.Dial(udppoller.Transport, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := fmt.Fprint(conn, toWrite)
	if err != nil {
		return err
	}
	assert.Equal(t, len(toWrite), n, "unexpected number of bytes written")
	return nil
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.InfoLevel)
	return zap.New(core), recorded
}

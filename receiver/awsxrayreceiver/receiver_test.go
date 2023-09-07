// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
		receivertest.NewNopCreateSettings(),
	)
	assert.True(t, errors.Is(err, component.ErrNilNextConsumer), "consumer is nil should be detected")
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
		receivertest.NewNopCreateSettings(),
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
		receivertest.NewNopCreateSettings(),
	)
	assert.Error(t, err, "receiver creation should fail due to failure to create UCP poller")
}

// TODO: Update this test to assert on the format of traces
// once the transformation from X-Ray segments -> OTLP is done.
func TestSegmentsPassedToConsumer(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping test on darwin")
	}
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10596")

	receiverID := component.NewID("TestSegmentsPassedToConsumer")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	addr, rcvr, _ := createAndOptionallyStartReceiver(t, nil, true, tt.ToReceiverCreateSettings())
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	content, err := os.ReadFile(filepath.Join("../../internal/aws/xray", "testdata", "ddbSample.txt"))
	assert.NoError(t, err, "can not read raw segment")

	err = writePacket(t, addr, segmentHeader+string(content))
	assert.NoError(t, err, "can not write packet in the happy case")

	sink := rcvr.(*xrayReceiver).consumer.(*consumertest.TracesSink)

	assert.Eventuallyf(t, func() bool {
		got := sink.AllTraces()
		return len(got) == 1
	}, 10*time.Second, 5*time.Millisecond, "consumer should eventually get the X-Ray span")

	assert.NoError(t, tt.CheckReceiverTraces(udppoller.Transport, 18, 0))
}

func TestTranslatorErrorsOut(t *testing.T) {
	receiverID := component.NewID("TestTranslatorErrorsOut")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, nil, true, tt.ToReceiverCreateSettings())
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	err = writePacket(t, addr, segmentHeader+"invalidSegment")
	assert.NoError(t, err, "can not write packet in the "+receiverID.String()+" case")

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		return len(logs) > 0 && strings.Contains(logs[len(logs)-1].Message,
			"X-Ray segment to OT traces conversion failed")
	}, 10*time.Second, 5*time.Millisecond, "poller should log warning because consumer errored out")

	assert.NoError(t, tt.CheckReceiverTraces(udppoller.Transport, 1, 1))
}

func TestSegmentsConsumerErrorsOut(t *testing.T) {
	receiverID := component.NewID("TestSegmentsConsumerErrorsOut")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	addr, rcvr, recordedLogs := createAndOptionallyStartReceiver(t, consumertest.NewErr(errors.New("can't consume traces")), true, tt.ToReceiverCreateSettings())
	defer func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	content, err := os.ReadFile(filepath.Join("../../internal/aws/xray", "testdata", "serverSample.txt"))
	assert.NoError(t, err, "can not read raw segment")

	err = writePacket(t, addr, segmentHeader+string(content))
	assert.NoError(t, err, "can not write packet")

	assert.Eventuallyf(t, func() bool {
		logs := recordedLogs.All()
		return len(logs) > 0 && strings.Contains(logs[len(logs)-1].Message,
			"Trace consumer errored out")
	}, 10*time.Second, 5*time.Millisecond, "poller should log warning because consumer errored out")

	assert.NoError(t, tt.CheckReceiverTraces(udppoller.Transport, 1, 1))
}

func TestPollerCloseError(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID("TestPollerCloseError"))
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, nil, false, tt.ToReceiverCreateSettings())
	mPoller := &mockPoller{closeErr: errors.New("mockPollerCloseErr")}
	rcvr.(*xrayReceiver).poller = mPoller
	rcvr.(*xrayReceiver).server = &mockProxy{}
	err = rcvr.Shutdown(context.Background())
	assert.ErrorIs(t, err, mPoller.closeErr, "expected error")
}

func TestProxyCloseError(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID("TestPollerCloseError"))
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, nil, false, tt.ToReceiverCreateSettings())
	mProxy := &mockProxy{closeErr: errors.New("mockProxyCloseErr")}
	rcvr.(*xrayReceiver).poller = &mockPoller{}
	rcvr.(*xrayReceiver).server = mProxy
	err = rcvr.Shutdown(context.Background())
	assert.ErrorIs(t, err, mProxy.closeErr, "expected error")
}

func TestBothPollerAndProxyCloseError(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID("TestBothPollerAndProxyCloseError"))
	assert.NoError(t, err, "SetupTelemetry should succeed")
	defer func() {
		assert.NoError(t, tt.Shutdown(context.Background()))
	}()

	t.Setenv(defaultRegionEnvName, mockRegion)

	_, rcvr, _ := createAndOptionallyStartReceiver(t, nil, false, tt.ToReceiverCreateSettings())
	mPoller := &mockPoller{closeErr: errors.New("mockPollerCloseErr")}
	mProxy := &mockProxy{closeErr: errors.New("mockProxyCloseErr")}
	rcvr.(*xrayReceiver).poller = mPoller
	rcvr.(*xrayReceiver).server = mProxy
	err = rcvr.Shutdown(context.Background())
	assert.ErrorIs(t, err, mPoller.closeErr, "expected error")
	assert.ErrorIs(t, err, mProxy.closeErr, "expected error")
}

type mockPoller struct {
	closeErr error
}

func (m *mockPoller) SegmentsChan() <-chan udppoller.RawSegment {
	return make(chan udppoller.RawSegment, 1)
}

func (m *mockPoller) Start(_ context.Context) {}

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

func (m *mockProxy) Shutdown(_ context.Context) error {
	if m.closeErr != nil {
		return m.closeErr
	}
	return nil
}

func createAndOptionallyStartReceiver(
	t *testing.T,
	csu consumer.Traces,
	start bool,
	set receiver.CreateSettings) (string, receiver.Traces, *observer.ObservedLogs) {
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
	set.Logger = logger
	rcvr, err := newReceiver(
		&Config{
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
		set,
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

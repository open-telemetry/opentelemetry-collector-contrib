// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var (
	expectedForm = "<165>1 2003-08-24T12:14:15Z 192.0.2.1 myproc 8710 - - It's time to make the do-nuts.\n"
	originalForm = "<165>1 2003-08-24T05:14:15-07:00 192.0.2.1 myproc 8710 - - It's time to make the do-nuts."
)

type exporterTCPTest struct {
	srv net.TCPListener
	exp *syslogexporter
}

type exporterUnixTest struct {
	srv *net.UnixListener
	exp *syslogexporter
}

func exampleLog(t *testing.T) plog.LogRecord {
	buffer := plog.NewLogRecord()
	buffer.Body().SetStr(originalForm)
	timestamp := "2003-08-24T05:14:15-07:00"
	timeStr, err := time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err, "failed to start test syslog server")
	ts := pcommon.NewTimestampFromTime(timeStr)
	buffer.SetTimestamp(ts)
	attrMap := map[string]any{
		"proc_id": "8710", "message": "It's time to make the do-nuts.",
		"appname": "myproc", "hostname": "192.0.2.1", "priority": int64(165),
		"version": int64(1),
	}
	for k, v := range attrMap {
		if _, ok := v.(string); ok {
			buffer.Attributes().PutStr(k, v.(string))
		} else {
			buffer.Attributes().PutInt(k, v.(int64))
		}
	}
	return buffer
}

func logRecordsToLogs(record plog.LogRecord) plog.Logs {
	logs := plog.NewLogs()
	logsSlice := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	ls := logsSlice.AppendEmpty()
	record.CopyTo(ls)
	return logs
}

func createExporterCreateSettings() exporter.Settings {
	return exporter.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}
}

func TestInitExporter(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		network  string
		port     int
		protocol string
	}{
		{
			name:     "TCP",
			endpoint: "test.com",
			network:  "tcp",
			port:     514,
			protocol: "rfc5424",
		},
		{
			name:     "Unix socket",
			endpoint: "test.sock",
			network:  "unix",
			port:     0,
			protocol: "rfc5424",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			_, err := initExporter(&Config{
				Endpoint: testInstance.endpoint,
				Network:  testInstance.network,
				Port:     testInstance.port,
				Protocol: testInstance.protocol,
			}, createExporterCreateSettings())
			assert.NoError(t, err)
		})
	}
}

func buildTCPValidExporter(t *testing.T, server net.TCPListener, cfg *Config) (*syslogexporter, error) {
	var port string
	var err error
	hostPort := server.Addr().String()
	cfg.Endpoint, port, err = net.SplitHostPort(hostPort)
	require.NoError(t, err, "could not parse port")
	cfg.Port, err = strconv.Atoi(port)
	require.NoError(t, err, "type error")
	exp, err := initExporter(cfg, createExporterCreateSettings())
	require.NoError(t, err)
	return exp, err
}

func buildTCPInvalidExporter(t *testing.T, server net.TCPListener, cfg *Config) (*syslogexporter, error) {
	var port string
	var err error
	hostPort := server.Addr().String()
	cfg.Endpoint, port, err = net.SplitHostPort(hostPort)
	require.NoError(t, err, "could not parse endpoint")
	require.NotNil(t, port)
	invalidPort := "112" // Assign invalid port
	cfg.Port, err = strconv.Atoi(invalidPort)
	require.NoError(t, err, "type error")
	exp, err := initExporter(cfg, createExporterCreateSettings())
	require.NoError(t, err)
	return exp, err
}

func createTCPServer() (net.TCPListener, error) {
	var addr net.TCPAddr
	addr.IP = net.IP{127, 0, 0, 1}
	addr.Port = 0
	testServer, err := net.ListenTCP("tcp", &addr)
	return *testServer, err
}

func prepareTCPExporterTest(t *testing.T, cfg *Config, invalidExporter bool) *exporterTCPTest {
	// Start a test syslog server
	var err error
	testServer, err := createTCPServer()
	require.NoError(t, err, "failed to start test syslog server")
	var exp *syslogexporter
	if invalidExporter {
		exp, err = buildTCPInvalidExporter(t, testServer, cfg)
	} else {
		exp, err = buildTCPValidExporter(t, testServer, cfg)
	}
	require.NoError(t, err, "Error building exporter")
	require.NotNil(t, exp)
	return &exporterTCPTest{
		srv: testServer,
		exp: exp,
	}
}

func createTCPTestConfig() *Config {
	config := createDefaultConfig().(*Config)
	config.Network = "tcp"
	config.TLS.Insecure = true
	return config
}

func sendTestMessage(t *testing.T, exp *syslogexporter) error {
	buffer := exampleLog(t)
	logs := logRecordsToLogs(buffer)
	return exp.pushLogsData(context.Background(), logs)
}

func TestTCPSyslogExportSuccess(t *testing.T) {
	test := prepareTCPExporterTest(t, createTCPTestConfig(), false)
	require.NotNil(t, test.exp)
	defer test.srv.Close()
	err := sendTestMessage(t, test.exp)
	assert.NoError(t, err, "could not send message")
	err = test.srv.SetDeadline(time.Now().Add(time.Second * 1))
	require.NoError(t, err, "cannot set deadline")
	conn, err := test.srv.AcceptTCP()
	require.NoError(t, err, "could not accept connection")
	defer conn.Close()
	b, err := io.ReadAll(conn)
	require.NoError(t, err, "could not read all")
	assert.Equal(t, expectedForm, string(b))
}

func TestTCPSyslogExportFail(t *testing.T) {
	test := prepareTCPExporterTest(t, createTCPTestConfig(), true)
	defer test.srv.Close()
	consumerErr := sendTestMessage(t, test.exp)
	var consumerErrorLogs consumererror.Logs
	ok := errors.As(consumerErr, &consumerErrorLogs)
	assert.True(t, ok)
	consumerLogs := consumerErrorLogs.Data()
	rls := consumerLogs.ResourceLogs()
	require.Equal(t, 1, rls.Len())
	scl := rls.At(0).ScopeLogs()
	require.Equal(t, 1, scl.Len())
	lrs := scl.At(0).LogRecords()
	require.Equal(t, 1, lrs.Len())
	droppedLog := lrs.At(0).Body().AsString()
	err := test.srv.SetDeadline(time.Now().Add(time.Second * 1))
	require.NoError(t, err, "cannot set deadline")
	conn, err := test.srv.AcceptTCP()
	require.ErrorContains(t, err, "i/o timeout")
	require.Nil(t, conn)
	assert.ErrorContains(t, consumerErr, "dial tcp 127.0.0.1:112: connect")
	assert.Equal(t, droppedLog, originalForm)
}

func createUnixSocketTestConfig(t *testing.T) *Config {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	return &Config{
		Endpoint: socketPath,
		Network:  "unix",
		Protocol: "rfc5424",
	}
}

func createUnixSocketServer(socketPath string) (*net.UnixListener, error) {
	addr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	listener, err := net.ListenUnix("unix", addr)
	return listener, err
}

func prepareUnixSocketExporterTest(t *testing.T, cfg *Config, invalidExporter bool) *exporterUnixTest {
	var err error

	testServer, err := createUnixSocketServer(cfg.Endpoint)
	require.NoError(t, err, "failed to start test syslog server")

	var exp *syslogexporter
	if invalidExporter {
		invalidCfg := &Config{
			Endpoint: "invalid.sock",
			Network:  "unix",
		}
		exp, err = initExporter(invalidCfg, createExporterCreateSettings())
	} else {
		exp, err = initExporter(cfg, createExporterCreateSettings())
	}
	require.NoError(t, err, "Error building exporter")
	require.NotNil(t, exp)
	return &exporterUnixTest{
		srv: testServer,
		exp: exp,
	}
}

func TestUnixSocketExporterSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows (functionality Unix specific)")
	}

	test := prepareUnixSocketExporterTest(t, createUnixSocketTestConfig(t), false)
	require.NotNil(t, test.exp)
	defer test.srv.Close()

	err := sendTestMessage(t, test.exp)

	assert.NoError(t, err, "could not send message")
	err = test.srv.SetDeadline(time.Now().Add(time.Second * 1))
	require.NoError(t, err, "cannot set deadline")
	conn, err := test.srv.AcceptUnix()
	require.NoError(t, err, "could not accept connection")
	defer conn.Close()
	b, err := io.ReadAll(conn)
	require.NoError(t, err, "could not read all")
	assert.Equal(t, expectedForm, string(b))
}

func TestUnixSocketExporterFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows (functionality Unix specific)")
	}

	test := prepareUnixSocketExporterTest(t, createUnixSocketTestConfig(t), true)
	defer test.srv.Close()

	consumerErr := sendTestMessage(t, test.exp)

	var consumerErrorLogs consumererror.Logs
	ok := errors.As(consumerErr, &consumerErrorLogs)
	assert.True(t, ok)
	consumerLogs := consumerErrorLogs.Data()
	rls := consumerLogs.ResourceLogs()
	require.Equal(t, 1, rls.Len())
	scl := rls.At(0).ScopeLogs()
	require.Equal(t, 1, scl.Len())
	lrs := scl.At(0).LogRecords()
	require.Equal(t, 1, lrs.Len())
	droppedLog := lrs.At(0).Body().AsString()
	err := test.srv.SetDeadline(time.Now().Add(time.Second * 1))
	require.NoError(t, err, "cannot set deadline")
	conn, err := test.srv.AcceptUnix()
	require.ErrorContains(t, err, "i/o timeout")
	require.Nil(t, conn)
	assert.ErrorContains(t, consumerErr, "dial unix invalid.sock: connect: no such file or directory")
	assert.Equal(t, droppedLog, originalForm)
}

func TestTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		network     string
		tlsSettings configtls.ClientConfig
		tlsConfig   *tls.Config
	}{
		{
			name:        "TCP with TLS configuration",
			endpoint:    "test.com",
			network:     "tcp",
			tlsSettings: configtls.ClientConfig{},
			tlsConfig:   &tls.Config{},
		},
		{
			name:        "TCP insecure",
			endpoint:    "test.com",
			network:     "tcp",
			tlsSettings: configtls.ClientConfig{Insecure: true},
			tlsConfig:   nil,
		},
		{
			name:        "UDP with TLS configuration",
			endpoint:    "test.com",
			network:     "udp",
			tlsSettings: configtls.ClientConfig{},
			tlsConfig:   nil,
		},
		{
			name:        "UDP insecure",
			endpoint:    "test.com",
			network:     "udp",
			tlsSettings: configtls.ClientConfig{Insecure: true},
			tlsConfig:   nil,
		},
		{
			name:        "Unix Socket with TLS configuration",
			endpoint:    "test.sock",
			network:     "socket",
			tlsSettings: configtls.ClientConfig{},
			tlsConfig:   nil,
		},
		{
			name:        "Unix Socket insecure",
			endpoint:    "test.sock",
			network:     "socket",
			tlsSettings: configtls.ClientConfig{Insecure: true},
			tlsConfig:   nil,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			exporter, err := initExporter(
				&Config{
					Endpoint: testInstance.endpoint,
					Network:  testInstance.network,
					Port:     514,
					Protocol: "rfc5424",
					TLS:      testInstance.tlsSettings,
				},
				createExporterCreateSettings())

			assert.NoError(t, err)
			if testInstance.tlsConfig != nil {
				assert.NotNil(t, exporter.tlsConfig)
			} else {
				assert.Nil(t, exporter.tlsConfig)
			}
		})
	}
}

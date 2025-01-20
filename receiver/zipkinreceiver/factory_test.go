// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig()

	tReceiver, err := createTracesReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		cfg,
		consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = createTracesReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		cfg,
		consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")
}

func TestCreateReceiverDeprecatedWarning(t *testing.T) {
	cfg := &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: defaultHTTPEndpoint,
		},
		ParseStringTags: false,
	}

	var buffer bytes.Buffer
	writer := zapcore.AddSync(&buffer)

	encoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewCore(encoder, writer, zapcore.DebugLevel)
	logger := zap.New(core)

	set := receivertest.NewNopSettings()
	set.Logger = logger

	_, err := createTracesReceiver(
		context.Background(),
		set,
		cfg,
		consumertest.NewNop(),
	)

	assert.NoError(t, err)

	logOutput := buffer.String()
	if !bytes.Contains([]byte(logOutput), []byte(deprecationConfigMsg)) {
		t.Errorf("Expected log message not found. Got: %s", logOutput)
	}
}

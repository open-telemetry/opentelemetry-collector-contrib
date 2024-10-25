// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewFactorySucceedsAndReturnsValidResult(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())
	assert.Equal(t, component.StabilityLevelDevelopment, f.TracesToTracesStability())
	assert.Equal(t, component.StabilityLevelDevelopment, f.LogsToLogsStability())
}

func TestNewFactoryDefaultConfigIsValid(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)

	cfg := f.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCanStartUpAndShutDownTracesToTracesWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := connectortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	tracesToTraces, createErr := f.CreateTracesToTraces(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, tracesToTraces)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := tracesToTraces.Start(startCtx, host)
	assert.NoError(t, startErr)

	shutDownCtx := context.Background()
	shutDownErr := tracesToTraces.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanStartUpAndShutDownLogsToLogsWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := connectortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	logsToLogs, createErr := f.CreateLogsToLogs(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, logsToLogs)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := logsToLogs.Start(startCtx, host)
	assert.NoError(t, startErr)

	shutDownCtx := context.Background()
	shutDownErr := logsToLogs.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanWriteTracesToTracesWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := connectortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	tracesToTraces, createErr := f.CreateTracesToTraces(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, tracesToTraces)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := tracesToTraces.Start(startCtx, host)
	assert.NoError(t, startErr)

	consumeCtx := context.Background()
	dataToConsume := ptrace.NewTraces()
	consumeErr := tracesToTraces.ConsumeTraces(consumeCtx, dataToConsume)
	assert.NoError(t, consumeErr)

	shutDownCtx := context.Background()
	shutDownErr := tracesToTraces.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanWriteLogsToLogsWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := connectortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	logsToLogs, createErr := f.CreateLogsToLogs(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, logsToLogs)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := logsToLogs.Start(startCtx, host)
	assert.NoError(t, startErr)

	consumeCtx := context.Background()
	dataToConsume := plog.NewLogs()
	consumeErr := logsToLogs.ConsumeLogs(consumeCtx, dataToConsume)
	assert.NoError(t, consumeErr)

	shutDownCtx := context.Background()
	shutDownErr := logsToLogs.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

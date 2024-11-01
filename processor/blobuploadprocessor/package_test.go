// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactorySucceedsAndReturnsValidResult(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())
	assert.Equal(t, component.StabilityLevelDevelopment, f.TracesStability())
	assert.Equal(t, component.StabilityLevelDevelopment, f.LogsStability())
}

func TestNewFactoryDefaultConfigIsValid(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)

	cfg := f.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCanStartUpAndShutDownTracesWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := processortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	Traces, createErr := f.CreateTraces(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, Traces)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := Traces.Start(startCtx, host)
	assert.NoError(t, startErr)

	shutDownCtx := context.Background()
	shutDownErr := Traces.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanStartUpAndShutDownLogsWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := processortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	Logs, createErr := f.CreateLogs(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, Logs)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := Logs.Start(startCtx, host)
	assert.NoError(t, startErr)

	shutDownCtx := context.Background()
	shutDownErr := Logs.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanWriteTracesWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := processortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	Traces, createErr := f.CreateTraces(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, Traces)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := Traces.Start(startCtx, host)
	assert.NoError(t, startErr)

	consumeCtx := context.Background()
	dataToConsume := ptrace.NewTraces()
	consumeErr := Traces.ConsumeTraces(consumeCtx, dataToConsume)
	assert.NoError(t, consumeErr)

	shutDownCtx := context.Background()
	shutDownErr := Traces.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

func TestCanWriteLogsWithDefaultConfig(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.NotNil(t, f.CreateDefaultConfig())

	consumer := consumertest.NewNop()
	settings := processortest.NewNopSettings()
	cfg := f.CreateDefaultConfig()
	createCtx := context.Background()
	Logs, createErr := f.CreateLogs(createCtx, settings, cfg, consumer)

	assert.NoError(t, createErr)
	assert.NotNil(t, Logs)

	host := componenttest.NewNopHost()
	startCtx := context.Background()
	startErr := Logs.Start(startCtx, host)
	assert.NoError(t, startErr)

	consumeCtx := context.Background()
	dataToConsume := plog.NewLogs()
	consumeErr := Logs.ConsumeLogs(consumeCtx, dataToConsume)
	assert.NoError(t, consumeErr)

	shutDownCtx := context.Background()
	shutDownErr := Logs.Shutdown(shutDownCtx)
	assert.NoError(t, shutDownErr)
}

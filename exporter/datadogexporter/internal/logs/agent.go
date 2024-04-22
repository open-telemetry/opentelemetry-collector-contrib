// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/logs/auditor"
	"github.com/DataDog/datadog-agent/pkg/logs/client"
	"github.com/DataDog/datadog-agent/pkg/logs/pipeline"
	"github.com/DataDog/datadog-agent/pkg/status/health"
	"github.com/DataDog/datadog-agent/pkg/util/startstop"
	"go.uber.org/zap"
)

const (
	intakeTrackType = "logs"

	// Log messages
	multiLineWarning = "multi_line processing rules are not supported as global processing rules."
)

// Agent represents the data pipeline that collects, decodes, processes and sends logs to the backend.
type Agent struct {
	log      log.Component
	config   pkgconfigmodel.Reader
	hostname hostnameinterface.Component

	endpoints        *config.Endpoints
	auditor          auditor.Auditor
	destinationsCtx  *client.DestinationsContext
	pipelineProvider pipeline.Provider
	health           *health.Handle
}

func NewLogsAgent(log log.Component, cfg pkgconfigmodel.Reader, hostname hostnameinterface.Component) *Agent {
	logsAgent := &Agent{
		log:      log,
		config:   cfg,
		hostname: hostname,
	}
	return logsAgent
}

func (a *Agent) Start(context.Context) error {
	a.log.Debug("Starting logs-agent...")

	// setup the server config
	endpoints, err := buildEndpoints(a.config)

	if err != nil {
		message := fmt.Sprintf("Invalid endpoints: %v", err)
		return errors.New(message)
	}

	a.endpoints = endpoints

	err = a.setupAgent()

	if err != nil {
		_ = a.log.Error("Could not start logs-agent: ", zap.Error(err))
		return err
	}

	a.startPipeline()
	a.log.Debug("logs-agent started")

	return nil
}

func (a *Agent) setupAgent() error {
	// setup global processing rules
	processingRules, err := config.GlobalProcessingRules(a.config)
	if err != nil {
		message := fmt.Sprintf("Invalid processing rules: %v", err)
		return errors.New(message)
	}

	if config.HasMultiLineRule(processingRules) {
		_ = a.log.Warn(multiLineWarning)
	}

	a.SetupPipeline(processingRules)
	return nil
}

// Start starts all the elements of the data pipeline
// in the right order to prevent data loss
func (a *Agent) startPipeline() {
	starter := startstop.NewStarter(
		a.destinationsCtx,
		a.auditor,
		a.pipelineProvider,
	)
	starter.Start()
}

func (a *Agent) Stop(context.Context) error {
	a.log.Debug("Stopping logs-agent")

	stopper := startstop.NewSerialStopper(
		a.pipelineProvider,
		a.auditor,
		a.destinationsCtx,
	)

	// This will try to stop everything in order, including the potentially blocking
	// parts like the sender. After StopTimeout it will just stop the last part of the
	// pipeline, disconnecting it from the auditor, to make sure that the pipeline is
	// flushed before stopping.
	// TODO: Add this feature in the stopper.
	c := make(chan struct{})
	go func() {
		stopper.Stop()
		close(c)
	}()
	timeout := time.Duration(a.config.GetInt("logs_config.stop_grace_period")) * time.Second
	select {
	case <-c:
	case <-time.After(timeout):
		a.log.Debug("Timed out when stopping logs-agent, forcing it to stop now")
		// We force all destinations to read/flush all the messages they get without
		// trying to write to the network.
		a.destinationsCtx.Stop()
		// Wait again for the stopper to complete.
		// In some situation, the stopper unfortunately never succeed to complete,
		// we've already reached the grace period, give it some more seconds and
		// then force quit.
		timeout := time.NewTimer(5 * time.Second)
		select {
		case <-c:
		case <-timeout.C:
			_ = a.log.Warn("Force close of the Logs Agent.")
		}
	}
	a.log.Debug("logs-agent stopped")
	return nil
}

func (a *Agent) GetPipelineProvider() pipeline.Provider {
	return a.pipelineProvider
}

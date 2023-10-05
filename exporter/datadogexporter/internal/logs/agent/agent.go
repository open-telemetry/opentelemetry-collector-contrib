// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package logs

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"time"

	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	"github.com/DataDog/datadog-agent/pkg/conf"
	"github.com/DataDog/datadog-agent/pkg/logs/auditor"
	"github.com/DataDog/datadog-agent/pkg/logs/client"
	"github.com/DataDog/datadog-agent/pkg/logs/message"
	"github.com/DataDog/datadog-agent/pkg/logs/pipeline"
	"github.com/DataDog/datadog-agent/pkg/status/health"
	"github.com/DataDog/datadog-agent/pkg/util/go_routines"
	"github.com/DataDog/datadog-agent/pkg/util/startstop"

	"go.uber.org/atomic"
)

const (
	// key used to display a warning message on the agent status
	invalidProcessingRules = "invalid_global_processing_rules"
	invalidEndpoints       = "invalid_endpoints"
	intakeTrackType        = "logs"

	// Log messages
	multiLineWarning = "multi_line processing rules are not supported as global processing rules."
)

type dependencies struct {
	Log    *zap.Logger
	Config conf.Component
}

// agent represents the data pipeline that collects, decodes,
// processes and sends logs to the backend.  See the package README for
// a description of its operation.
type Agent struct {
	log    *zap.Logger
	config conf.ConfigReader

	endpoints        *config.Endpoints
	auditor          auditor.Auditor
	destinationsCtx  *client.DestinationsContext
	pipelineProvider pipeline.Provider
	health           *health.Handle

	// started is true if the logs agent is running
	started         *atomic.Bool
	getHostnameFunc message.GetHostnameFunc
}

func NewLogsAgent(log *zap.Logger, cfg conf.ConfigReader) *Agent {
	logsAgent := &Agent{
		log:     log,
		config:  cfg,
		started: atomic.NewBool(false),
	}

	return logsAgent
}

func (a *Agent) Start(context.Context) error {
	a.log.Info("Starting logs-agent...")

	// setup the server config
	endpoints, err := buildEndpoints(a.config)

	if err != nil {
		message := fmt.Sprintf("Invalid endpoints: %v", err)
		return errors.New(message)
	}

	a.endpoints = endpoints

	err = a.setupAgent()

	if err != nil {
		a.log.Error("Could not start logs-agent: ", zap.Error(err))
		return err
	}

	a.startPipeline()
	a.log.Info("logs-agent started")

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
		a.log.Warn(multiLineWarning)
	}

	a.SetupPipeline(processingRules)
	return nil
}

// Start starts all the elements of the data pipeline
// in the right order to prevent data loss
func (a *Agent) startPipeline() {
	a.started.Store(true)

	starter := startstop.NewStarter(
		a.destinationsCtx,
		a.auditor,
		a.pipelineProvider,
	)
	starter.Start()
}

func (a *Agent) Stop(context.Context) error {
	a.log.Info("Stopping logs-agent")

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
		a.log.Info("Timed out when stopping logs-agent, forcing it to stop now")
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
			a.log.Warn("Force close of the Logs Agent, dumping the Go routines.")
			if stack, err := go_routines.GetGoRoutinesDump(a.config); err != nil {
				a.log.Warn(fmt.Sprintf("can't get the Go routines dump: %s\n", err))
			} else {
				a.log.Warn(stack)
			}
		}
	}
	a.log.Info("logs-agent stopped")
	return nil
}

func (a *Agent) GetPipelineProvider() pipeline.Provider {
	return a.pipelineProvider
}

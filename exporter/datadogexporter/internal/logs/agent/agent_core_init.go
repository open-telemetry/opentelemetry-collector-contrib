// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !serverless

package logs

import (
	"time"

	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	"github.com/DataDog/datadog-agent/pkg/conf"
	"github.com/DataDog/datadog-agent/pkg/logs/auditor"
	"github.com/DataDog/datadog-agent/pkg/logs/client"
	"github.com/DataDog/datadog-agent/pkg/logs/client/http"
	diagnostic "github.com/DataDog/datadog-agent/pkg/logs/diagnostic/module"
	"github.com/DataDog/datadog-agent/pkg/logs/pipeline"
	"github.com/DataDog/datadog-agent/pkg/status/health"
)

// NewAgent returns a new Logs Agent
func (a *Agent) SetupPipeline(
	processingRules []*config.ProcessingRule,
) {
	health := health.RegisterLiveness("logs-agent")

	// setup the auditor
	// We pass the health handle to the auditor because it's the end of the pipeline and the most
	// critical part. Arguably it could also be plugged to the destination.
	auditorTTL := time.Duration(a.config.GetInt("logs_config.auditor_ttl")) * time.Hour
	auditor := auditor.New(a.config.GetString("logs_config.run_path"), auditor.DefaultRegistryFilename, auditorTTL, health)
	destinationsCtx := client.NewDestinationsContext()

	// setup the pipeline provider that provides pairs of processor and sender
	pipelineProvider := pipeline.NewProvider(config.NumberOfPipelines, auditor, &diagnostic.NoopMessageReceiver{}, processingRules, a.endpoints, destinationsCtx, a.config, a.getHostnameFunc, nil, nil)

	a.auditor = auditor
	a.destinationsCtx = destinationsCtx
	a.pipelineProvider = pipelineProvider

}

// buildEndpoints builds endpoints for the logs agent
func buildEndpoints(coreConfig conf.ConfigReader) (*config.Endpoints, error) {
	httpConnectivity := config.HTTPConnectivityFailure
	if endpoints, err := config.BuildHTTPEndpointsWithVectorOverride(coreConfig, intakeTrackType, config.AgentJSONIntakeProtocol, config.DefaultIntakeOrigin); err == nil {
		httpConnectivity = http.CheckConnectivity(endpoints.Main, coreConfig)
	}
	return config.BuildEndpointsWithVectorOverride(coreConfig, httpConnectivity, intakeTrackType, config.AgentJSONIntakeProtocol, config.DefaultIntakeOrigin)
}

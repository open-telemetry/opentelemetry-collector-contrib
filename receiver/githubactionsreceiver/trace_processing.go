// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type jsonTracesUnmarshaler struct {
	logger *zap.Logger
}

func (j *jsonTracesUnmarshaler) UnmarshalTraces(blob []byte, config *Config) (ptrace.Traces, error) {
	var event map[string]json.RawMessage
	if err := json.Unmarshal(blob, &event); err != nil {
		j.logger.Error("Failed to unmarshal blob", zap.Error(err))
		return ptrace.Traces{}, err
	}

	var traces ptrace.Traces
	if _, ok := event["workflow_job"]; ok {
		var jobEvent WorkflowJobEvent
		err := json.Unmarshal(blob, &jobEvent)
		if err != nil {
			j.logger.Error("Failed to unmarshal job event", zap.Error(err))
			return ptrace.Traces{}, err
		}
		j.logger.Debug("Unmarshalling WorkflowJobEvent")
		traces, err = eventToTraces(&jobEvent, config, j.logger)
		if err != nil {
			j.logger.Error("Failed to convert event to traces", zap.Error(err))
			return ptrace.Traces{}, err
		}
	} else if _, ok := event["workflow_run"]; ok {
		var runEvent WorkflowRunEvent
		err := json.Unmarshal(blob, &runEvent)
		if err != nil {
			j.logger.Error("Failed to unmarshal run event", zap.Error(err))
			return ptrace.Traces{}, err
		}
		j.logger.Debug("Unmarshalling WorkflowRunEvent")
		traces, err = eventToTraces(&runEvent, config, j.logger)
		if err != nil {
			j.logger.Error("Failed to convert event to traces", zap.Error(err))
			return ptrace.Traces{}, err
		}
	} else {
		j.logger.Warn("Unknown event type")
		return ptrace.Traces{}, fmt.Errorf("unknown event type")
	}

	return traces, nil
}

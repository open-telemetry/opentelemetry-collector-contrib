/*
 * Unless explicitly stated otherwise all files in this repository are licensed
 * under the Apache License Version 2.0.
 *
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2020 Datadog, Inc.
 */
package datadogexporter

import (
	"C"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DataDog/datadog-agent/pkg/trace/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
)

var (
	obfuscator     *obfuscate.Obfuscator
	edgeConnection TraceEdgeConnection
)

type (
	RawTracePayload struct {
		Message string `json:"message"`
		Tags    string `json:"tags"`
	}
)

// Configure will set up the bindings
//export Configure
func Configure(rootURL, apiKey string, InsecureSkipVerify bool) {
	// Need to make a copy of these values, otherwise the underlying memory
	// might be cleaned up by the runtime.
	localRootURL := fmt.Sprintf("%s", rootURL)
	localAPIKey := fmt.Sprintf("%s", apiKey)

	obfuscator = obfuscate.NewObfuscator(&obfuscate.Config{
		ES: obfuscate.JSONSettings{
			Enabled: true,
		},
		Mongo: obfuscate.JSONSettings{
			Enabled: true,
		},
		RemoveQueryString: true,
		RemovePathDigits:  true,
		RemoveStackTraces: true,
		Redis:             true,
		Memcached:         true,
	})
	edgeConnection = CreateTraceEdgeConnection(localRootURL, localAPIKey, InsecureSkipVerify)
}

// returns 0 on success, 1 on error
//export ForwardTraces
func ForwardTraces(serializedTraces string) int {
	rawTracePayloads, err := unmarshalSerializedTraces(serializedTraces)
	if err != nil {
		fmt.Printf("Couldn't forward traces: %v", err)
		return 1
	}

	processedTracePayloads, err := processRawTracePayloads(rawTracePayloads)
	if err != nil {
		fmt.Printf("Couldn't forward traces: %v", err)
		return 1
	}

	aggregatedTracePayloads := aggregateTracePayloadsByEnv(processedTracePayloads)

	err = sendTracesToIntake(aggregatedTracePayloads)
	if err != nil {
		fmt.Printf("Couldn't forward traces: %v", err)
		return 1
	}

	return 0
}

func unmarshalSerializedTraces(serializedTraces string) ([]RawTracePayload, error) {
	var rawTracePayloads []RawTracePayload
	err := json.Unmarshal([]byte(serializedTraces), &rawTracePayloads)

	if err != nil {
		return rawTracePayloads, fmt.Errorf("Couldn't unmarshal serialized traces, %v", err)
	}

	return rawTracePayloads, nil
}

func processRawTracePayloads(rawTracePayloads []RawTracePayload) ([]*pb.TracePayload, error) {
	var processedTracePayloads []*pb.TracePayload
	for _, rawTracePayload := range rawTracePayloads {
		traceList, err := ProcessTrace(rawTracePayload.Message, obfuscator, rawTracePayload.Tags)
		if err != nil {
			return processedTracePayloads, err
		}
		processedTracePayloads = append(processedTracePayloads, traceList...)
	}
	return processedTracePayloads, nil
}

func aggregateTracePayloadsByEnv(tracePayloads []*pb.TracePayload) []*pb.TracePayload {
	lookup := make(map[string]*pb.TracePayload)
	for _, tracePayload := range tracePayloads {
		key := fmt.Sprintf("%s|%s", tracePayload.HostName, tracePayload.Env)
		var existingPayload *pb.TracePayload
		if val, ok := lookup[key]; ok {
			existingPayload = val
		} else {
			existingPayload = &pb.TracePayload{
				HostName: tracePayload.HostName,
				Env:      tracePayload.Env,
				Traces:   make([]*pb.APITrace, 0),
			}
			lookup[key] = existingPayload
		}
		existingPayload.Traces = append(existingPayload.Traces, tracePayload.Traces...)
	}

	newPayloads := make([]*pb.TracePayload, 0)

	for _, tracePayload := range lookup {
		newPayloads = append(newPayloads, tracePayload)
	}
	return newPayloads
}

func sendTracesToIntake(tracePayloads []*pb.TracePayload) error {
	hadErr := false
	for _, tracePayload := range tracePayloads {
		err := edgeConnection.SendTraces(context.Background(), tracePayload, 3)
		if err != nil {
			fmt.Printf("Failed to send traces with error %v\n", err)
			hadErr = true
		}
		stats := ComputeAPMStats(tracePayload)
		err = edgeConnection.SendStats(context.Background(), stats, 3)
		if err != nil {
			fmt.Printf("Failed to send trace stats with error %v\n", err)
			hadErr = true
		}
	}
	if hadErr {
		return errors.New("Failed to send traces or stats to intake")
	}
	return nil
}

func main() {}

/*
 * Unless explicitly stated otherwise all files in this repository are licensed
 * under the Apache License Version 2.0.
 *
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2020 Datadog, Inc.
 */
package datadogexporter

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalSerializedTraces(t *testing.T) {
	input := "[{\"message\":\"traces\",\"tags\":\"tag1:value\"},{\"message\":\"traces\",\"tags\":\"tag1:value\"}]"

	output, _ := unmarshalSerializedTraces(input)

	assert.Equal(t, output[0].Message, "traces")
	assert.Equal(t, output[0].Tags, "tag1:value")
	assert.Equal(t, output[1].Message, "traces")
	assert.Equal(t, output[1].Tags, "tag1:value")
}

func TestAggregateTracePayloadsByEnv(t *testing.T) {
	payload1 := pb.TracePayload{
		HostName: "Host",
		Env:      "Env1",
		Traces:   make([]*pb.APITrace, 0),
	}

	payload2 := pb.TracePayload{
		HostName: "Host",
		Env:      "Env1",
		Traces:   make([]*pb.APITrace, 0),
	}

	payload3 := pb.TracePayload{
		HostName: "Host",
		Env:      "Env2",
		Traces:   make([]*pb.APITrace, 0),
	}

	input := []*pb.TracePayload{&payload1, &payload2, &payload3}
	output := aggregateTracePayloadsByEnv(input)
	assert.Equal(t, len(output), 2)
}

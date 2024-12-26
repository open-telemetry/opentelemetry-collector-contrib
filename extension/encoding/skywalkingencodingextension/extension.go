// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/skywalkingencodingextension"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"

	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	skywalkingProto = "skywalking_proto"
)

type skywalkingExtension struct {
	config *Config
}

func (e *skywalkingExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	segment := &agentV3.SegmentObject{}
	err := proto.Unmarshal(buf, segment)
	if err != nil {
		return ptrace.Traces{}, err
	}
	return skywalking.ProtoToTraces(segment), nil
}

func (e *skywalkingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *skywalkingExtension) Shutdown(_ context.Context) error {
	return nil
}

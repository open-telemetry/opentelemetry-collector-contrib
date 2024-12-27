// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/skywalkingencodingextension"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/protobuf/proto"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking"
)

type skywalkingProtobufTrace struct {
}

func (j skywalkingProtobufTrace) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	segment := &agentV3.SegmentObject{}
	err := proto.Unmarshal(buf, segment)
	if err != nil {
		return ptrace.Traces{}, err
	}
	return skywalking.ProtoToTraces(segment), nil
}

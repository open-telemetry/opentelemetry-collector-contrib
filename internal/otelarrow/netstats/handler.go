// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"

import (
	"context"
	"strings"

	"google.golang.org/grpc/stats"
)

type netstatsContext struct{} // value: string

type statsHandler struct {
	rep *NetworkReporter
}

var _ stats.Handler = statsHandler{}

func (rep *NetworkReporter) Handler() stats.Handler {
	return statsHandler{rep: rep}
}

// TagRPC implements grpc/stats.Handler
func (statsHandler) TagRPC(ctx context.Context, s *stats.RPCTagInfo) context.Context {
	return context.WithValue(ctx, netstatsContext{}, s.FullMethodName)
}

// trustUncompressed is a super hacky way of knowing when the
// uncompressed size is realistic.  nothing else would work -- the
// same handler is used by both arrow and non-arrow, and the
// `*stats.Begin` which indicates streaming vs. not streaming does not
// appear in TagRPC() where we could store it in context.  this
// approach is considered a better alternative than others, however
// ugly.  when non-arrow RPCs are sent, the instrumentation works
// correctly, this avoids special instrumentation outside of the Arrow
// components.
func trustUncompressed(method string) bool {
	return !strings.Contains(method, "arrow.v1")
}

func (h statsHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	switch rs.(type) {
	case *stats.InHeader, *stats.InTrailer, *stats.Begin, *stats.OutHeader, *stats.OutTrailer:
		// Note we have some info about header WireLength,
		// but intentionally not counting.
		return
	}
	method := "unknown"
	if name := ctx.Value(netstatsContext{}); name != nil {
		method = name.(string)
	}
	switch s := rs.(type) {
	case *stats.InPayload:
		var ss SizesStruct
		ss.Method = method
		if trustUncompressed(method) {
			ss.Length = int64(s.Length)
		}
		ss.WireLength = int64(s.WireLength)
		h.rep.CountReceive(ctx, ss)

	case *stats.OutPayload:
		var ss SizesStruct
		ss.Method = method
		if trustUncompressed(method) {
			ss.Length = int64(s.Length)
		}
		ss.WireLength = int64(s.WireLength)
		h.rep.CountSend(ctx, ss)
	}
}

// TagConn implements grpc/stats.Handler
func (statsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn implements grpc/stats.Handler
func (statsHandler) HandleConn(_ context.Context, _ stats.ConnStats) {
	// Note: ConnBegin and ConnEnd
}

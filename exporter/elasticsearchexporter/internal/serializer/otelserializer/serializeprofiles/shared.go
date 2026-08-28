// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"bytes"

	"go.opentelemetry.io/ebpf-profiler/libpf"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
)

type (
	unixTime64 = serializer.UnixTime64
	frameID    = serializer.FrameID
)

func newUnixTime64(t uint64) unixTime64 { return serializer.NewUnixTime64(t) }

func newFrameID(fileID libpf.FileID, addressOrLineno libpf.AddressOrLineno) frameID {
	return serializer.NewFrameID(fileID, addressOrLineno)
}

func newFrameIDFromString(s string) (frameID, error) { return serializer.NewFrameIDFromString(s) }

func newFrameIDFromBytes(b []byte) (frameID, error) { return serializer.NewFrameIDFromBytes(b) }

func encodeFrameTypesTo(dst *bytes.Buffer, frameTypes []libpf.FrameType) {
	serializer.EncodeFrameTypesTo(dst, frameTypes)
}

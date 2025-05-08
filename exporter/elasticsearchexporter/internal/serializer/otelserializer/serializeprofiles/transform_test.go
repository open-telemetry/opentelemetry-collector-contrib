// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
)

var (
	stacktraceIDBase64 = stacktraceIDFormat(0xcafebeef, 0xd00d1eaf)

	buildID, buildIDEncoded, buildIDBase64 = formatFileIDFormat(0x0011223344556677,
		0x8899aabbccddeeff)
	buildID2, buildID2Encoded, buildID2Base64 = formatFileIDFormat(0x0112233445566778,
		0x899aabbccddeeffe)
	buildID3, buildID3Encoded, _ = formatFileIDFormat(0x1122334455667788,
		0x99aabbccddeeffee)

	frameIDBase64  = libpf.NewFrameID(buildID, address).String()
	frameID2Base64 = libpf.NewFrameID(buildID2, address2).String()
	frameID3Base64 = libpf.NewFrameID(buildID3, address3).String()
)

const (
	address  = 111
	address2 = 222
	address3 = 333
)

func stacktraceIDFormat(hi, lo uint64) string {
	// Base64() is used in the host agent to encode stacktraceID.
	return libpf.NewFileID(hi, lo).Base64()
}

func formatFileIDFormat(hi, lo uint64) (fileID libpf.FileID, fileIDHex, fileIDBase64 string) {
	// StringNoQuotes() is used in the host agent to encode stacktraceID and buildID.
	// We should possibly switch to Base64 encoding.
	fileID = libpf.NewFileID(hi, lo)
	fileIDHex = fileID.StringNoQuotes()
	fileIDBase64 = fileID.Base64()
	return
}

func TestTransform(t *testing.T) {
	wantedTraceID := mkStackTraceID(t, []libpf.FrameID{
		libpf.NewFrameID(buildID, address),
		libpf.NewFrameID(buildID2, address2),
	})
	for _, tt := range []struct {
		name                  string
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantPayload []StackPayload
		wantErr     error
	}{
		{
			name: "with an empty sample",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.StringTable().Append("samples", "count", "cpu", "nanoseconds")
				st := p.SampleType().AppendEmpty()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := p.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)

				p.Sample().AppendEmpty()

				return rp
			},

			wantPayload: nil,
			wantErr:     nil,
		},
		{
			name: "with an invalid profiling type",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.StringTable().Append("off-CPU", "events")
				st := p.SampleType().AppendEmpty()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)

				p.Sample().AppendEmpty()

				return rp
			},

			wantPayload: nil,
			wantErr:     errors.New("expected sampling type of  [[\"samples\",\"count\"]] but got [[\"off-CPU\", \"events\"]]"),
		},
		{
			name: "with no sample value and no line number on location",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.StringTable().Append("samples", "count", "cpu", "nanoseconds")
				st := p.SampleType().AppendEmpty()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := p.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(42)

				l := p.LocationTable().AppendEmpty()
				l.SetAddress(111)

				return rp
			},

			wantPayload: nil,
			wantErr:     nil,
		},
		{
			name: "with a single indexed sample",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				a := p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)

				p.StringTable().Append("firefox", "libc.so", "samples", "count", "cpu", "nanoseconds")
				st := p.SampleType().AppendEmpty()
				st.SetTypeStrindex(2)
				st.SetUnitStrindex(3)
				pt := p.PeriodType()
				pt.SetTypeStrindex(4)
				pt.SetUnitStrindex(5)

				m := p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(0)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(2)
				m.SetFilenameStrindex(1)

				l := p.LocationTable().AppendEmpty()
				l.SetAddress(address)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(0)
				l = p.LocationTable().AppendEmpty()
				l.SetAddress(address2)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(1)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(42)
				s.Value().Append(1)
				s.SetLocationsLength(2)
				s.SetLocationsStartIndex(0)

				return rp
			},

			wantPayload: []StackPayload{
				{
					StackTrace: StackTrace{
						EcsVersion: EcsVersion{V: EcsVersionString},
						DocID:      wantedTraceID,
						FrameIDs:   frameID2Base64 + frameIDBase64,
						Types: frameTypesToString([]libpf.FrameType{
							libpf.NativeFrame,
							libpf.NativeFrame,
						}),
					},
					StackFrames: []StackFrame{},
					Executables: []ExeMetadata{
						NewExeMetadata(
							buildIDBase64,
							GetStartOfWeekFromTime(time.Now()),
							buildIDBase64,
							"firefox",
						),
						NewExeMetadata(
							buildID2Base64,
							GetStartOfWeekFromTime(time.Now()),
							buildID2Base64,
							"libc.so",
						),
					},
					UnsymbolizedLeafFrames: []UnsymbolizedLeafFrame{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      frameIDBase64,
							FrameID:    []string{frameIDBase64},
						},
					},
					UnsymbolizedExecutables: []UnsymbolizedExecutable{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildIDBase64,
							FileID:     []string{buildIDBase64},
						},
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildID2Base64,
							FileID:     []string{buildID2Base64},
						},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    42000000000,
						StackTraceID: wantedTraceID,
						Count:        1,
					},
				},
			},
			wantErr: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rp := tt.buildResourceProfiles()
			sp := rp.ScopeProfiles().At(0)

			payload, err := Transform(rp.Resource(), sp.Scope(), sp.Profiles().At(0))
			require.NoError(t, checkAndResetTimes(payload))
			sortPayloads(payload)
			sortPayloads(tt.wantPayload)
			require.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantPayload, payload)
		})
	}
}

func TestStackPayloads(t *testing.T) {
	wantedTraceID := mkStackTraceID(t, []libpf.FrameID{
		libpf.NewFrameID(buildID, address),
		libpf.NewFrameID(buildID2, address2),
	})
	for _, tt := range []struct {
		name                  string
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantPayload []StackPayload
		wantErr     error
	}{
		{ //nolint:dupl
			name: "with a single indexed sample",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.StringTable().Append(stacktraceIDBase64, "firefox", "libc.so")

				a := p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")

				l := p.LocationTable().AppendEmpty()
				l.SetMappingIndex(0)
				l.SetAddress(address)
				l.AttributeIndices().Append(3)
				l = p.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address2)
				l.AttributeIndices().Append(3)

				m := p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(1)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(2)
				m.SetFilenameStrindex(2)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(1)
				s.Value().Append(1)
				s.SetLocationsLength(2)
				s.SetLocationsStartIndex(0)

				return rp
			},

			wantPayload: []StackPayload{
				{
					StackTrace: StackTrace{
						EcsVersion: EcsVersion{V: EcsVersionString},
						DocID:      wantedTraceID,
						FrameIDs:   frameID2Base64 + frameIDBase64,
						Types: frameTypesToString([]libpf.FrameType{
							libpf.FrameType(3),
							libpf.FrameType(3),
						}),
					},
					StackFrames: []StackFrame{},
					Executables: []ExeMetadata{
						NewExeMetadata(
							buildIDBase64,
							GetStartOfWeekFromTime(time.Now()),
							buildIDBase64,
							"firefox",
						),
						NewExeMetadata(
							buildID2Base64,
							GetStartOfWeekFromTime(time.Now()),
							buildID2Base64,
							"libc.so",
						),
					},
					UnsymbolizedLeafFrames: []UnsymbolizedLeafFrame{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      frameIDBase64,
							FrameID:    []string{frameIDBase64},
						},
					},
					UnsymbolizedExecutables: []UnsymbolizedExecutable{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildIDBase64,
							FileID:     []string{buildIDBase64},
						},
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildID2Base64,
							FileID:     []string{buildID2Base64},
						},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Count:        1,
					},
				},
			},
		},
		{
			name: "with a duplicated sample",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.StringTable().Append(stacktraceIDBase64, "firefox", "libc.so")

				a := p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")

				l := p.LocationTable().AppendEmpty()
				l.SetMappingIndex(0)
				l.SetAddress(address)
				l.AttributeIndices().Append(2)
				l = p.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address2)
				l.AttributeIndices().Append(2)

				m := p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)
				m.SetFilenameStrindex(1)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(2)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(1)
				s.Value().Append(2)
				s.SetLocationsLength(2)
				s.SetLocationsStartIndex(0)

				return rp
			},

			wantPayload: []StackPayload{
				{
					StackTrace: StackTrace{
						EcsVersion: EcsVersion{V: EcsVersionString},
						DocID:      wantedTraceID,
						FrameIDs:   frameID2Base64 + frameIDBase64,
						Types: frameTypesToString([]libpf.FrameType{
							libpf.FrameType(3),
							libpf.FrameType(3),
						}),
					},
					StackFrames: []StackFrame{},
					Executables: []ExeMetadata{
						NewExeMetadata(
							buildIDBase64,
							GetStartOfWeekFromTime(time.Now()),
							buildIDBase64,
							"firefox",
						),
						NewExeMetadata(
							buildID2Base64,
							GetStartOfWeekFromTime(time.Now()),
							buildID2Base64,
							"libc.so",
						),
					},
					UnsymbolizedLeafFrames: []UnsymbolizedLeafFrame{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      frameIDBase64,
							FrameID:    []string{frameIDBase64},
						},
					},
					UnsymbolizedExecutables: []UnsymbolizedExecutable{
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildIDBase64,
							FileID:     []string{buildIDBase64},
						},
						{
							EcsVersion: EcsVersion{V: EcsVersionString},
							DocID:      buildID2Base64,
							FileID:     []string{buildID2Base64},
						},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Count:        2,
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rp := tt.buildResourceProfiles()
			sp := rp.ScopeProfiles().At(0)

			payloads, err := stackPayloads(rp.Resource(), sp.Scope(), sp.Profiles().At(0))
			require.NoError(t, checkAndResetTimes(payloads))
			sortPayloads(payloads)
			sortPayloads(tt.wantPayload)
			require.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantPayload, payloads)
		})
	}
}

func TestStackTraceEvent(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		timestamp             uint64
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantEvent StackTraceEvent
	}{
		{
			name: "sets host specific data",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				_ = rp.Resource().Attributes().FromRaw(map[string]any{
					string(semconv.ServiceVersionKey): "1.2.0",
				})

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.StringTable().Append(stacktraceIDBase64)

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				StackTraceID: stacktraceIDBase64,
				Count:        1,
			},
		},
		{
			name:      "sets the timestamp",
			timestamp: 1000000000,
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.StringTable().Append(stacktraceIDBase64)

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				TimeStamp:    1000000000000000000,
				StackTraceID: stacktraceIDBase64,
				Count:        1,
			},
		},
		{
			name: "sets the stack trace ID",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.StringTable().Append(stacktraceIDBase64)

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				StackTraceID: stacktraceIDBase64,
				Count:        1,
			},
		},
		{
			name: "sets event specific data",
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.StringTable().Append(stacktraceIDBase64)

				a := p.AttributeTable().AppendEmpty()
				a.SetKey(string(semconv.K8SPodNameKey))
				a.Value().SetStr("my_pod")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey(string(semconv.ContainerNameKey))
				a.Value().SetStr("my_container")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey(string(semconv.ThreadNameKey))
				a.Value().SetStr("my_thread")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey(string(semconv.ServiceNameKey))
				a.Value().SetStr("my_service")

				s := p.Sample().AppendEmpty()
				s.AttributeIndices().Append(0, 1, 2, 3)

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:    EcsVersion{V: EcsVersionString},
				PodName:       "my_pod",
				ContainerName: "my_container",
				ThreadName:    "my_thread",
				StackTraceID:  stacktraceIDBase64,
				Count:         1,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rp := tt.buildResourceProfiles()
			p := rp.ScopeProfiles().At(0).Profiles().At(0)
			s := p.Sample().At(0)

			event := stackTraceEvent(stacktraceIDBase64, p, s, map[string]string{})
			event.TimeStamp = newUnixTime64(tt.timestamp)

			assert.Equal(t, tt.wantEvent, event)
		})
	}
}

func TestStackTrace(t *testing.T) {
	for _, tt := range []struct {
		name         string
		buildProfile func() pprofile.Profile

		wantTrace StackTrace
	}{
		{
			name: "creates a stack trace",
			buildProfile: func() pprofile.Profile {
				p := pprofile.NewProfile()

				a := p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("kernel")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("dotnet")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("profile.frame.type")
				a.Value().SetStr("native")
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = p.AttributeTable().AppendEmpty()
				a.SetKey("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID3Encoded)

				p.StringTable().Append(
					stacktraceIDBase64,
					"kernel",
					"native",
					"dotnet",
				)

				l := p.LocationTable().AppendEmpty()
				l.SetMappingIndex(0)
				l.SetAddress(address)
				l.AttributeIndices().Append(0)
				l = p.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address2)
				l.AttributeIndices().Append(1)
				l = p.LocationTable().AppendEmpty()
				l.SetMappingIndex(2)
				l.SetAddress(address3)
				l.AttributeIndices().Append(2)

				li := l.Line().AppendEmpty()
				li.SetLine(1)
				li = l.Line().AppendEmpty()
				li.SetLine(3)

				m := p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(3)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(4)
				m = p.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(5)

				s := p.Sample().AppendEmpty()
				s.SetLocationsLength(3)

				return p
			},

			wantTrace: StackTrace{
				EcsVersion: EcsVersion{V: EcsVersionString},
				FrameIDs:   frameID3Base64 + frameID2Base64 + frameIDBase64,
				Types: frameTypesToString([]libpf.FrameType{
					libpf.KernelFrame,
					libpf.DotnetFrame,
					libpf.NativeFrame,
				}),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.buildProfile()
			s := p.Sample().At(0)

			frames, frameTypes, _, err := stackFrames(p, s)
			require.NoError(t, err)

			stacktrace := stackTrace("", frames, frameTypes)
			assert.Equal(t, tt.wantTrace, stacktrace)

			assert.Len(t, frameTypes, len(frames))
		})
	}
}

// frameTypesToString converts a slice of FrameType to a RLE encoded string as stored in ES.
//
// Decode such strings with e.g. 'echo -n Ago | basenc --base64url -d | od -t x1'.
// Output "02 0a" means 02 frames with type 0a (10).
// In libpf/frametype.go you find DotnetFrame with value 10.
func frameTypesToString(frameTypes []libpf.FrameType) string {
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	encodeFrameTypesTo(buf, frameTypes)
	return buf.String()
}

func mkStackTraceID(t *testing.T, frameIDs []libpf.FrameID) string {
	p := pprofile.NewProfile()
	s := p.Sample().AppendEmpty()
	s.SetLocationsLength(int32(len(frameIDs)))

	a := p.AttributeTable().AppendEmpty()
	a.SetKey("profile.frame.type")
	a.Value().SetStr("native")

	for i, frameID := range frameIDs {
		p.StringTable().Append(frameID.FileID().StringNoQuotes())

		a := p.AttributeTable().AppendEmpty()
		a.SetKey("process.executable.build_id.htlhash")
		a.Value().SetStr(frameID.FileID().StringNoQuotes())

		m := p.MappingTable().AppendEmpty()
		m.AttributeIndices().Append(int32(i + 1))

		l := p.LocationTable().AppendEmpty()
		l.SetMappingIndex(int32(i))
		l.SetAddress(uint64(frameID.AddressOrLine()))
		l.AttributeIndices().Append(0)
	}

	frames, _, _, err := stackFrames(p, s)
	require.NoError(t, err)

	traceID, err := stackTraceID(frames)
	require.NoError(t, err)

	return traceID
}

// sortPayloads brings the payloads into a deterministic form to allow comparisons.
func sortPayloads(payloads []StackPayload) {
	for idx := range payloads {
		payload := &payloads[idx]
		sort.Slice(payload.UnsymbolizedExecutables, func(i, j int) bool {
			return payload.UnsymbolizedExecutables[i].DocID < payload.UnsymbolizedExecutables[j].DocID
		})
	}
}

func checkAndResetTimes(payloads []StackPayload) error {
	var errs []error
	for i := range payloads {
		payload := &payloads[i]
		for j := range payload.UnsymbolizedLeafFrames {
			frame := &payload.UnsymbolizedLeafFrames[j]
			if !isWithinLastSecond(frame.Created) {
				errs = append(errs, fmt.Errorf("payload[%d].UnsymbolizedLeafFrames[%d].Created is too old: %v",
					i, j, frame.Created))
			}
			if !isWithinLastSecond(frame.Next) {
				errs = append(errs, fmt.Errorf("payload[%d].UnsymbolizedLeafFrames[%d].Next is too old: %v",
					i, j, frame.Next))
			}
			frame.Created = time.Time{}
			frame.Next = time.Time{}
		}
		for j := range payload.UnsymbolizedExecutables {
			executable := &payload.UnsymbolizedExecutables[j]
			if !isWithinLastSecond(executable.Created) {
				errs = append(errs, fmt.Errorf("payload[%d].UnsymbolizedExecutables[%d].Created is too old: %v",
					i, j, executable.Created))
			}
			if !isWithinLastSecond(executable.Next) {
				errs = append(errs, fmt.Errorf("payload[%d].UnsymbolizedExecutables[%d].Next is too old: %v",
					i, j, executable.Next))
			}
			executable.Created = time.Time{}
			executable.Next = time.Time{}
		}
	}
	return errors.Join(errs...)
}

func isWithinLastSecond(t time.Time) bool {
	return time.Since(t) < time.Second
}

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

	frameIDBase64  = newFrameID(buildID, address).String()
	frameID2Base64 = newFrameID(buildID2, address2).String()
	frameID3Base64 = newFrameID(buildID3, address3).String()
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
	return fileID, fileIDHex, fileIDBase64
}

func TestTransform(t *testing.T) {
	wantedTraceID := mkStackTraceID(t, []frameID{
		newFrameID(buildID, address),
		newFrameID(buildID2, address2),
	})
	for _, tt := range []struct {
		name                  string
		buildDictionary       func() pprofile.ProfilesDictionary
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantPayload []StackPayload
		wantErr     error
	}{
		{
			name: "with an empty sample",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("samples", "count", "cpu", "nanoseconds")
				dic.StackTable().AppendEmpty()

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				st := p.SampleType()
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
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("off-CPU", "events")
				dic.StackTable().AppendEmpty()

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				st := p.SampleType()
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
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("samples", "count", "cpu", "nanoseconds")
				dic.StackTable().AppendEmpty()
				l := dic.LocationTable().AppendEmpty()
				l.SetAddress(111)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				st := p.SampleType()
				st.SetTypeStrindex(0)
				st.SetUnitStrindex(1)
				pt := p.PeriodType()
				pt.SetTypeStrindex(2)
				pt.SetUnitStrindex(3)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(42)

				return rp
			},

			wantPayload: nil,
			wantErr:     nil,
		},
		{
			name: "with a single indexed sample",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				stack := dic.StackTable().AppendEmpty()

				dic.StringTable().Append("firefox", "libc.so", "samples", "count", "cpu", "nanoseconds")

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(6)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(7)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(8)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)

				dic.MappingTable().AppendEmpty()
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(0)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(2)
				m.SetFilenameStrindex(1)

				l := dic.LocationTable().AppendEmpty()
				l.SetAddress(address)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(1)
				l = dic.LocationTable().AppendEmpty()
				l.SetAddress(address2)
				l.AttributeIndices().Append(0)
				l.SetMappingIndex(2)
				stack.LocationIndices().Append(0, 1)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.SetPeriod(1e9 / 20)

				st := p.SampleType()
				st.SetTypeStrindex(2)
				st.SetUnitStrindex(3)
				pt := p.PeriodType()
				pt.SetTypeStrindex(4)
				pt.SetUnitStrindex(5)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(42)
				s.Values().Append(1)
				s.SetStackIndex(0)

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
					HostMetadata: HostResourceData{
						Data: map[string]string{},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    42000000000,
						StackTraceID: wantedTraceID,
						Frequency:    20,
						Count:        1,
					},
					HostMetadata: HostResourceData{},
				},
			},
			wantErr: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dic := tt.buildDictionary()
			rp := tt.buildResourceProfiles()
			sp := rp.ScopeProfiles().At(0)

			payload, err := Transform(dic, rp.Resource(), sp.Scope(), sp.Profiles().At(0))
			require.NoError(t, checkAndResetTimes(payload))
			sortPayloads(payload)
			sortPayloads(tt.wantPayload)
			require.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantPayload, payload)
		})
	}
}

func TestStackPayloads(t *testing.T) {
	wantedTraceID := mkStackTraceID(t, []frameID{
		newFrameID(buildID, address),
		newFrameID(buildID2, address2),
	})
	for name, tt := range map[string]struct {
		buildDictionary       func() pprofile.ProfilesDictionary
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantPayload []StackPayload
		wantErr     error
	}{
		//nolint:dupl
		"with a single indexed sample": {
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				stack := dic.StackTable().AppendEmpty()
				dic.StringTable().Append(stacktraceIDBase64, "firefox", "libc.so")

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(3)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(4)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(5)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(6)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address)
				l.AttributeIndices().Append(3)
				l = dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(2)
				l.SetAddress(address2)
				l.AttributeIndices().Append(3)
				stack.LocationIndices().Append(0, 1)

				dic.MappingTable().AppendEmpty()
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(1)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(2)
				m.SetFilenameStrindex(2)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.SetPeriod(1e9 / 20)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(1)
				s.Values().Append(1)
				s.SetStackIndex(0)

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
					HostMetadata: HostResourceData{
						Data: map[string]string{},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Frequency:    20,
						Count:        1,
					},
					HostMetadata: HostResourceData{},
				},
			},
		},
		"with a duplicated sample": {
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				stack := dic.StackTable().AppendEmpty()
				dic.StringTable().Append(stacktraceIDBase64, "firefox", "libc.so")

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(3)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(4)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(5)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address)
				l.AttributeIndices().Append(2)
				l = dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(2)
				l.SetAddress(address2)
				l.AttributeIndices().Append(2)
				stack.LocationIndices().Append(0, 1)

				dic.MappingTable().AppendEmpty()
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)
				m.SetFilenameStrindex(1)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(2)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.SetPeriod(1e9 / 20)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(1)
				s.Values().Append(2)

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
					HostMetadata: HostResourceData{
						Data: map[string]string{},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Frequency:    20,
						Count:        1,
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Frequency:    20,
						Count:        1,
					},
				},
			},
		},
		"with a mapping without BuildID": {
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				stack := dic.StackTable().AppendEmpty()
				dic.StringTable().Append(stacktraceIDBase64, "firefox", "libc.so", "no_build_id_binary")

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(4)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(5)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(6)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address)
				l.AttributeIndices().Append(2)
				l = dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(2)
				l.SetAddress(address2)
				l.AttributeIndices().Append(2)
				stack.LocationIndices().Append(0, 1)

				dic.MappingTable().AppendEmpty()
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(0)
				m.SetFilenameStrindex(1)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(1)
				m.SetFilenameStrindex(2)
				m = dic.MappingTable().AppendEmpty()
				// No build ID attribute for this mapping
				m.SetFilenameStrindex(3)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()
				p.SetPeriod(1e9 / 20)

				s := p.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(1)
				s.Values().Append(1)

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
						// Note: no ExeMetadata for the third mapping since it has no BuildID
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
						// Note: no unsymbolized executable for the mapping without build ID
					},
					HostMetadata: HostResourceData{
						Data: map[string]string{},
					},
				},
				{
					StackTraceEvent: StackTraceEvent{
						EcsVersion:   EcsVersion{V: EcsVersionString},
						TimeStamp:    1000000000,
						StackTraceID: wantedTraceID,
						Frequency:    20,
						Count:        1,
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			dic := tt.buildDictionary()
			rp := tt.buildResourceProfiles()
			sp := rp.ScopeProfiles().At(0)

			payloads, err := stackPayloads(dic, rp.Resource(), sp.Scope(), sp.Profiles().At(0))
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
		buildDictionary       func() pprofile.ProfilesDictionary
		buildResourceProfiles func() pprofile.ResourceProfiles

		wantEvent StackTraceEvent
	}{
		{
			name: "sets host specific data",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append(stacktraceIDBase64)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				_ = rp.Resource().Attributes().FromRaw(map[string]any{
					string(semconv.ServiceVersionKey): "1.2.0",
				})

				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				StackTraceID: stacktraceIDBase64,
				Frequency:    20,
				Count:        1,
			},
		},
		{
			name:      "sets the timestamp",
			timestamp: 1000000000,
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append(stacktraceIDBase64)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				TimeStamp:    1000000000000000000,
				StackTraceID: stacktraceIDBase64,
				Frequency:    20,
				Count:        1,
			},
		},
		{
			name: "sets the stack trace ID",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append(stacktraceIDBase64)

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				p.Sample().AppendEmpty()

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:   EcsVersion{V: EcsVersionString},
				StackTraceID: stacktraceIDBase64,
				Frequency:    20,
				Count:        1,
			},
		},
		{
			name: "sets event specific data",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append(stacktraceIDBase64)
				dic.StackTable().AppendEmpty()

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(1)
				dic.StringTable().Append(string(semconv.ThreadNameKey))
				a.Value().SetStr("my_thread")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(2)
				dic.StringTable().Append(string(semconv.ServiceNameKey))
				a.Value().SetStr("my_service")

				return dic
			},
			buildResourceProfiles: func() pprofile.ResourceProfiles {
				rp := pprofile.NewResourceProfiles()
				_ = rp.Resource().Attributes().FromRaw(map[string]any{
					string(semconv.K8SPodNameKey):       "my_pod",
					string(semconv.ContainerNameKey):    "my_container",
					string(semconv.ContainerIDKey):      "my_container_id",
					string(semconv.K8SNamespaceNameKey): "my_k8s_namespace_name",
				})
				sp := rp.ScopeProfiles().AppendEmpty()
				p := sp.Profiles().AppendEmpty()

				s := p.Sample().AppendEmpty()
				s.AttributeIndices().Append(0, 1)

				return rp
			},

			wantEvent: StackTraceEvent{
				EcsVersion:       EcsVersion{V: EcsVersionString},
				PodName:          "my_pod",
				K8sNamespaceName: "my_k8s_namespace_name",
				ContainerName:    "my_container",
				ContainerID:      "my_container_id",
				ThreadName:       "my_thread",
				ServiceName:      "my_service",
				StackTraceID:     stacktraceIDBase64,
				Frequency:        20,
				Count:            1,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dic := tt.buildDictionary()
			rp := tt.buildResourceProfiles()
			p := rp.ScopeProfiles().At(0).Profiles().At(0)
			s := p.Sample().At(0)

			hostMetadata := newHostMetadata(dic, rp.Resource(), rp.ScopeProfiles().At(0).Scope(), p)
			event := stackTraceEvent(dic, stacktraceIDBase64, s, 20, hostMetadata)
			event.TimeStamp = newUnixTime64(tt.timestamp)

			assert.Equal(t, tt.wantEvent, event)
		})
	}
}

func TestStackTrace(t *testing.T) {
	for _, tt := range []struct {
		name            string
		buildDictionary func() pprofile.ProfilesDictionary
		buildProfile    func() pprofile.Profile

		wantTrace StackTrace
	}{
		{
			name: "creates a stack trace",
			buildDictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				stack := dic.StackTable().AppendEmpty()

				dic.StringTable().Append(
					stacktraceIDBase64,
					"kernel",
					"native",
					"dotnet",
				)

				a := dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(4)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("kernel")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(5)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("dotnet")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(6)
				dic.StringTable().Append("profile.frame.type")
				a.Value().SetStr("native")
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(7)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildIDEncoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(8)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID2Encoded)
				a = dic.AttributeTable().AppendEmpty()
				a.SetKeyStrindex(9)
				dic.StringTable().Append("process.executable.build_id.htlhash")
				a.Value().SetStr(buildID3Encoded)

				l := dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(1)
				l.SetAddress(address)
				l.AttributeIndices().Append(0)
				l = dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(2)
				l.SetAddress(address2)
				l.AttributeIndices().Append(1)
				l = dic.LocationTable().AppendEmpty()
				l.SetMappingIndex(3)
				l.SetAddress(address3)
				l.AttributeIndices().Append(2)

				li := l.Line().AppendEmpty()
				li.SetLine(1)
				li = l.Line().AppendEmpty()
				li.SetLine(3)

				// Create a location without build ID
				f := dic.FunctionTable().AppendEmpty()
				dic.StringTable().Append("fibonacci")
				f.SetNameStrindex(int32(dic.StringTable().Len() - 1))
				dic.StringTable().Append("myApp")
				f.SetFilenameStrindex(int32(dic.StringTable().Len() - 1))
				locWithoutBuildID := dic.LocationTable().AppendEmpty()
				locWithoutBuildID.SetMappingIndex(0)
				locWithoutBuildID.AttributeIndices().Append(0)
				li = locWithoutBuildID.Line().AppendEmpty()
				li.SetLine(99)
				stack.LocationIndices().Append(0, 1, 2, 3)

				dic.MappingTable().AppendEmpty() // empty default mapping at pos 0
				m := dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(3)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(4)
				m = dic.MappingTable().AppendEmpty()
				m.AttributeIndices().Append(5)

				return dic
			},
			buildProfile: func() pprofile.Profile {
				p := pprofile.NewProfile()
				p.Sample().AppendEmpty()

				return p
			},

			wantTrace: StackTrace{
				EcsVersion: EcsVersion{V: EcsVersionString},
				FrameIDs:   "5y1yFRb3UCHnLXIVFvdQIQAAAAAAAABj" + frameID3Base64 + frameID2Base64 + frameIDBase64,
				Types: frameTypesToString([]libpf.FrameType{
					libpf.KernelFrame,
					libpf.DotnetFrame,
					libpf.NativeFrame,
					libpf.KernelFrame,
				}),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dic := tt.buildDictionary()
			p := tt.buildProfile()
			s := p.Sample().At(0)

			frames, frameTypes, _, err := stackFrames(dic, s)
			require.NoError(t, err)

			stacktrace := stackTrace("", frames, frameTypes)
			assert.Equal(t, tt.wantTrace, stacktrace)

			assert.Len(t, frameTypes, len(frames))
		})
	}
}

func TestGetLocations(t *testing.T) {
	dic := pprofile.NewProfilesDictionary()

	// By convention location_table[0] is always present with a default value.
	dic.LocationTable().AppendEmpty()
	// Add three locations to the dictionary
	loc1 := dic.LocationTable().AppendEmpty()
	loc1.SetAddress(0x1000)
	loc2 := dic.LocationTable().AppendEmpty()
	loc2.SetAddress(0x2000)
	loc3 := dic.LocationTable().AppendEmpty()
	loc3.SetAddress(0x3000)

	// Create a stack with indices to the locations
	stack := dic.StackTable().AppendEmpty()
	stack.LocationIndices().Append(1, 2, 3)

	// Call getLocations and check the result
	locations := getLocations(dic, stack)
	require.Len(t, locations, 3)
	assert.Equal(t, uint64(0x1000), locations[0].Address())
	assert.Equal(t, uint64(0x2000), locations[1].Address())
	assert.Equal(t, uint64(0x3000), locations[2].Address())

	// Test with empty stack
	emptyStack := dic.StackTable().AppendEmpty()
	locations = getLocations(dic, emptyStack)
	assert.Empty(t, locations)
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

func mkStackTraceID(t *testing.T, frameIDs []frameID) string {
	dic := pprofile.NewProfilesDictionary()
	dic.MappingTable().AppendEmpty()

	p := pprofile.NewProfile()
	indices := make([]int32, len(frameIDs))
	for i := range frameIDs {
		indices[i] = int32(i)
	}
	s := p.Sample().AppendEmpty()

	a := dic.AttributeTable().AppendEmpty()
	a.SetKeyStrindex(0)
	dic.StringTable().Append("profile.frame.type")
	a.Value().SetStr("native")

	stack := dic.StackTable().AppendEmpty()

	for i, frameID := range frameIDs {
		dic.StringTable().Append(frameID.FileID().StringNoQuotes())

		a := dic.AttributeTable().AppendEmpty()
		a.SetKeyStrindex(int32(dic.StringTable().Len()))
		dic.StringTable().Append("process.executable.build_id.htlhash")
		a.Value().SetStr(frameID.FileID().StringNoQuotes())

		m := dic.MappingTable().AppendEmpty()
		m.AttributeIndices().Append(int32(i + 1))

		l := dic.LocationTable().AppendEmpty()
		l.SetMappingIndex(int32(i + 1))
		l.SetAddress(uint64(frameID.AddressOrLine()))
		l.AttributeIndices().Append(0)

		stack.LocationIndices().Append(int32(dic.LocationTable().Len() - 1))
	}

	frames, _, _, err := stackFrames(dic, s)
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

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func mustNewID(t *testing.T, s string) component.ID {
	t.Helper()
	var id component.ID
	require.NoError(t, id.UnmarshalText([]byte(s)))
	return id
}

func TestValidateStreams(t *testing.T) {
	t.Parallel()

	encA := mustNewID(t, "fake/a")
	encB := mustNewID(t, "fake/b")

	tests := []struct {
		name    string
		routes  []CloudWatchStream
		wantErr string // substring; empty means no error
	}{
		{
			name: "valid: name with explicit log group",
			routes: []CloudWatchStream{
				{Name: "lambda-payments", LogGroupPattern: "/aws/lambda/payment-*", Encoding: encA},
			},
		},
		{
			name: "valid: name with explicit log stream",
			routes: []CloudWatchStream{
				{Name: "vpc-eni", LogStreamPattern: "eni-*", Encoding: encA},
			},
		},
		{
			name: "valid: known name with no patterns (defaults will apply)",
			routes: []CloudWatchStream{
				{Name: "vpcflow", Encoding: encA},
			},
		},
		{
			name: "valid: known name with explicit pattern (overrides default)",
			routes: []CloudWatchStream{
				{Name: "lambda", LogGroupPattern: "/custom/*", Encoding: encA},
			},
		},
		{
			name: "missing name",
			routes: []CloudWatchStream{
				{LogGroupPattern: "/aws/lambda/*", Encoding: encA},
			},
			wantErr: "'name' is required",
		},
		{
			name: "unknown name with no patterns",
			routes: []CloudWatchStream{
				{Name: "unknown-service", Encoding: encA},
			},
			wantErr: `name "unknown-service" has no defaults`,
		},
		{
			name: "missing encoding",
			routes: []CloudWatchStream{
				{Name: "foo", LogGroupPattern: "/aws/lambda/*"},
			},
			wantErr: "'encoding' is required",
		},
		{
			name: "duplicate name",
			routes: []CloudWatchStream{
				{Name: "vpcflow", Encoding: encA},
				{Name: "vpcflow", Encoding: encB},
			},
			wantErr: `duplicate name "vpcflow"`,
		},
		{
			name: "invalid payload",
			routes: []CloudWatchStream{
				{Name: "vpcflow", Encoding: encA, Payload: "raw"},
			},
			wantErr: `invalid 'payload' value "raw"`,
		},
		{
			name: "valid: explicit message payload",
			routes: []CloudWatchStream{
				{Name: "vpcflow", Encoding: encA, Payload: PayloadMessage},
			},
		},
		{
			name: "valid: explicit envelope payload",
			routes: []CloudWatchStream{
				{Name: "lambda", Encoding: encA, Payload: PayloadEnvelope},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateStreams(tt.routes)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestWithDefaults(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")

	tests := []struct {
		name string
		in   CloudWatchStream
		want CloudWatchStream
	}{
		{
			name: "known name no patterns no payload -> pattern and payload defaulted",
			in:   CloudWatchStream{Name: "vpcflow", Encoding: encID},
			want: CloudWatchStream{Name: "vpcflow", Encoding: encID, LogStreamPattern: "eni-*", Payload: PayloadEnvelope},
		},
		{
			name: "known name with explicit pattern -> pattern kept, payload defaulted",
			in:   CloudWatchStream{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID},
			want: CloudWatchStream{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID, Payload: PayloadEnvelope},
		},
		{
			name: "known name with explicit payload -> pattern defaulted, payload kept",
			in:   CloudWatchStream{Name: "lambda", Encoding: encID, Payload: PayloadEnvelope},
			want: CloudWatchStream{Name: "lambda", Encoding: encID, LogGroupPattern: "/aws/lambda/*", Payload: PayloadEnvelope},
		},
		{
			name: "known name with explicit pattern and payload -> nothing changed",
			in:   CloudWatchStream{Name: "lambda", LogGroupPattern: "/custom/*", Encoding: encID, Payload: PayloadEnvelope},
			want: CloudWatchStream{Name: "lambda", LogGroupPattern: "/custom/*", Encoding: encID, Payload: PayloadEnvelope},
		},
		{
			name: "unknown name -> payload defaulted to message",
			in:   CloudWatchStream{Name: "weird", Encoding: encID},
			want: CloudWatchStream{Name: "weird", Encoding: encID, Payload: DefaultPayloadMode},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.in.withDefaults())
		})
	}
}

func TestSortStreams(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")
	// in() builds an input stream (no payload set; user-style).
	in := func(group, stream string) CloudWatchStream {
		return CloudWatchStream{LogGroupPattern: group, LogStreamPattern: stream, Encoding: encID}
	}
	// out() builds the post-withDefaults form (payload filled to DefaultPayloadMode).
	out := func(group, stream string) CloudWatchStream {
		return CloudWatchStream{LogGroupPattern: group, LogStreamPattern: stream, Encoding: encID, Payload: DefaultPayloadMode}
	}

	tests := []struct {
		name  string
		input []CloudWatchStream
		want  []CloudWatchStream
	}{
		{
			name: "catch-all goes last",
			input: []CloudWatchStream{
				in("*", ""),
				in("/aws/lambda/*", ""),
			},
			want: []CloudWatchStream{
				out("/aws/lambda/*", ""),
				out("*", ""),
			},
		},
		{
			name: "log_group before log_stream-only",
			input: []CloudWatchStream{
				in("", "eni-*"),
				in("/aws/lambda/*", ""),
			},
			want: []CloudWatchStream{
				out("/aws/lambda/*", ""),
				out("", "eni-*"),
			},
		},
		{
			name: "more specific group pattern first",
			input: []CloudWatchStream{
				in("/aws/lambda/*", ""),
				in("/aws/lambda/payment-*", ""),
			},
			want: []CloudWatchStream{
				out("/aws/lambda/payment-*", ""),
				out("/aws/lambda/*", ""),
			},
		},
		{
			name: "three-level interaction",
			input: []CloudWatchStream{
				in("*", ""),                     // catch-all
				in("", "eni-*"),                 // stream-only
				in("/aws/lambda/*", ""),         // generic group
				in("/aws/lambda/payment-*", ""), // specific group
			},
			want: []CloudWatchStream{
				out("/aws/lambda/payment-*", ""),
				out("/aws/lambda/*", ""),
				out("", "eni-*"),
				out("*", ""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sortStreams(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSortStreamsAppliesDefaults(t *testing.T) {
	t.Parallel()

	encID := mustNewID(t, "fake/a")
	in := []CloudWatchStream{
		{Name: "lambda", Encoding: encID},  // expands to LogGroupPattern: /aws/lambda/*, payload: message
		{Name: "vpcflow", Encoding: encID}, // expands to LogStreamPattern: eni-*, payload: envelope
	}
	got := sortStreams(in)

	// After defaults: lambda has a log_group_pattern, vpcflow has only log_stream_pattern.
	// Sort places log_group entries before log_stream-only entries.
	require.Len(t, got, 2)
	assert.Equal(t, "/aws/lambda/*", got[0].LogGroupPattern)
	assert.Equal(t, PayloadMessage, got[0].Payload)
	assert.Equal(t, "eni-*", got[1].LogStreamPattern)
	assert.Equal(t, PayloadEnvelope, got[1].Payload)
}

func TestSortStreamsEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, sortStreams(nil))
	assert.Nil(t, sortStreams([]CloudWatchStream{}))
}

// fakePlogUnmarshaler stands in for plog.Unmarshaler inner extensions.
type fakePlogUnmarshaler struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*fakePlogUnmarshaler) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.NewLogs(), nil
}

// nonUnmarshaler implements only component.Component, used to test wrong-type error.
type nonUnmarshaler struct {
	component.StartFunc
	component.ShutdownFunc
}

type fakeHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func newFakeHost(t *testing.T, extensions map[component.ID]component.Component) component.Host {
	t.Helper()
	return &fakeHost{Host: componenttest.NewNopHost(), extensions: extensions}
}

func TestResolveStreams(t *testing.T) {
	t.Parallel()

	innerID := mustNewID(t, "fake/inner")
	otherID := mustNewID(t, "fake/other")
	selfID := mustNewID(t, "aws_logs_encoding/cw_router")

	t.Run("happy path: defaults + sort + payload", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &fakePlogUnmarshaler{},
			otherID: &fakePlogUnmarshaler{},
		})
		streams := []CloudWatchStream{
			{Name: "vpcflow", Encoding: innerID},                        // defaults: stream eni-*, payload envelope
			{Name: "lambda", Encoding: otherID},                         // defaults: group /aws/lambda/*, payload message
			{Name: "catchall", LogGroupPattern: "*", Encoding: innerID}, // explicit catch-all
		}
		routes, err := resolveStreams(streams, host, selfID)
		require.NoError(t, err)
		require.Len(t, routes, 3)

		// After sort: log_group entries first, log_stream-only next, catch-all last.
		assert.Equal(t, "lambda", routes[0].name)
		assert.Equal(t, PayloadMessage, routes[0].payload)
		assert.NotNil(t, routes[0].logGroupParts)
		assert.Nil(t, routes[0].logStreamParts)

		assert.Equal(t, "vpcflow", routes[1].name)
		assert.Equal(t, PayloadEnvelope, routes[1].payload)
		assert.Nil(t, routes[1].logGroupParts)
		assert.NotNil(t, routes[1].logStreamParts)

		assert.Equal(t, "catchall", routes[2].name)
	})

	t.Run("missing extension ID", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{})
		_, err := resolveStreams([]CloudWatchStream{{Name: "vpcflow", Encoding: innerID}}, host, selfID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `encoding extension "fake/inner" not found`)
	})

	t.Run("wrong extension type", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &nonUnmarshaler{},
		})
		_, err := resolveStreams([]CloudWatchStream{{Name: "vpcflow", Encoding: innerID}}, host, selfID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement plog.Unmarshaler")
	})

	t.Run("self-reference cycle", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			selfID: &fakePlogUnmarshaler{},
		})
		_, err := resolveStreams(
			[]CloudWatchStream{{Name: "loop", LogGroupPattern: "*", Encoding: selfID}},
			host, selfID,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refers back to this extension")
	})

	t.Run("empty streams returns nil", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{})
		routes, err := resolveStreams(nil, host, selfID)
		require.NoError(t, err)
		assert.Nil(t, routes)
	})
}

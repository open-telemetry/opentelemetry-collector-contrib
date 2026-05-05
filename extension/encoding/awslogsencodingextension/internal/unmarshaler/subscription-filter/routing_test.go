// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
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
			name: "known name no patterns -> default applied",
			in:   CloudWatchStream{Name: "vpcflow", Encoding: encID},
			want: CloudWatchStream{Name: "vpcflow", Encoding: encID, LogStreamPattern: "eni-*"},
		},
		{
			name: "known name with explicit pattern -> default skipped",
			in:   CloudWatchStream{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID},
			want: CloudWatchStream{Name: "vpcflow", LogGroupPattern: "/custom/*", Encoding: encID},
		},
		{
			name: "unknown name -> route returned unchanged",
			in:   CloudWatchStream{Name: "weird", Encoding: encID},
			want: CloudWatchStream{Name: "weird", Encoding: encID},
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
	mk := func(group, stream string) CloudWatchStream {
		return CloudWatchStream{LogGroupPattern: group, LogStreamPattern: stream, Encoding: encID}
	}

	tests := []struct {
		name  string
		input []CloudWatchStream
		want  []CloudWatchStream
	}{
		{
			name: "catch-all goes last",
			input: []CloudWatchStream{
				mk("*", ""),
				mk("/aws/lambda/*", ""),
			},
			want: []CloudWatchStream{
				mk("/aws/lambda/*", ""),
				mk("*", ""),
			},
		},
		{
			name: "log_group before log_stream-only",
			input: []CloudWatchStream{
				mk("", "eni-*"),
				mk("/aws/lambda/*", ""),
			},
			want: []CloudWatchStream{
				mk("/aws/lambda/*", ""),
				mk("", "eni-*"),
			},
		},
		{
			name: "more specific group pattern first",
			input: []CloudWatchStream{
				mk("/aws/lambda/*", ""),
				mk("/aws/lambda/payment-*", ""),
			},
			want: []CloudWatchStream{
				mk("/aws/lambda/payment-*", ""),
				mk("/aws/lambda/*", ""),
			},
		},
		{
			name: "three-level interaction",
			input: []CloudWatchStream{
				mk("*", ""),                     // catch-all
				mk("", "eni-*"),                 // stream-only
				mk("/aws/lambda/*", ""),         // generic group
				mk("/aws/lambda/payment-*", ""), // specific group
			},
			want: []CloudWatchStream{
				mk("/aws/lambda/payment-*", ""),
				mk("/aws/lambda/*", ""),
				mk("", "eni-*"),
				mk("*", ""),
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
		{Name: "lambda", Encoding: encID},  // expands to LogGroupPattern: /aws/lambda/*
		{Name: "vpcflow", Encoding: encID}, // expands to LogStreamPattern: eni-*
	}
	got := sortStreams(in)

	// After defaults: lambda has a log_group_pattern, vpcflow has only log_stream_pattern.
	// Sort places log_group entries before log_stream-only entries.
	require.Len(t, got, 2)
	assert.Equal(t, "/aws/lambda/*", got[0].LogGroupPattern)
	assert.Equal(t, "eni-*", got[1].LogStreamPattern)
}

func TestSortStreamsEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, sortStreams(nil))
	assert.Nil(t, sortStreams([]CloudWatchStream{}))
}

// fakeLogsExtension is a minimal encoding.LogsUnmarshalerExtension used to
// stand in for inner extensions in router tests. Each instance carries a
// label so tests can assert which inner Match returned.
type fakeLogsExtension struct {
	component.StartFunc
	component.ShutdownFunc
	label string
}

func (*fakeLogsExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.NewLogs(), nil
}

func (*fakeLogsExtension) NewLogsDecoder(_ io.Reader, _ ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return nil, nil
}

// nonLogsExtension implements only component.Component, not the logs
// unmarshaler interface — used to test the wrong-type error.
type nonLogsExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

// fakeHost wraps a map of extensions so router resolution can find them.
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

func TestNewRouter(t *testing.T) {
	t.Parallel()

	innerID := mustNewID(t, "fake/inner")
	otherID := mustNewID(t, "fake/other")
	selfID := mustNewID(t, "aws_logs_encoding/cw_router")

	t.Run("happy path with multiple routes", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &fakeLogsExtension{label: "inner"},
			otherID: &fakeLogsExtension{label: "other"},
		})
		routes := []CloudWatchStream{
			{Name: "vpcflow", Encoding: innerID},                        // default LogStreamPattern: eni-*
			{Name: "lambda", Encoding: otherID},                         // default LogGroupPattern: /aws/lambda/*
			{Name: "catchall", LogGroupPattern: "*", Encoding: innerID}, // catch-all
		}
		router, err := NewRouter(routes, host, selfID)
		require.NoError(t, err)
		require.NotNil(t, router)
		require.Len(t, router.streams, 3)

		// Sort placed log_group entries before log_stream-only, then catch-all last.
		// lambda has /aws/lambda/* (group), vpcflow has eni-* (stream-only), catchall is *.
		assert.Equal(t, "lambda", router.streams[0].name)
		assert.Equal(t, "vpcflow", router.streams[1].name)
		assert.Equal(t, "catchall", router.streams[2].name)

		// Each route's pattern is pre-split.
		assert.NotNil(t, router.streams[0].logGroupParts)
		assert.Nil(t, router.streams[0].logStreamParts)
		assert.Nil(t, router.streams[1].logGroupParts)
		assert.NotNil(t, router.streams[1].logStreamParts)
	})

	t.Run("missing extension ID", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{})
		routes := []CloudWatchStream{
			{Name: "vpcflow", Encoding: innerID},
		}
		_, err := NewRouter(routes, host, selfID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `encoding extension "fake/inner" not found`)
	})

	t.Run("wrong extension type", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &nonLogsExtension{},
		})
		routes := []CloudWatchStream{
			{Name: "vpcflow", Encoding: innerID},
		}
		_, err := NewRouter(routes, host, selfID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not implement encoding.LogsUnmarshalerExtension")
	})

	t.Run("self-reference cycle", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{
			selfID: &fakeLogsExtension{label: "self"},
		})
		routes := []CloudWatchStream{
			{Name: "loop", LogGroupPattern: "*", Encoding: selfID},
		}
		_, err := NewRouter(routes, host, selfID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refers back to this extension")
	})

	t.Run("empty routes returns empty router", func(t *testing.T) {
		t.Parallel()
		host := newFakeHost(t, map[component.ID]component.Component{})
		router, err := NewRouter(nil, host, selfID)
		require.NoError(t, err)
		require.NotNil(t, router)
		assert.Empty(t, router.streams)
	})
}

func TestRouterMatch(t *testing.T) {
	t.Parallel()

	selfID := mustNewID(t, "aws_logs_encoding/cw_router")

	t.Run("matches against router with multiple routes", func(t *testing.T) {
		t.Parallel()

		innerID := mustNewID(t, "fake/inner")
		otherID := mustNewID(t, "fake/other")
		catchID := mustNewID(t, "fake/catch")

		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &fakeLogsExtension{label: "inner"},
			otherID: &fakeLogsExtension{label: "other"},
			catchID: &fakeLogsExtension{label: "catch"},
		})
		routes := []CloudWatchStream{
			{Name: "lambda-payments", LogGroupPattern: "/aws/lambda/payment-*", Encoding: innerID},
			{Name: "lambda", Encoding: otherID},  // default /aws/lambda/*
			{Name: "vpcflow", Encoding: innerID}, // default eni-*
			{Name: "catchall", LogGroupPattern: "*", Encoding: catchID},
		}
		router, err := NewRouter(routes, host, selfID)
		require.NoError(t, err)

		tests := []struct {
			name      string
			logGroup  string
			logStream string
			wantRoute string
		}{
			{
				name:      "specific lambda pattern wins over generic",
				logGroup:  "/aws/lambda/payment-service",
				logStream: "any",
				wantRoute: "lambda-payments",
			},
			{
				name:      "generic lambda matches when payment doesn't",
				logGroup:  "/aws/lambda/orders-service",
				logStream: "any",
				wantRoute: "lambda",
			},
			{
				name:      "log_stream eni-* matches",
				logGroup:  "/whatever/group",
				logStream: "eni-0abc",
				wantRoute: "vpcflow",
			},
			{
				name:      "catch-all when nothing else matches",
				logGroup:  "/random/group",
				logStream: "random-stream",
				wantRoute: "catchall",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				inner, name, err := router.Match(tt.logGroup, tt.logStream)
				require.NoError(t, err)
				assert.Equal(t, tt.wantRoute, name)
				assert.NotNil(t, inner)
			})
		}
	})

	t.Run("returns error when no route matches", func(t *testing.T) {
		t.Parallel()

		innerID := mustNewID(t, "fake/inner")
		host := newFakeHost(t, map[component.ID]component.Component{
			innerID: &fakeLogsExtension{label: "inner"},
		})
		// Single specific route, no catch-all.
		routes := []CloudWatchStream{
			{Name: "lambda", Encoding: innerID}, // default /aws/lambda/*
		}
		router, err := NewRouter(routes, host, selfID)
		require.NoError(t, err)

		inner, name, err := router.Match("/aws/rds/instance/mydb", "anything")
		require.Error(t, err)
		assert.Nil(t, inner)
		assert.Empty(t, name)
		assert.Contains(t, err.Error(), `no route matches logGroup="/aws/rds/instance/mydb"`)
	})
}

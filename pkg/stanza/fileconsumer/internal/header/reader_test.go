// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package header

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/keyvalue"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
)

func TestReader(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"
	regexConf.ParseTo = entry.RootableField{Field: entry.NewBodyField()}

	kvConf := keyvalue.NewConfig()
	kvConf.ParseFrom = entry.NewBodyField("header_line")
	kvConf.Delimiter = ":"

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	cfg, err := NewConfig(set, "^#", []operator.Config{
		{Builder: regexConf},
		{Builder: kvConf},
	}, unicode.UTF8)
	require.NoError(t, err)

	reader, err := NewReader(set, *cfg)
	assert.NoError(t, err)

	attrs := make(map[string]any)
	assert.NoError(t, reader.Process(context.Background(), []byte("# foo:bar\n"), attrs))
	assert.NoError(t, reader.Process(context.Background(), []byte("# hello:world\n"), attrs))
	assert.ErrorIs(t, reader.Process(context.Background(), []byte("First log line"), attrs), ErrEndOfHeader)
	assert.Len(t, attrs, 2)
	assert.Equal(t, "bar", attrs["foo"])
	assert.Equal(t, "world", attrs["hello"])

	assert.NoError(t, reader.Stop())
}

func TestSkipUnmatchedHeaderLine(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"
	regexConf.ParseTo = entry.RootableField{Field: entry.NewBodyField()}

	kvConf := keyvalue.NewConfig()
	kvConf.ParseFrom = entry.NewBodyField("header_line")
	kvConf.Delimiter = ":"

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	cfg, err := NewConfig(set, "^#", []operator.Config{
		{Builder: regexConf},
		{Builder: kvConf},
	}, unicode.UTF8)
	require.NoError(t, err)

	reader, err := NewReader(set, *cfg)
	assert.NoError(t, err)

	attrs := make(map[string]any)
	assert.NoError(t, reader.Process(context.Background(), []byte("# foo:bar\n"), attrs))
	assert.NoError(t, reader.Process(context.Background(), []byte("# matches header regex but not metadata operator assumptions\n"), attrs))
	assert.NoError(t, reader.Process(context.Background(), []byte("# hello:world\n"), attrs))
	assert.ErrorIs(t, reader.Process(context.Background(), []byte("First log line"), attrs), ErrEndOfHeader)
	assert.Len(t, attrs, 2)
	assert.Equal(t, "bar", attrs["foo"])
	assert.Equal(t, "world", attrs["hello"])

	assert.NoError(t, reader.Stop())
}

func TestNewReaderErr(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	_, err := NewReader(set, Config{})
	assert.Error(t, err)
}

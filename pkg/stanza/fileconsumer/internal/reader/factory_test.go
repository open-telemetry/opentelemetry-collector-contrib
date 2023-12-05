// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	defaultMaxLogSize         = 1024 * 1024
	defaultMaxConcurrentFiles = 1024
	defaultEncoding           = "utf-8"
	defaultPollInterval       = 200 * time.Millisecond
	defaultFlushPeriod        = 500 * time.Millisecond
)

func testFactory(t *testing.T, sCfg split.Config, maxLogSize int, flushPeriod time.Duration) (*Factory, *emittest.Sink) {
	enc, err := decode.LookupEncoding(defaultEncoding)
	require.NoError(t, err)

	splitFunc, err := sCfg.Func(enc, false, maxLogSize)
	require.NoError(t, err)

	sink := emittest.NewSink()
	return &Factory{
		SugaredLogger: testutil.Logger(t),
		Config: &Config{
			FingerprintSize: fingerprint.DefaultSize,
			MaxLogSize:      maxLogSize,
			Emit:            sink.Callback,
			FlushTimeout:    flushPeriod,
		},
		FromBeginning: true,
		Encoding:      enc,
		SplitFunc:     splitFunc,
		TrimFunc:      trim.Whitespace,
	}, sink
}

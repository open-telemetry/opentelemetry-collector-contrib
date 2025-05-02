// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

func Test_newTracesReceiver_err(t *testing.T) {
	c := Config{
		Encoding: defaultEncoding,
	}
	_, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultTracesUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
}

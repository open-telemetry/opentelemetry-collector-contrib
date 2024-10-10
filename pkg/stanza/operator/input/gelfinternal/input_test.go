package gelfinternal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	// "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	// "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestHandleGELFMessage_FullMessage(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = ":0"
	set := componenttest.NewNopTelemetrySettings()
	op, build_err := cfg.Build(set)
	require.NoError(t, build_err)
	input, ok := op.(*Input)
	require.True(t, ok)
	input.Start(nil)
	defer input.Stop()

	message := Message{
		Version:  "1.1",
		Host:     "localhost",
		Short:    "hello-world",
		Full:     "hello-world",
		TimeUnix: float64(time.Now().Unix()),
		Level:    1,
		Facility: "test",
	}

	messageId := [8]byte{5, 1, 2, 3, 4, 5, 6, 7}
	data, err := json.Marshal(message)
	require.NoError(t, err)

	var packet []byte

	// Magic number
	packet = append(packet, magicChunked...)
	// Sequence Number
	packet = append(packet, byte(1)) 
	// Total chunks
	packet = append(packet, byte(1))
	// Message ID
	packet = append(packet, messageId[:]...)
	// Data
	packet = append(packet, data...)

	input.HandleGELFMessage(context.Background(), packet, len(packet))

	assert.Equal(t, 1, len(input.buffer))

	assert.Equal(t, 0, len(input.lastBuffer))

	input.HandleGELFMessage(context.Background(), packet, len(packet))

}
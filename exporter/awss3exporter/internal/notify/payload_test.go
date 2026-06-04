// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalBatchSingle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 19, 37, 59, 0, time.UTC)
	events := []Event{{Bucket: "my-bucket", Key: "path/to/object.json", Size: 2048}}

	body, err := marshalBatch(events, now)
	require.NoError(t, err)

	var got s3Envelope
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Records, 1)

	rec := got.Records[0]
	assert.Equal(t, eventSource, rec.EventSource)
	assert.Equal(t, eventName, rec.EventName)
	assert.Equal(t, "2026-04-24T19:37:59Z", rec.EventTime)
	assert.Equal(t, "my-bucket", rec.S3.Bucket.Name)
	assert.Equal(t, "path%2Fto%2Fobject.json", rec.S3.Object.Key)
	assert.Equal(t, int64(2048), rec.S3.Object.Size)
}

func TestMarshalBatchURLEncoding(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 19, 37, 59, 0, time.UTC)
	events := []Event{{
		Bucket: "my-bucket",
		Key:    "raw/plus+char file.json",
		Size:   10,
	}}

	body, err := marshalBatch(events, now)
	require.NoError(t, err)

	// Assert the escaped bytes appear in the wire body so the receiver, which
	// calls url.QueryUnescape, will round-trip back to the original key.
	assert.Contains(t, string(body), "raw%2Fplus%2Bchar+file.json")

	var env s3Envelope
	require.NoError(t, json.Unmarshal(body, &env))
	require.Len(t, env.Records, 1)
	assert.Equal(t, "raw%2Fplus%2Bchar+file.json", env.Records[0].S3.Object.Key)
}

func TestMarshalBatchMultiRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 24, 19, 37, 59, 0, time.UTC)
	events := []Event{
		{Bucket: "b", Key: "k1", Size: 1},
		{Bucket: "b", Key: "k2", Size: 2},
		{Bucket: "b", Key: "k3", Size: 3},
	}

	body, err := marshalBatch(events, now)
	require.NoError(t, err)

	var env s3Envelope
	require.NoError(t, json.Unmarshal(body, &env))
	require.Len(t, env.Records, 3)

	for i, rec := range env.Records {
		assert.Equal(t, events[i].Key, rec.S3.Object.Key,
			"keys in this test have no special characters so escaping is a no-op")
		assert.Equal(t, events[i].Size, rec.S3.Object.Size)
		// Every record shares the batch's eventTime.
		assert.Equal(t, "2026-04-24T19:37:59Z", rec.EventTime)
	}
}

func TestMarshalBatchEventTimeIsUTCRFC3339(t *testing.T) {
	t.Parallel()

	// Local-zone input must be formatted as UTC on the wire.
	loc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	now := time.Date(2026, 4, 24, 12, 37, 59, 0, loc) // 19:37:59 UTC

	body, err := marshalBatch([]Event{{Bucket: "b", Key: "k", Size: 1}}, now)
	require.NoError(t, err)

	var env s3Envelope
	require.NoError(t, json.Unmarshal(body, &env))
	require.Len(t, env.Records, 1)
	assert.True(t, strings.HasSuffix(env.Records[0].EventTime, "Z"),
		"eventTime must be in UTC (Z suffix)")
	assert.Equal(t, "2026-04-24T19:37:59Z", env.Records[0].EventTime)
}

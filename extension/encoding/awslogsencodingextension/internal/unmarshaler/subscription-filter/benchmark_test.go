// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter

import (
	"bytes"
	"fmt"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// cloudTrailMessage is a CloudTrail data event as delivered through a
// subscription filter: a JSON document carried inside the JSON string
// "message", so every quote in it is escaped.
const cloudTrailMessage = `{"eventVersion":"1.11","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLEID:session-%[1]d","arn":"arn:aws:sts::123456789012:assumed-role/ExampleRole/session-%[1]d","accountId":"123456789012","accessKeyId":"ASIAEXAMPLEKEY","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLEID","arn":"arn:aws:iam::123456789012:role/ExampleRole","accountId":"123456789012","userName":"ExampleRole"},"attributes":{"creationDate":"2026-01-01T00:00:00Z","mfaAuthenticated":"false"}}},"eventTime":"2026-01-01T00:00:00Z","eventSource":"dynamodb.amazonaws.com","eventName":"PutItem","awsRegion":"us-east-1","sourceIPAddress":"dynamodb.amazonaws.com","userAgent":"dynamodb.amazonaws.com","requestParameters":{"tableName":"arn:aws:dynamodb:us-east-1:123456789012:table/example","key":{"SK":"ITEM#%[1]d","PK":"ITEM#%[1]d"},"items":["dateCreated","attributes","SK","PK","version","dateUpdated"]},"responseElements":null,"requestID":"EXAMPLEREQUESTID%[1]d","eventID":"00000000-0000-0000-0000-%012[1]d","readOnly":false,"resources":[{"accountId":"123456789012","type":"AWS::DynamoDB::Table","ARN":"arn:aws:dynamodb:us-east-1:123456789012:table/example"}],"eventType":"AwsApiCall","apiVersion":"2012-08-10","managementEvent":false,"recipientAccountId":"123456789012","eventCategory":"Data"}`

const plainMessage = "2026-01-01T00:00:00.000Z INFO request %d completed in 12ms status=200 path=/v1/example user_agent=example-client/1.0"

// newSubscriptionFilterContent returns nEnvelopes decompressed subscription
// filter envelopes, each with nEvents log events whose message is built from
// messageFormat.
func newSubscriptionFilterContent(b *testing.B, nEnvelopes, nEvents int, messageFormat string) []byte {
	b.Helper()
	var buf bytes.Buffer
	for e := range nEnvelopes {
		events := make([]cloudwatchLogsLogEvent, nEvents)
		for i := range events {
			id := e*nEvents + i
			events[i] = cloudwatchLogsLogEvent{
				ID:        fmt.Sprintf("%056d", id),
				Timestamp: 1767225600000,
				Message:   fmt.Sprintf(messageFormat, id),
			}
		}
		data, err := gojson.Marshal(cloudwatchLogsData{
			MessageType:         "DATA_MESSAGE",
			Owner:               "123456789012",
			LogGroup:            "aws-cloudtrail-logs-123456789012",
			LogStream:           "123456789012_CloudTrail_us-east-1",
			SubscriptionFilters: []string{"example-filter"},
			LogEvents:           events,
		})
		require.NoError(b, err)
		buf.Write(data)
	}
	return buf.Bytes()
}

func BenchmarkUnmarshalAWSLogs(b *testing.B) {
	tests := map[string]struct {
		nEnvelopes    int
		nEvents       int
		messageFormat string
	}{
		"plain_1_envelope_1_event":      {nEnvelopes: 1, nEvents: 1, messageFormat: plainMessage},
		"plain_35_envelopes_165_events": {nEnvelopes: 35, nEvents: 165, messageFormat: plainMessage},
		"json_1_envelope_1_event":       {nEnvelopes: 1, nEvents: 1, messageFormat: cloudTrailMessage},
		"json_1_envelope_165_events":    {nEnvelopes: 1, nEvents: 165, messageFormat: cloudTrailMessage},
		"json_35_envelopes_165_events":  {nEnvelopes: 35, nEvents: 165, messageFormat: cloudTrailMessage},
		"json_1_envelope_1000_events":   {nEnvelopes: 1, nEvents: 1000, messageFormat: cloudTrailMessage},
	}

	u := NewSubscriptionFilterUnmarshaler(component.BuildInfo{})
	for name, test := range tests {
		data := newSubscriptionFilterContent(b, test.nEnvelopes, test.nEvents, test.messageFormat)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			r := bytes.NewReader(data)
			for b.Loop() {
				r.Reset(data)
				_, err := u.UnmarshalAWSLogs(r)
				require.NoError(b, err)
			}
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/notify"

import (
	"encoding/json"
	"net/url"
	"time"
)

// The receiver contract expects the AWS S3 event shape. Only the fields
// consumed by lakerunner's pubsub-http parser are load-bearing
// (bucket.name, object.key, object.size); the rest are included for
// compatibility with other AWS-event-shape consumers and for debugging.
const (
	eventSource = "aws:s3"
	eventName   = "ObjectCreated:Put"
)

// s3Envelope is the outer JSON object carried in a single POST. Naming the
// Records slice with a capital R matches the AWS event shape exactly.
type s3Envelope struct {
	Records []s3Record `json:"Records"`
}

type s3Record struct {
	EventSource string   `json:"eventSource"`
	EventName   string   `json:"eventName"`
	EventTime   string   `json:"eventTime"`
	S3          s3Entity `json:"s3"`
}

type s3Entity struct {
	Bucket s3Bucket `json:"bucket"`
	Object s3Object `json:"object"`
}

type s3Bucket struct {
	Name string `json:"name"`
}

type s3Object struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// marshalBatch serializes a slice of events into a single S3-event envelope.
//
// The supplied `now` is written into every record's eventTime field. This
// reflects the moment the notifier serialized the batch rather than the
// per-event upload time, which is acceptable per spec because the field is
// not load-bearing for lakerunner.
//
// Object keys are URL-encoded here; bucket names are written verbatim since
// S3 bucket naming rules already produce transport-safe strings.
func marshalBatch(events []Event, now time.Time) ([]byte, error) {
	eventTime := now.UTC().Format(time.RFC3339)
	envelope := s3Envelope{Records: make([]s3Record, len(events))}
	for i, e := range events {
		envelope.Records[i] = s3Record{
			EventSource: eventSource,
			EventName:   eventName,
			EventTime:   eventTime,
			S3: s3Entity{
				Bucket: s3Bucket{Name: e.Bucket},
				Object: s3Object{
					Key:  url.QueryEscape(e.Key),
					Size: e.Size,
				},
			},
		}
	}
	return json.Marshal(envelope)
}

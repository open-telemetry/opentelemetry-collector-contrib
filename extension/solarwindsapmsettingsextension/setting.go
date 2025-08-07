// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

type Arguments struct {
	BucketCapacity               float64 `json:"BucketCapacity"`
	BucketRate                   float64 `json:"BucketRate"`
	TriggerRelaxedBucketCapacity float64 `json:"TriggerRelaxedBucketCapacity"`
	TriggerRelaxedBucketRate     float64 `json:"TriggerRelaxedBucketRate"`
	TriggerStrictBucketCapacity  float64 `json:"TriggerStrictBucketCapacity"`
	TriggerStrictBucketRate      float64 `json:"TriggerStrictBucketRate"`
}
type Setting struct {
	Value     int64     `json:"value"`
	Flags     string    `json:"flags"`
	Timestamp int64     `json:"timestamp"`
	TTL       int64     `json:"ttl"`
	Arguments Arguments `json:"arguments"`
}

type SettingWithWarning struct {
	Setting
	Warning string `json:"warning"`
}

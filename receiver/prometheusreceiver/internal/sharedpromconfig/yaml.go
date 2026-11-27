// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sharedpromconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"

import (
	"encoding/json"

	"github.com/goccy/go-yaml"
	commonconfig "github.com/prometheus/common/config"
)

// UnredactedYAMLOptions returns options for encoding Prometheus credentials in
// memory during reloads and change detection. Logs and API responses use redaction.
func UnredactedYAMLOptions() []yaml.EncodeOption {
	return []yaml.EncodeOption{
		yaml.CustomMarshaler(func(s commonconfig.Secret) ([]byte, error) {
			return json.Marshal(string(s))
		}),
		// URL.MarshalYAML redacts the password, so emit the full URL instead.
		yaml.CustomMarshaler(marshalUnredactedURL),
		yaml.CustomMarshaler(func(u *commonconfig.URL) ([]byte, error) {
			if u == nil {
				return []byte("null"), nil
			}
			return marshalUnredactedURL(*u)
		}),
	}
}

func marshalUnredactedURL(u commonconfig.URL) ([]byte, error) {
	if u.URL == nil {
		return []byte("null"), nil
	}
	return json.Marshal(u.String())
}

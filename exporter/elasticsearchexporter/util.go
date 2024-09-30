// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/lestrrat-go/strftime"
)

const (
	maxDataStreamBytes       = 100
	disallowedNamespaceRunes = "\\/*?\"<>| ,#:"
	disallowedDatasetRunes   = "-\\/*?\"<>| ,#:"
)

// Sanitize the datastream fields (dataset, namespace) to apply restrictions
// as outlined in https://www.elastic.co/guide/en/ecs/current/ecs-data_stream.html
func sanitizeDataStreamDataset(field string) string {
	field = strings.Map(replaceReservedRune(disallowedDatasetRunes), field)
	if len(field) > maxDataStreamBytes {
		return field[:maxDataStreamBytes]
	}

	return field
}

// Sanitize the datastream fields (dataset, namespace) to apply restrictions
// as outlined in https://www.elastic.co/guide/en/ecs/current/ecs-data_stream.html
func sanitizeDataStreamNamespace(field string) string {
	field = strings.Map(replaceReservedRune(disallowedNamespaceRunes), field)
	if len(field) > maxDataStreamBytes {
		return field[:maxDataStreamBytes]
	}
	return field
}

func replaceReservedRune(disallowedRunes string) func(r rune) rune {
	return func(r rune) rune {
		if strings.ContainsRune(disallowedRunes, r) {
			return '_'
		}
		return unicode.ToLower(r)
	}
}

func generateIndexWithLogstashFormat(index string, conf *LogstashFormatSettings, t time.Time) (string, error) {
	if conf.Enabled {
		partIndex := fmt.Sprintf("%s%s", index, conf.PrefixSeparator)
		var buf bytes.Buffer
		p, err := strftime.New(fmt.Sprintf("%s%s", partIndex, conf.DateFormat))
		if err != nil {
			return partIndex, err
		}
		if err = p.Format(&buf, t); err != nil {
			return partIndex, err
		}
		index = buf.String()
	}
	return index, nil
}

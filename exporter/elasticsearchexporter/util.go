// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"fmt"
	"time"

	"github.com/lestrrat-go/strftime"
)

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

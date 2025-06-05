// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package opensearchexporter

import (
	"bytes"
	"fmt"
	"time"

	"github.com/lestrrat-go/strftime"
)

func GenerateIndexWithLogstashFormat(index string, conf *LogstashFormatSettings, t time.Time) string {
	if conf.Enabled {
		partIndex := fmt.Sprintf("%s%s", index, conf.PrefixSeparator)
		var buf bytes.Buffer
		p, err := strftime.New(fmt.Sprintf("%s%s", partIndex, conf.DateFormat))
		if err != nil {
			return partIndex
		}
		if err = p.Format(&buf, t); err != nil {
			return partIndex
		}
		index = buf.String()
	}
	return index
}

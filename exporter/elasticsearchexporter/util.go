// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"fmt"
	"github.com/lestrrat-go/strftime"
	"time"
)

func generateIndex(index string, conf *LogstashFormatSettings, t time.Time) (string, error) {
	if conf.Enabled {
		partIndex := fmt.Sprintf("%s%s", conf.Prefix, conf.PrefixSeparator)
		var buf bytes.Buffer
		p, err := strftime.New(fmt.Sprintf("%s%s", partIndex, conf.DateFormat))
		if err = p.Format(&buf, t); err != nil {
			return partIndex, err
		}
		index = buf.String()
	}
	return index, nil
}

package opensearchexporter

import (
	"bytes"
	"fmt"
	"github.com/lestrrat-go/strftime"
	"time"
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

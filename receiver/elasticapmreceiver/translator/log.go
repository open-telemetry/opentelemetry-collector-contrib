package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseLog(log *modelpb.Log, attrs pcommon.Map) {
	if log == nil {
		return
	}

	PutOptionalStr(attrs, "log.level", &log.Level)
	PutOptionalStr(attrs, "log.logger", &log.Logger)
	parseLogOrigin(log.Origin, attrs)
}

func parseLogOrigin(origin *modelpb.LogOrigin, attrs pcommon.Map) {
	if origin == nil {
		return
	}

	parseLogOriginFile(origin.File, attrs)
	PutOptionalStr(attrs, "log.origin.function", &origin.FunctionName)
}

func parseLogOriginFile(file *modelpb.LogOriginFile, attrs pcommon.Map) {
	if file == nil {
		return
	}

	PutOptionalStr(attrs, "log.origin.file.name", &file.Name)
	PutOptionalInt(attrs, "log.origin.file.line", &file.Line)
}

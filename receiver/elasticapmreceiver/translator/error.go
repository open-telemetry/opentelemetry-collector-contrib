package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseError(err *modelpb.Error, attrs pcommon.Map) {
	if err == nil {
		return
	}

	// TODO: err.Custom
	parseException(err.Exception, attrs)
	parseErrorLog(err.Log, attrs)
	PutOptionalStr(attrs, "error.id", &err.Id)
	PutOptionalStr(attrs, "error.grouping_key", &err.GroupingKey)
	PutOptionalStr(attrs, "error.culprit", &err.Culprit)
	PutOptionalStr(attrs, "error.stacktrace", &err.StackTrace)
	PutOptionalStr(attrs, "error.message", &err.Message)
	PutOptionalStr(attrs, "error.type", &err.Type)
}

func parseException(exception *modelpb.Exception, attrs pcommon.Map) {
	if exception == nil {
		return
	}

	// TODO: exception.Attributes
	// TODO: exception.Stacktrace
	// TODO: exception.Cause

	PutOptionalStr(attrs, conventions.AttributeExceptionMessage, &exception.Message)
	PutOptionalStr(attrs, "exception.module", &exception.Module)
	PutOptionalStr(attrs, "exception.code", &exception.Code)
	PutOptionalStr(attrs, conventions.AttributeExceptionType, &exception.Type)
	PutOptionalBool(attrs, "exception.handled", exception.Handled)
}

func parseErrorLog(log *modelpb.ErrorLog, attrs pcommon.Map) {
	if log == nil {
		return
	}
	// TODO: error.Stacktrace

	PutOptionalStr(attrs, "error.log.message", &log.Message)
	PutOptionalStr(attrs, "error.log.level", &log.Level)
	PutOptionalStr(attrs, "error.log.param_message", &log.ParamMessage)
	PutOptionalStr(attrs, "error.log.logger_name", &log.LoggerName)
}

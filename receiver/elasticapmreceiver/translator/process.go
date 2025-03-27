package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseProcess(process *modelpb.Process, attrs pcommon.Map) {
	if process == nil {
		return
	}
	PutOptionalInt(attrs, conventions.AttributeProcessParentPID, &process.Ppid)
	parseProcessThread(process.Thread, attrs)
	PutOptionalStr(attrs, conventions.AttributeProcessRuntimeName, &process.Title)
	PutOptionalStr(attrs, conventions.AttributeProcessCommandLine, &process.CommandLine)
	PutOptionalStr(attrs, conventions.AttributeProcessExecutableName, &process.Executable)
	PutStrArray(attrs, conventions.AttributeProcessCommandLine, process.Argv)
	PutOptionalInt(attrs, conventions.AttributeProcessPID, &process.Pid)
}

func parseProcessThread(thread *modelpb.ProcessThread, attrs pcommon.Map) {
	if thread == nil {
		return
	}
	PutOptionalStr(attrs, conventions.AttributeThreadName, &thread.Name)
	PutOptionalInt(attrs, conventions.AttributeThreadID, &thread.Id)
}

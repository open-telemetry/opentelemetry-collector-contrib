package attributes

import (
	"go.opentelemetry.io/collector/translator/conventions"

	"fmt"
)


type processAttributes struct {
	ExecutableName string
	ExecutablePath string
	Command        string
	CommandLine    string
	PID            int64
	Owner          string
}

func (pattrs *processAttributes) extractProcessTags() []string {
	tags := make([]string, 0, 1)

	// According to OTel conventions: https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/process.md,
	// a process can be defined by any of the 4 following attributes: process.executable.name, process.executable.path, process.command or process.command_line
	// (process.command_args isn't in the current attribute conventions: https://github.com/open-telemetry/opentelemetry-collector/blob/ecb27f49d4e26ae42d82e6ea18d57b08e252452d/translator/conventions/opentelemetry.go#L58-L63)
	// We go through them in order of preference (from most concise to less concise), and add the first available one as identifier.
	// We don't add all of them to avoid increasing the number of tags unnecessarily.

	if pattrs.ExecutableName != "" { // otelcol
		tags = append(tags, fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutableName, pattrs.ExecutableName))
	} else if pattrs.ExecutablePath != "" { // /usr/bin/cmd/otelcol
		tags = append(tags, fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutablePath, pattrs.ExecutablePath))
	} else if pattrs.Command != "" { // cmd/otelcol
		tags = append(tags, fmt.Sprintf("%s:%s", conventions.AttributeProcessCommand, pattrs.Command))
	} else if pattrs.CommandLine != "" { // cmd/otelcol --config="/path/to/config.yaml"
		tags = append(tags, fmt.Sprintf("%s:%s", conventions.AttributeProcessCommandLine, pattrs.CommandLine))
	}

	// For now, we don't care about the process ID nor the process owner.

	return tags
}
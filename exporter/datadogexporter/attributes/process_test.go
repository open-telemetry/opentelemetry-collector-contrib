package attributes

import (
	"go.opentelemetry.io/collector/translator/conventions"

	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractProcessTags(t *testing.T) {
	pattrs := processAttributes{
		ExecutableName: "otelcol",
		ExecutablePath: "/usr/bin/cmd/otelcol",
		Command:        "cmd/otelcol",
		CommandLine:    "cmd/otelcol --config=\"/path/to/config.yaml\"",
		PID:            1,
		Owner:          "root",
	}

	assert.Equal(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutableName, "otelcol"),
	}, pattrs.extractProcessTags())

	pattrs = processAttributes{
		ExecutablePath: "/usr/bin/cmd/otelcol",
		Command:        "cmd/otelcol",
		CommandLine:    "cmd/otelcol --config=\"/path/to/config.yaml\"",
		PID:            1,
		Owner:          "root",
	}

	assert.Equal(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutablePath, "/usr/bin/cmd/otelcol"),
	}, pattrs.extractProcessTags())

	pattrs = processAttributes{
		Command:     "cmd/otelcol",
		CommandLine: "cmd/otelcol --config=\"/path/to/config.yaml\"",
		PID:         1,
		Owner:       "root",
	}

	assert.Equal(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessCommand, "cmd/otelcol"),
	}, pattrs.extractProcessTags())

	pattrs = processAttributes{
		CommandLine: "cmd/otelcol --config=\"/path/to/config.yaml\"",
		PID:         1,
		Owner:       "root",
	}

	assert.Equal(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessCommandLine, "cmd/otelcol --config=\"/path/to/config.yaml\""),
	}, pattrs.extractProcessTags())
}

func TestExtractProcessEmpty(t *testing.T) {
	pattrs := processAttributes{}

	assert.Equal(t, []string{}, pattrs.extractProcessTags())
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const waitDuration = 1 * time.Second

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// Build will build a journald input operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	args, err := c.buildArgs()
	if err != nil {
		return nil, err
	}

	return &Input{
		InputOperator: inputOperator,
		newCmd: func(ctx context.Context, cursor []byte) cmd {
			// Copy args and if needed, add the cursor flag
			journalArgs := append([]string{}, args...)
			if cursor != nil {
				journalArgs = append(journalArgs, "--after-cursor", string(cursor))
			}
			return exec.CommandContext(ctx, "journalctl", journalArgs...) // #nosec - ...
			// journalctl is an executable that is required for this operator to function
		},
		json: jsoniter.ConfigFastest,
	}, nil
}

func (c Config) buildArgs() ([]string, error) {
	args := make([]string, 0, 10)

	// Export logs in UTC time
	args = append(args, "--utc")

	// Export logs as JSON
	args = append(args, "--output=json")

	// Continue watching logs until cancelled
	args = append(args, "--follow")

	switch c.StartAt {
	case "end":
	case "beginning":
		args = append(args, "--no-tail")
	default:
		return nil, fmt.Errorf("invalid value '%s' for parameter 'start_at'", c.StartAt)
	}

	for _, unit := range c.Units {
		args = append(args, "--unit", unit)
	}

	for _, identifier := range c.Identifiers {
		args = append(args, "--identifier", identifier)
	}

	args = append(args, "--priority", c.Priority)

	if len(c.Grep) > 0 {
		args = append(args, "--grep", c.Grep)
	}

	if c.Dmesg {
		args = append(args, "--dmesg")
	}

	switch {
	case c.Directory != nil:
		args = append(args, "--directory", *c.Directory)
	case len(c.Files) > 0:
		for _, file := range c.Files {
			args = append(args, "--file", file)
		}
	}

	if len(c.Matches) > 0 {
		matches, err := c.buildMatchesConfig()
		if err != nil {
			return nil, err
		}
		args = append(args, matches...)
	}

	if c.All {
		args = append(args, "--all")
	}

	return args, nil
}

func buildMatchConfig(mc MatchConfig) ([]string, error) {
	re := regexp.MustCompile("^[_A-Z]+$")

	// Sort keys to be consistent with every run and to be predictable for tests
	sortedKeys := make([]string, 0, len(mc))
	for key := range mc {
		if !re.MatchString(key) {
			return []string{}, fmt.Errorf("'%s' is not a valid Systemd field name", key)
		}
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	configs := []string{}
	for _, key := range sortedKeys {
		configs = append(configs, fmt.Sprintf("%s=%s", key, mc[key]))
	}

	return configs, nil
}

func (c Config) buildMatchesConfig() ([]string, error) {
	matches := []string{}

	for i, mc := range c.Matches {
		if i > 0 {
			matches = append(matches, "+")
		}
		mcs, err := buildMatchConfig(mc)
		if err != nil {
			return []string{}, err
		}

		matches = append(matches, mcs...)
	}

	return matches, nil
}

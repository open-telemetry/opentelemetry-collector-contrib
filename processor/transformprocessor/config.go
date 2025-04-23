// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"errors"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

var (
	flatLogsFeatureGate = featuregate.GlobalRegistry().MustRegister("transform.flatten.logs", featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Flatten log data prior to transformation so every record has a unique copy of the resource and scope. Regroups logs based on resource and scope after transformations."),
		featuregate.WithRegisterFromVersion("v0.103.0"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32080#issuecomment-2120764953"),
	)
	errFlatLogsGateDisabled = errors.New("'flatten_data' requires the 'transform.flatten.logs' feature gate to be enabled")
)

// Config defines the configuration for the processor.
type Config struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing a statement.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by statements and continues on to the next statement. This is the recommended mode.
	// `propagate` means the processor returns the error up the pipeline.  This will result in the payload being dropped from the collector.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`

	TraceStatements  []common.ContextStatements `mapstructure:"trace_statements"`
	MetricStatements []common.ContextStatements `mapstructure:"metric_statements"`
	LogStatements    []common.ContextStatements `mapstructure:"log_statements"`

	FlattenData bool `mapstructure:"flatten_data"`
	logger      *zap.Logger
}

// Unmarshal is used internally by mapstructure to parse the transformprocessor configuration (Config),
// adding support to structured and flat configuration styles.
// When the flat configuration style is used, all statements are grouped into a common.ContextStatements
// object, with empty [common.ContextStatements.Context] value.
// On the other hand, structured configurations are parsed following the mapstructure Config format.
//
// Example of flat configuration:
//
//	log_statements:
//	  - set(attributes["service.new_name"], attributes["service.name"])
//	  - delete_key(attributes, "service.name")
//
// Example of structured configuration:
//
//	log_statements:
//	  - context: "span"
//	    statements:
//	      - set(attributes["service.new_name"], attributes["service.name"])
//	      - delete_key(attributes, "service.name")
func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if conf == nil {
		return nil
	}

	contextStatementsFields := map[string]*[]common.ContextStatements{
		"trace_statements":  &c.TraceStatements,
		"metric_statements": &c.MetricStatements,
		"log_statements":    &c.LogStatements,
	}

	contextStatementsPatch := map[string]any{}
	for fieldName := range contextStatementsFields {
		if !conf.IsSet(fieldName) {
			continue
		}
		rawVal := conf.Get(fieldName)
		values, ok := rawVal.([]any)
		if !ok {
			return fmt.Errorf("invalid %s type, expected: array, got: %t", fieldName, rawVal)
		}
		if len(values) == 0 {
			continue
		}

		statementsConfigs := make([]any, 0, len(values))
		var basicStatements []any
		for _, value := range values {
			// Array of strings means it's a basic configuration style
			if reflect.TypeOf(value).Kind() == reflect.String {
				basicStatements = append(basicStatements, value)
			} else {
				if len(basicStatements) > 0 {
					return errors.New("configuring multiple configuration styles is not supported, please use only Basic configuration or only Advanced configuration")
				}
				statementsConfigs = append(statementsConfigs, value)
			}
		}

		if len(basicStatements) > 0 {
			statementsConfigs = append(statementsConfigs, map[string]any{"statements": basicStatements})
		}

		contextStatementsPatch[fieldName] = statementsConfigs
	}

	if len(contextStatementsPatch) > 0 {
		err := conf.Merge(confmap.NewFromStringMap(contextStatementsPatch))
		if err != nil {
			return err
		}
	}

	err := conf.Unmarshal(c)
	if err != nil {
		return err
	}

	return err
}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error {
	var errors error

	if len(c.TraceStatements) > 0 {
		pc, err := common.NewTraceParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithSpanParser(traces.SpanFunctions()), common.WithSpanEventParser(traces.SpanEventFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.TraceStatements {
			_, err = pc.ParseContextStatements(cs)
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if len(c.MetricStatements) > 0 {
		pc, err := common.NewMetricParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithMetricParser(metrics.MetricFunctions()), common.WithDataPointParser(metrics.DataPointFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.MetricStatements {
			_, err := pc.ParseContextStatements(cs)
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if len(c.LogStatements) > 0 {
		pc, err := common.NewLogParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithLogParser(logs.LogFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.LogStatements {
			_, err = pc.ParseContextStatements(cs)
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if c.FlattenData && !flatLogsFeatureGate.IsEnabled() {
		errors = multierr.Append(errors, errFlatLogsGateDisabled)
	}

	return errors
}

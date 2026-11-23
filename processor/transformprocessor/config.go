// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

var (
	errFlatLogsGateDisabled          = errors.New("'flatten.log' requires the 'transform.flatten.logs' feature gate to be enabled")
	errProfileStatementsGateDisabled = errors.New("'profile_statements' requires the 'processor.transform.enableProfiles' feature gate to be enabled")
)

// Config defines the configuration for the processor.
type Config struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing a statement.
	// Valid values are `ignore`, `silent` and `propagate`.
	// `ignore` means the processor ignores errors returned by statements and continues on to the next statement. This is the recommended mode.
	// `silent` is the same as `ignore` but the errors are not logged.
	// `propagate` means the processor returns the error up the pipeline.  This will result in the payload being dropped from the collector.
	// The default value is `ignore`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`

	// TraceStatements configures the OTTL statements executed against trace data.
	TraceStatements []ContextStatements `mapstructure:"trace_statements"`
	// MetricStatements configures the OTTL statements executed against metric data.
	MetricStatements []ContextStatements `mapstructure:"metric_statements"`
	// LogStatements configures the OTTL statements executed against log data.
	LogStatements []ContextStatements `mapstructure:"log_statements"`
	// ProfileStatements configures the OTTL statements executed against profile data.
	ProfileStatements []ContextStatements `mapstructure:"profile_statements"`

	// Flatten configures flattening of the data before the statements run.
	Flatten FlattenConfig `mapstructure:"flatten"`
	logger  *zap.Logger

	dataPointFunctions map[string]ottl.Factory[*ottldatapoint.TransformContext]
	exemplarFunctions  map[string]ottl.Factory[*ottlexemplar.TransformContext]
	logFunctions       map[string]ottl.Factory[*ottllog.TransformContext]
	metricFunctions    map[string]ottl.Factory[*ottlmetric.TransformContext]
	spanEventFunctions map[string]ottl.Factory[*ottlspanevent.TransformContext]
	spanFunctions      map[string]ottl.Factory[*ottlspan.TransformContext]
	profileFunctions   map[string]ottl.Factory[*ottlprofile.TransformContext]
}

// FlattenConfig configures which OTTL contexts the data is flattened to before the statements run.
type FlattenConfig struct {
	// Log gives each log record a distinct copy of its resource and scope.
	// Requires the transform.flatten.logs feature gate.
	Log bool `mapstructure:"log"`
}

var _ ottl.StatementsGetter = (*ContextStatements)(nil)

// ContextID is the name of the OTTL context a group of statements runs against.
type ContextID string

// The OTTL contexts that can be set on [ContextStatements.Context].
const (
	ContextIDResource  ContextID = "resource"
	ContextIDScope     ContextID = "scope"
	ContextIDSpan      ContextID = "span"
	ContextIDSpanEvent ContextID = "spanevent"
	ContextIDMetric    ContextID = "metric"
	ContextIDDataPoint ContextID = "datapoint"
	ContextIDExemplar  ContextID = "exemplar"
	ContextIDLog       ContextID = "log"
	ContextIDProfile   ContextID = "profile"
)

// UnmarshalText sets the ContextID from its case-insensitive name and rejects unknown contexts.
func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case ContextIDResource, ContextIDScope, ContextIDSpan, ContextIDSpanEvent, ContextIDMetric, ContextIDDataPoint, ContextIDExemplar, ContextIDLog, ContextIDProfile:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %v", str)
	}
}

// ContextStatements is a group of OTTL statements and conditions executed together.
type ContextStatements struct {
	// Context is the OTTL context the statements run against. When empty, it is inferred from the statements and conditions.
	Context ContextID `mapstructure:"context"`
	// Conditions are OTTL conditions that must evaluate to true for the statements to run.
	// Multiple conditions are combined with a logical OR.
	Conditions []string `mapstructure:"conditions"`
	// Statements are the OTTL statements to execute.
	Statements []string `mapstructure:"statements"`
	// ErrorMode determines how the processor reacts to errors that occur while processing
	// this group of statements. When provided, it overrides the default Config ErrorMode.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// SharedCache enables sharing per-context caches between all statement groups
	// with this option enabled in the same processor instance.
	SharedCache bool `mapstructure:"shared_cache"`
}

// GetStatements returns the group's OTTL statements.
func (c ContextStatements) GetStatements() []string {
	return c.Statements
}

func (c ContextStatements) toCommon() common.ContextStatements {
	return common.ContextStatements{
		Context:     common.ContextID(c.Context),
		Conditions:  c.Conditions,
		Statements:  c.Statements,
		ErrorMode:   c.ErrorMode,
		SharedCache: c.SharedCache,
	}
}

func toCommonContextStatements(contextStatements []ContextStatements) []common.ContextStatements {
	result := make([]common.ContextStatements, len(contextStatements))
	for i, cs := range contextStatements {
		result[i] = cs.toCommon()
	}
	return result
}

// Unmarshal is used internally by mapstructure to parse the transformprocessor configuration (Config),
// adding support to structured and flat configuration styles.
// When the flat configuration style is used, all statements are grouped into a ContextStatements
// object, with empty [ContextStatements.Context] value.
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
//	  - context: "log"
//	    statements:
//	      - set(attributes["service.new_name"], attributes["service.name"])
//	      - delete_key(attributes, "service.name")
func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if conf == nil {
		return nil
	}

	contextStatementsFields := map[string]*[]ContextStatements{
		"trace_statements":   &c.TraceStatements,
		"metric_statements":  &c.MetricStatements,
		"log_statements":     &c.LogStatements,
		"profile_statements": &c.ProfileStatements,
	}

	contextStatementsPatch := map[string]any{}
	for fieldName := range contextStatementsFields {
		if !conf.IsSet(fieldName) {
			continue
		}
		rawVal := conf.Get(fieldName)
		values, ok := rawVal.([]any)
		if !ok {
			return fmt.Errorf("invalid %s type, expected: array, got: %T", fieldName, rawVal)
		}
		if len(values) == 0 {
			continue
		}

		statementsConfigs := make([]any, 0, len(values))
		var basicStatements []any
		for _, value := range values {
			if value == nil {
				return fmt.Errorf("invalid %s item: empty statement list items are not supported", fieldName)
			}
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

// Validate checks that all statements and conditions parse and that required feature gates are enabled.
func (c *Config) Validate() error {
	var errors error

	if len(c.TraceStatements) > 0 {
		pc, err := common.NewTraceParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithSpanParser(c.spanFunctions), common.WithSpanEventParser(c.spanEventFunctions))
		if err != nil {
			return err
		}
		for _, cs := range c.TraceStatements {
			_, err = pc.ParseContextStatements(cs.toCommon())
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if len(c.MetricStatements) > 0 {
		pc, err := common.NewMetricParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithMetricParser(c.metricFunctions), common.WithDataPointParser(c.dataPointFunctions), common.WithExemplarParser(c.exemplarFunctions))
		if err != nil {
			return err
		}
		for _, cs := range c.MetricStatements {
			_, err := pc.ParseContextStatements(cs.toCommon())
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if len(c.LogStatements) > 0 {
		pc, err := common.NewLogParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithLogParser(c.logFunctions))
		if err != nil {
			return err
		}
		for _, cs := range c.LogStatements {
			_, err = pc.ParseContextStatements(cs.toCommon())
			if err != nil {
				errors = multierr.Append(errors, err)
			}
		}
	}

	if len(c.ProfileStatements) > 0 {
		if !metadata.ProcessorTransformEnableProfilesFeatureGate.IsEnabled() {
			errors = multierr.Append(errors, errProfileStatementsGateDisabled)
		} else {
			pc, err := common.NewProfileParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithProfileParser(c.profileFunctions))
			if err != nil {
				return err
			}
			for _, cs := range c.ProfileStatements {
				_, err = pc.ParseContextStatements(cs.toCommon())
				if err != nil {
					errors = multierr.Append(errors, err)
				}
			}
		}
	}

	if c.Flatten.Log && !metadata.TransformFlattenLogsFeatureGate.IsEnabled() {
		errors = multierr.Append(errors, errFlatLogsGateDisabled)
	}

	return errors
}

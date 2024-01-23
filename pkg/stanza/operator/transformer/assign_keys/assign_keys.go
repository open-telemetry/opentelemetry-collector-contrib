package assign_keys

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
)

const operatorType = "assign_keys"

var assignKeysTransformerFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"filelog.assignKeys",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows usage of `assign_keys` transformer."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30321"),
)

func init() {
	if assignKeysTransformerFeatureGate.IsEnabled() {
		operator.Register(operatorType, func() operator.Builder { return NewConfig() })
	}
}

// NewConfig creates a new assign_keys operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new assign_keys operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a assign_keys operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
	Keys                     []string    `mapstructure:"keys"`
}

// Build will build an assign_keys operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if len(c.Keys) == 0 {
		return nil, fmt.Errorf("assign_keys missing required field keys")
	}

	if e, ok := c.Field.FieldInterface.(entry.BodyField); ok {
		return &Transformer[entry.BodyField]{
			TransformerOperator: transformerOperator,
			Field:               e,
			Keys:                c.Keys,
		}, nil
	}

	if e, ok := c.Field.FieldInterface.(entry.ResourceField); ok {
		return &Transformer[entry.ResourceField]{
			TransformerOperator: transformerOperator,
			Field:               e,
			Keys:                c.Keys,
		}, nil
	}

	if e, ok := c.Field.FieldInterface.(entry.AttributeField); ok {
		return &Transformer[entry.AttributeField]{
			TransformerOperator: transformerOperator,
			Field:               e,
			Keys:                c.Keys,
		}, nil
	}

	return nil, fmt.Errorf("invalid field type: %T", c.Field.FieldInterface)
}

// Transformer transforms a list in the entry field into a map. Each value is assigned a key from configuration keys
type Transformer[T interface {
	entry.BodyField | entry.ResourceField | entry.AttributeField
	entry.FieldInterface
}] struct {
	helper.TransformerOperator
	Field T
	Keys  []string
}

// Process will process an entry with AssignKeys transformation.
func (p *Transformer[T]) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply AssignKeys to an entry
func (p *Transformer[T]) Transform(entry *entry.Entry) error {
	inputListInterface, ok := entry.Delete(p.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply assign_keys: field %s does not exist on entry", p.Field)
	}

	inputList, ok := inputListInterface.([]any)
	if !ok {
		return p.rollbackTransformAndGetError(entry, inputListInterface, fmt.Sprintf("apply assign_keys: couldn't convert field %s to []any", p.Field))
	}
	if len(inputList) != len(p.Keys) {
		return p.rollbackTransformAndGetError(entry, inputListInterface, fmt.Sprintf("apply assign_keys: field %s contains %d values while expected keys are %s contain %d keys", p.Field, len(inputList), p.Keys, len(p.Keys)))
	}

	assignedMap := p.AssignKeys(p.Keys, inputList)

	err := entry.Set(p.Field, assignedMap)
	if err != nil {
		return err
	}
	return nil
}

func (p *Transformer[T]) AssignKeys(keys []string, values []any) map[string]any {
	outputMap := make(map[string]any, len(keys))
	for i, key := range keys {
		outputMap[key] = values[i]
	}

	return outputMap
}

func (p *Transformer[T]) rollbackTransformAndGetError(entry *entry.Entry, originalValue interface{}, rollbackReasonMessage string) error {
	err := entry.Set(p.Field, originalValue)
	if err != nil {
		return errors.Wrap(err, "reset assign_keys field due to: "+rollbackReasonMessage+" has failed")
	}
	return fmt.Errorf(rollbackReasonMessage, p.Field)
}

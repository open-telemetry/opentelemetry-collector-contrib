// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/clientutil"
)

// Settings specifies the processor settings.
type Settings struct {
	// Actions specifies the list of attributes to act on.
	// The set of actions are {INSERT, UPDATE, UPSERT, DELETE, HASH, EXTRACT, CONVERT}.
	// This is a required field.
	Actions []ActionKeyValue `mapstructure:"actions"`
}

// ActionKeyValue specifies the attribute key to act upon.
type ActionKeyValue struct {
	// Key specifies the attribute to act upon.
	// This is a required field.
	Key string `mapstructure:"key"`

	// Value specifies the value to populate for the key.
	// The type of the value is inferred from the configuration.
	Value any `mapstructure:"value"`

	// A regex pattern must be specified for the action EXTRACT.
	// It uses the attribute specified by `key' to extract values from
	// The target keys are inferred based on the names of the matcher groups
	// provided and the names will be inferred based on the values of the
	// matcher group.
	// Note: All subexpressions must have a name.
	// Note: The value type of the source key must be a string. If it isn't,
	// no extraction will occur.
	RegexPattern string `mapstructure:"pattern"`

	// FromAttribute specifies the attribute to use to populate
	// the value. If the attribute doesn't exist, no action is performed.
	FromAttribute string `mapstructure:"from_attribute"`

	// FromContext specifies the context value to use to populate
	// the value. The values would be searched in client.Info.Metadata.
	// If the key doesn't exist, no action is performed.
	// If the key has multiple values the values will be joined with `;` separator.
	FromContext string `mapstructure:"from_context"`

	// ConvertedType specifies the target type of an attribute to be converted
	// If the key doesn't exist, no action is performed.
	// If the value cannot be converted, the original value will be left as-is
	ConvertedType string `mapstructure:"converted_type"`

	// Action specifies the type of action to perform.
	// The set of values are {INSERT, UPDATE, UPSERT, DELETE, HASH}.
	// Both lower case and upper case are supported.
	// INSERT -  Inserts the key/value to attributes when the key does not exist.
	//           No action is applied to attributes where the key already exists.
	//           Either Value, FromAttribute or FromContext must be set.
	// UPDATE -  Updates an existing key with a value. No action is applied
	//           to attributes where the key does not exist.
	//           Either Value, FromAttribute or FromContext must be set.
	// UPSERT -  Performs insert or update action depending on the attributes
	//           containing the key. The key/value is inserted to attributes
	//           that did not originally have the key. The key/value is updated
	//           for attributes where the key already existed.
	//           Either Value, FromAttribute or FromContext must be set.
	// DELETE  - Deletes the attribute. If the key doesn't exist,
	//           no action is performed.
	// HASH    - Calculates the SHA-1 hash of an existing value and overwrites the
	//           value with its SHA-1 hash result. If the feature gate
	//           `coreinternal.attraction.hash.sha256` is enabled, it uses SHA2-256
	//           instead.
	// EXTRACT - Extracts values using a regular expression rule from the input
	//           'key' to target keys specified in the 'rule'. If a target key
	//           already exists, it will be overridden.
	// CONVERT  - converts the type of an existing attribute, if convertable
	// This is a required field.
	Action Action `mapstructure:"action"`
}

func (a *ActionKeyValue) valueSourceCount() int {
	count := 0
	if a.Value != nil {
		count++
	}

	if a.FromAttribute != "" {
		count++
	}

	if a.FromContext != "" {
		count++
	}
	return count
}

// Action is the enum to capture the four types of actions to perform on an
// attribute.
type Action string

const (
	// INSERT adds the key/value to attributes when the key does not exist.
	// No action is applied to attributes where the key already exists.
	INSERT Action = "insert"

	// UPDATE updates an existing key with a value. No action is applied
	// to attributes where the key does not exist.
	UPDATE Action = "update"

	// UPSERT performs the INSERT or UPDATE action. The key/value is
	// inserted to attributes that did not originally have the key. The key/value is
	// updated for attributes where the key already existed.
	UPSERT Action = "upsert"

	// DELETE deletes the attribute. If the key doesn't exist, no action is performed.
	// Supports pattern which is matched against attribute key.
	DELETE Action = "delete"

	// HASH calculates the SHA-256 hash of an existing value and overwrites the
	// value with it's SHA-256 hash result.
	// Supports pattern which is matched against attribute key.
	HASH Action = "hash"

	// EXTRACT extracts values using a regular expression rule from the input
	// 'key' to target keys specified in the 'rule'. If a target key already
	// exists, it will be overridden.
	EXTRACT Action = "extract"

	// CONVERT converts the type of an existing attribute, if convertable
	CONVERT Action = "convert"
)

type attributeAction struct {
	Key           string
	FromAttribute string
	FromContext   string
	ConvertedType string
	// Compiled regex if provided
	Regex *regexp.Regexp
	// Attribute names extracted from the regexp's subexpressions.
	AttrNames []string
	// Number of non empty strings in above array

	// TODO https://go.opentelemetry.io/collector/issues/296
	// Do benchmark testing between having action be of type string vs integer.
	// The reason is attributes processor will most likely be commonly used
	// and could impact performance.
	Action         Action
	AttributeValue *pcommon.Value
}

// AttrProc is an attribute processor.
type AttrProc struct {
	actions []attributeAction
}

// NewAttrProc validates that the input configuration has all of the required fields for the processor
// and returns a AttrProc to be used to process attributes.
// An error is returned if there are any invalid inputs.
func NewAttrProc(settings *Settings) (*AttrProc, error) {
	attributeActions := make([]attributeAction, 0, len(settings.Actions))
	for i, a := range settings.Actions {
		// Convert `action` to lowercase for comparison.
		a.Action = Action(strings.ToLower(string(a.Action)))

		switch a.Action {
		case DELETE, HASH:
			// requires `key` and/or `pattern`
			if a.Key == "" && a.RegexPattern == "" {
				return nil, fmt.Errorf("error creating AttrProc due to missing required field (at least one of \"key\" and \"pattern\" have to be used) at the %d-th actions", i)
			}
		default:
			// `key` is a required field
			if a.Key == "" {
				return nil, fmt.Errorf("error creating AttrProc due to missing required field \"key\" at the %d-th actions", i)
			}
		}

		action := attributeAction{
			Key:    a.Key,
			Action: a.Action,
		}

		valueSourceCount := a.valueSourceCount()

		switch a.Action {
		case INSERT, UPDATE, UPSERT:
			if valueSourceCount == 0 {
				return nil, fmt.Errorf("error creating AttrProc. Either field \"value\", \"from_attribute\" or \"from_context\" setting must be specified for %d-th action", i)
			}

			if valueSourceCount > 1 {
				return nil, fmt.Errorf("error creating AttrProc due to multiple value sources being set at the %d-th actions", i)
			}
			if a.RegexPattern != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use the \"pattern\" field. This must not be specified for %d-th action", a.Action, i)
			}
			if a.ConvertedType != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use the \"converted_type\" field. This must not be specified for %d-th action", a.Action, i)
			}
			// Convert the raw value from the configuration to the internal trace representation of the value.
			if a.Value != nil {
				val := pcommon.NewValueEmpty()
				err := val.FromRaw(a.Value)
				if err != nil {
					return nil, err
				}
				action.AttributeValue = &val
			} else {
				action.FromAttribute = a.FromAttribute
				action.FromContext = a.FromContext
			}
		case HASH, DELETE:
			if a.Value != nil || a.FromAttribute != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use \"value\" or \"from_attribute\" field. These must not be specified for %d-th action", a.Action, i)
			}

			if a.RegexPattern != "" {
				re, err := regexp.Compile(a.RegexPattern)
				if err != nil {
					return nil, fmt.Errorf("error creating AttrProc. Field \"pattern\" has invalid pattern: \"%s\" to be set at the %d-th actions", a.RegexPattern, i)
				}
				action.Regex = re
			}
			if a.ConvertedType != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use the \"converted_type\" field. This must not be specified for %d-th action", a.Action, i)
			}
		case EXTRACT:
			if valueSourceCount > 0 {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use a value source field. These must not be specified for %d-th action", a.Action, i)
			}
			if a.RegexPattern == "" {
				return nil, fmt.Errorf("error creating AttrProc due to missing required field \"pattern\" for action \"%s\" at the %d-th action", a.Action, i)
			}
			if a.ConvertedType != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use the \"converted_type\" field. This must not be specified for %d-th action", a.Action, i)
			}
			re, err := regexp.Compile(a.RegexPattern)
			if err != nil {
				return nil, fmt.Errorf("error creating AttrProc. Field \"pattern\" has invalid pattern: \"%s\" to be set at the %d-th actions", a.RegexPattern, i)
			}
			attrNames := re.SubexpNames()
			if len(attrNames) <= 1 {
				return nil, fmt.Errorf("error creating AttrProc. Field \"pattern\" contains no named matcher groups at the %d-th actions", i)
			}

			for subExpIndex := 1; subExpIndex < len(attrNames); subExpIndex++ {
				if attrNames[subExpIndex] == "" {
					return nil, fmt.Errorf("error creating AttrProc. Field \"pattern\" contains at least one unnamed matcher group at the %d-th actions", i)
				}
			}
			action.Regex = re
			action.AttrNames = attrNames
		case CONVERT:
			if valueSourceCount > 0 || a.RegexPattern != "" {
				return nil, fmt.Errorf("error creating AttrProc. Action \"%s\" does not use value sources or \"pattern\" field. These must not be specified for %d-th action", a.Action, i)
			}
			switch a.ConvertedType {
			case stringConversionTarget:
			case intConversionTarget:
			case doubleConversionTarget:
			case "":
				return nil, fmt.Errorf("error creating AttrProc due to missing required field \"converted_type\" for action \"%s\" at the %d-th action", a.Action, i)
			default:
				return nil, fmt.Errorf("error creating AttrProc due to invalid value \"%s\" in field \"converted_type\" for action \"%s\" at the %d-th action", a.ConvertedType, a.Action, i)
			}
			action.ConvertedType = a.ConvertedType
		default:
			return nil, fmt.Errorf("error creating AttrProc due to unsupported action %q at the %d-th actions", a.Action, i)
		}

		attributeActions = append(attributeActions, action)
	}
	return &AttrProc{actions: attributeActions}, nil
}

// Process applies the AttrProc to an attribute map.
func (ap *AttrProc) Process(ctx context.Context, logger *zap.Logger, attrs pcommon.Map) {
	for _, action := range ap.actions {
		// TODO https://go.opentelemetry.io/collector/issues/296
		// Do benchmark testing between having action be of type string vs integer.
		// The reason is attributes processor will most likely be commonly used
		// and could impact performance.
		switch action.Action {
		case DELETE:
			attrs.Remove(action.Key)

			for _, k := range getMatchingKeys(action.Regex, attrs) {
				attrs.Remove(k)
			}
		case INSERT:
			av, found := getSourceAttributeValue(ctx, action, attrs)
			if !found {
				continue
			}
			if _, found = attrs.Get(action.Key); found {
				continue
			}
			av.CopyTo(attrs.PutEmpty(action.Key))
		case UPDATE:
			av, found := getSourceAttributeValue(ctx, action, attrs)
			if !found {
				continue
			}
			val, found := attrs.Get(action.Key)
			if !found {
				continue
			}
			av.CopyTo(val)
		case UPSERT:
			av, found := getSourceAttributeValue(ctx, action, attrs)
			if !found {
				continue
			}
			val, found := attrs.Get(action.Key)
			if found {
				av.CopyTo(val)
			} else {
				av.CopyTo(attrs.PutEmpty(action.Key))
			}
		case HASH:
			hashAttribute(action.Key, attrs)

			for _, k := range getMatchingKeys(action.Regex, attrs) {
				hashAttribute(k, attrs)
			}
		case EXTRACT:
			extractAttributes(action, attrs)
		case CONVERT:
			convertAttribute(logger, action, attrs)
		}
	}
}

func getAttributeValueFromContext(ctx context.Context, key string) (pcommon.Value, bool) {
	const (
		metadataPrefix   = "metadata."
		authPrefix       = "auth."
		clientAddressKey = "client.address"
	)

	ci := client.FromContext(ctx)
	var vals []string

	switch {
	case key == clientAddressKey:
		vals = []string{clientutil.Address(ci)}
	case strings.HasPrefix(key, metadataPrefix):
		mdKey := strings.TrimPrefix(key, metadataPrefix)
		vals = ci.Metadata.Get(mdKey)
	case strings.HasPrefix(key, authPrefix):
		if ci.Auth == nil {
			return pcommon.Value{}, false
		}

		attrName := strings.TrimPrefix(key, authPrefix)
		attr := ci.Auth.GetAttribute(attrName)

		switch a := attr.(type) {
		case string:
			return pcommon.NewValueStr(a), true
		case []string:
			vals = a
		default:
			// TODO: Warn about unexpected attribute types.
			return pcommon.Value{}, false
		}
	default:
		// Fallback to metadata for backwards compatibility.
		vals = ci.Metadata.Get(key)
	}

	if len(vals) == 0 {
		return pcommon.Value{}, false
	}

	return pcommon.NewValueStr(strings.Join(vals, ";")), true
}

func getSourceAttributeValue(ctx context.Context, action attributeAction, attrs pcommon.Map) (pcommon.Value, bool) {
	// Set the key with a value from the configuration.
	if action.AttributeValue != nil {
		return *action.AttributeValue, true
	}

	if action.FromContext != "" {
		return getAttributeValueFromContext(ctx, action.FromContext)
	}

	return attrs.Get(action.FromAttribute)
}

func hashAttribute(key string, attrs pcommon.Map) {
	if value, exists := attrs.Get(key); exists {
		sha2Hasher(value)
	}
}

func convertAttribute(logger *zap.Logger, action attributeAction, attrs pcommon.Map) {
	if value, exists := attrs.Get(action.Key); exists {
		convertValue(logger, action.Key, action.ConvertedType, value)
	}
}

func extractAttributes(action attributeAction, attrs pcommon.Map) {
	value, found := attrs.Get(action.Key)

	// Extracting values only functions on strings.
	if !found || value.Type() != pcommon.ValueTypeStr {
		return
	}

	// Note: The number of matches will always be equal to number of
	// subexpressions.
	matches := action.Regex.FindStringSubmatch(value.Str())
	if matches == nil {
		return
	}

	// Start from index 1, which is the first submatch (index 0 is the entire
	// match).
	for i := 1; i < len(matches); i++ {
		attrs.PutStr(action.AttrNames[i], matches[i])
	}
}

func getMatchingKeys(regexp *regexp.Regexp, attrs pcommon.Map) []string {
	var keys []string

	if regexp == nil {
		return keys
	}

	for k := range attrs.All() {
		if regexp.MatchString(k) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingUsername = errors.New(`"username" not specified in config`)
	errMissingPassword = errors.New(`"password" not specified in config`)

	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>:<port>`)

	errFieldExtractKeyAmbiguous = errors.New(`exactly one of "key" or "key_regex" must be set`)
)

const defaultEndpoint = "http://localhost:15672"

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	ClientConfig         confighttp.ClientConfig        `mapstructure:",squash"`
	Username             string                         `mapstructure:"username"`
	Password             configopaque.String            `mapstructure:"password"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Queues               QueuesConfig                   `mapstructure:"queues"`
}

// QueuesConfig configures queue-scoped scraping behavior.
type QueuesConfig struct {
	// Extract configures copying queue metadata into resource attributes.
	Extract ExtractConfig `mapstructure:"extract"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ExtractConfig defines rules for pulling arbitrary RabbitMQ metadata into resource attributes.
type ExtractConfig struct {
	// Arguments are rules for copying entries from each queue's `arguments` into resource
	// attributes, using the same rule format as k8sattributesprocessor's `extract`
	// annotations and labels.
	Arguments []FieldExtractConfig `mapstructure:"arguments"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// FieldExtractConfig allows specifying an extraction rule to pull a resource attribute
// out of a RabbitMQ queue's `arguments` map.
type FieldExtractConfig struct {
	// TagName is the name of the resource attribute the extracted value will be stored
	// under. When empty, the argument's own key is used as the attribute name. Extracted
	// attributes never overwrite an attribute already set on the resource.
	TagName string `mapstructure:"tag_name"`
	// Key is an exact queue argument key to extract. Exactly one of Key or KeyRegex must
	// be set.
	Key string `mapstructure:"key"`
	// KeyRegex is a regular expression matched against queue argument keys. Supports
	// backreferences in TagName (e.g. "$1") when KeyRegex contains capture groups.
	KeyRegex string `mapstructure:"key_regex"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err []error
	if cfg.Username == "" {
		err = append(err, errMissingUsername)
	}

	if cfg.Password == "" {
		err = append(err, errMissingPassword)
	}

	_, parseErr := url.Parse(cfg.ClientConfig.Endpoint)
	if parseErr != nil {
		wrappedErr := fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
		err = append(err, wrappedErr)
	}

	for _, rule := range cfg.Queues.Extract.Arguments {
		if (rule.Key == "") == (rule.KeyRegex == "") {
			err = append(err, fmt.Errorf("invalid queues::extract::arguments rule (tag_name: %q): %w", rule.TagName, errFieldExtractKeyAmbiguous))
			continue
		}
		if rule.KeyRegex != "" {
			if _, compileErr := compileKeyRegex(rule.KeyRegex); compileErr != nil {
				err = append(err, fmt.Errorf("invalid queues::extract::arguments key_regex %q: %w", rule.KeyRegex, compileErr))
			}
		}
	}

	return errors.Join(err...)
}

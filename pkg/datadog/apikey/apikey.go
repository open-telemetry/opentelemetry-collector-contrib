// Copyright The OpenTelemetryAuthors
// SPDX-License-Identifier: Apache-2.0

package apikey // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apikey"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
)

const (
	// NonHexChars is a regex of characters that are always invalid in a Datadog API key
	NonHexChars = "[^0-9a-fA-F]"
)

var (
	// ErrAPIKeyFormat is returned if API key contains invalid characters
	ErrAPIKeyFormat = errors.New("api::key contains invalid characters")
	// ErrUnsetAPIKey is returned when the API key is not set.
	ErrUnsetAPIKey = errors.New("api.key is not set")
	// NonHexRegex is a regex of characters that are always invalid in a Datadog API key
	NonHexRegex = regexp.MustCompile(NonHexChars)
)

// StaticAPIKeyCheck does offline validation of API Key, checking for non-empty
// and ensuring all values are valid hex characters.
func StaticAPIKeyCheck(key string) error {
	if key == "" {
		return ErrUnsetAPIKey
	}
	invalidAPIKeyChars := NonHexRegex.FindAllString(key, -1)
	if len(invalidAPIKeyChars) > 0 {
		return fmt.Errorf("%w: invalid characters: %s", ErrAPIKeyFormat, strings.Join(invalidAPIKeyChars, ", "))
	}
	return nil
}

// FullAPIKeyCheck is a helper function that validates the API key online
// and returns an API client as well as an error via a passed channel if it is invalid.
// Note: The Zorkian client portion is deprecated and will be removed in a future version
func FullAPIKeyCheck(
	ctx context.Context,
	apiKey string,
	errchan *chan error,
	buildInfo component.BuildInfo,
	isMetricExportV2Enabled bool,
	failOnInvalidKey bool,
	endpoint string,
	logger *zap.Logger,
	clientConfig confighttp.ClientConfig,
) (apiClient *datadogV2.MetricsApi, err error) {
	if isMetricExportV2Enabled {
		client := clientutil.CreateAPIClient(
			buildInfo,
			endpoint,
			clientConfig,
		)
		go func() { *errchan <- clientutil.ValidateAPIKey(ctx, apiKey, logger, client) }()
		apiClient = datadogV2.NewMetricsApi(client)
	} else {
		zorkianClient := clientutil.CreateZorkianClient(apiKey, endpoint)
		go func() { *errchan <- clientutil.ValidateAPIKeyZorkian(logger, zorkianClient) }()
		apiClient = nil
	}
	// validate apiKey
	if failOnInvalidKey {
		err = <-*errchan
		if err != nil {
			return nil, err
		}
	}
	return apiClient, nil
}

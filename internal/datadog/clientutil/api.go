// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"

import (
	"context"
	"errors"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

// GZipSubmitMetricsOptionalParameters is used to enable gzip compression for metric payloads submitted by native datadog client
var GZipSubmitMetricsOptionalParameters = datadogV2.NewSubmitMetricsOptionalParameters().WithContentEncoding(datadogV2.METRICCONTENTENCODING_GZIP)

// CreateAPIClient creates a new Datadog API client
func CreateAPIClient(buildInfo component.BuildInfo, endpoint string, hcs confighttp.ClientConfig) *datadog.APIClient {
	configuration := datadog.NewConfiguration()
	configuration.UserAgent = UserAgent(buildInfo)
	configuration.HTTPClient = NewHTTPClient(hcs)
	configuration.Compress = true
	configuration.Servers = datadog.ServerConfigurations{
		{
			URL:         "{site}",
			Description: "No description provided",
			Variables:   map[string]datadog.ServerVariable{"site": {DefaultValue: endpoint}},
		},
	}
	return datadog.NewAPIClient(configuration)
}

// ValidateAPIKey checks if the API key (not the APP key) is valid
func ValidateAPIKey(ctx context.Context, apiKey string, logger *zap.Logger, apiClient *datadog.APIClient) error {
	logger.Info("Validating API key.")
	authAPI := datadogV1.NewAuthenticationApi(apiClient)
	resp, httpresp, err := authAPI.Validate(GetRequestContext(ctx, apiKey))
	if err == nil && resp.Valid != nil && *resp.Valid {
		logger.Info("API key validation successful.")
		return nil
	}
	if err != nil {
		if httpresp != nil && httpresp.StatusCode == http.StatusForbidden {
			return WrapError(ErrInvalidAPI, httpresp)
		}
		logger.Warn("Error while validating API key", zap.Error(err))
		return nil
	}
	logger.Warn(ErrInvalidAPI.Error())
	return WrapError(ErrInvalidAPI, httpresp)
}

// GetRequestContext creates a new context with API key for DatadogV2 requests
func GetRequestContext(ctx context.Context, apiKey string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(
		ctx,
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{"apiKeyAuth": {Key: apiKey}},
	)
}

var ErrInvalidAPI = errors.New("API Key validation failed")

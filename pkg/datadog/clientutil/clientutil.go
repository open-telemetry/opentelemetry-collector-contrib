// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/clientutil"

import (
	"context"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/internal/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/scrub"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"
)

var GZipSubmitMetricsOptionalParameters = clientutil.GZipSubmitMetricsOptionalParameters
var ProtobufHeaders = clientutil.ProtobufHeaders

type Retrier = clientutil.Retrier

func NewRetrier(logger *zap.Logger, settings configretry.BackOffConfig, scrubber scrub.Scrubber) *Retrier {
	return clientutil.NewRetrier(logger, settings, scrubber)
}

func GetRequestContext(ctx context.Context, apiKey string) context.Context {
	return clientutil.GetRequestContext(ctx, apiKey)
}

func WrapError(err error, resp *http.Response) error {
	return clientutil.WrapError(err, resp)
}

func NewHTTPClient(hcs confighttp.ClientConfig) *http.Client {
	return clientutil.NewHTTPClient(hcs)
}

func UserAgent(buildInfo component.BuildInfo) string {
	return clientutil.UserAgent(buildInfo)
}

func SetDDHeaders(reqHeader http.Header, buildInfo component.BuildInfo, apiKey string) {
	clientutil.SetDDHeaders(reqHeader, buildInfo, apiKey)
}

func SetExtraHeaders(h http.Header, extras map[string]string) {
	clientutil.SetExtraHeaders(h, extras)
}

func CreateAPIClient(buildInfo component.BuildInfo, endpoint string, hcs confighttp.ClientConfig) *datadog.APIClient {
	return clientutil.CreateAPIClient(buildInfo, endpoint, hcs)
}

func ValidateAPIKey(ctx context.Context, apiKey string, logger *zap.Logger, apiClient *datadog.APIClient) error {
	return clientutil.ValidateAPIKey(ctx, apiKey, logger, apiClient)
}

func CreateZorkianClient(apiKey string, endpoint string) *zorkian.Client {
	return clientutil.CreateZorkianClient(apiKey, endpoint)
}

func ValidateAPIKeyZorkian(logger *zap.Logger, client *zorkian.Client) error {
	return clientutil.ValidateAPIKeyZorkian(logger, client)
}

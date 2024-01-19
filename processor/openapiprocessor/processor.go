// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
)

const (
	urlSchemeAttributeKey          = "url.scheme"
	urlFullAttributeKey            = "url.full"
	urlPathAttributeKey            = "url.path"
	urlQueryAttributeKey           = "url.query"
	serverAddressAttributeKey      = "server.address"
	httpRequestMethodAttributeKey  = "http.request.method"
	httpTargetAttributeKey         = "http.target"
	httpRouteAttributeKey          = "http.route"
	serviceNameAttributeKey        = "service.name"
	openAPIAttributePrefix         = "openapi."
	openAPIOperationIDAttributeKey = "openapi.operation_id"
	openAPIDeprecatedAttributeKey  = "openapi.deprecated"
)

func buildFullTarget(scheme string, host string, target string, query string) string {

	// Check if the target already has a scheme
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}

	// Check if the target already has a host
	if strings.HasPrefix(target, host) {
		return scheme + "://" + target
	}

	fullURL := scheme + "://" + host + target
	if query != "" {
		fullURL += "?" + query
	}

	return fullURL
}

type apiDirectoryResponse struct {
	Items []string `json:"items"`
}

type openAPIProcessor struct {
	ctx                context.Context
	nextConsumer       consumer.Traces
	logger             *zap.Logger
	apiReloadTicker    timeutils.TTicker
	apiReloadInterval  time.Duration
	routers            unsafe.Pointer
	serviceHostMapping unsafe.Pointer
	cfg                Config
}

func newTracesProcessor(ctx context.Context, settings component.TelemetrySettings, nextConsumer consumer.Traces, cfg Config) (processor.Traces, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	reloadDuration, err := time.ParseDuration(cfg.APIReloadInterval)
	if err != nil {
		return nil, err
	}

	oap := &openAPIProcessor{
		ctx:               ctx,
		nextConsumer:      nextConsumer,
		logger:            settings.Logger,
		apiReloadInterval: reloadDuration,
		cfg:               cfg,
	}

	emptyRouters := make(map[string]*Router)
	oap.swapRouters(&emptyRouters)

	emptyServiceHostMapping := make(map[string]string)
	oap.swapServiceHostMapping(&emptyServiceHostMapping)

	oap.loadApis()
	if oap.hasRemoteApis() && oap.apiReloadInterval > 0 {
		oap.apiReloadTicker = &timeutils.PolicyTicker{OnTickFunc: oap.loadApis}
	}

	return oap, nil
}

func (oap *openAPIProcessor) loadServiceHostMapping() *map[string]string {
	return (*map[string]string)(atomic.LoadPointer(&oap.serviceHostMapping))
}

func (oap *openAPIProcessor) swapServiceHostMapping(newServiceHostMapping *map[string]string) {
	atomic.StorePointer(&oap.serviceHostMapping, unsafe.Pointer(newServiceHostMapping))
}

func (oap *openAPIProcessor) loadRouters() *map[string]*Router {
	return (*map[string]*Router)(atomic.LoadPointer(&oap.routers))
}

func (oap *openAPIProcessor) swapRouters(newRouters *map[string]*Router) {
	atomic.StorePointer(&oap.routers, unsafe.Pointer(newRouters))
}

func (oap *openAPIProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (oap *openAPIProcessor) hasRemoteApis() bool {
	return len(oap.cfg.OpenAPIEndpoints) > 0 || len(oap.cfg.OpenAPIDirectories) > 0
}

// Start is invoked during service startup.
func (oap *openAPIProcessor) Start(context.Context, component.Host) error {
	if oap.apiReloadTicker != nil {
		oap.logger.Info("Starting OpenAPI processor reload ticker")
		oap.apiReloadTicker.Start(oap.apiReloadInterval)
	}
	return nil
}

// Shutdown is invoked during service shutdown.
func (oap *openAPIProcessor) Shutdown(context.Context) error {
	if oap.apiReloadTicker != nil {
		oap.logger.Info("Stopping OpenAPI processor reload ticker")
		oap.apiReloadTicker.Stop()
	}
	return nil
}

func (oap *openAPIProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	err := oap.processTraces(ctx, td)
	if err != nil {
		return err
	}
	return oap.nextConsumer.ConsumeTraces(ctx, td)
}

func (oap *openAPIProcessor) getHostFromServiceName(resourceSpan ptrace.ResourceSpans) string {
	serviceName, ok := resourceSpan.Resource().Attributes().Get(serviceNameAttributeKey)
	if !ok {
		return ""
	}

	serviceHostMapping := oap.loadServiceHostMapping()
	return (*serviceHostMapping)[serviceName.AsString()]
}

func (oap *openAPIProcessor) processTraces(_ context.Context, td ptrace.Traces) error {
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resourceSpan := resourceSpans.At(i)
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scope := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scope.Spans().Len(); k++ {
				span := scope.Spans().At(k)

				// Only process client and server spans because they are the only ones that have http attributes
				if span.Kind() != ptrace.SpanKindServer && span.Kind() != ptrace.SpanKindClient {
					continue
				}

				var host string
				// Get the http host attribute
				serverAddressAttr, ok := span.Attributes().Get(serverAddressAttributeKey)
				if !ok {
					host = oap.getHostFromServiceName(resourceSpan)
				} else {
					host = serverAddressAttr.AsString()
					// If host value is IP try serviceName
					if net.ParseIP(host) != nil || host == "" {
						host = oap.getHostFromServiceName(resourceSpan)
					}
				}

				if host == "" {
					continue
				}

				httpRequestMethodAttr, ok := span.Attributes().Get(httpRequestMethodAttributeKey)
				if !ok {
					continue
				}

				var fullTarget string
				if span.Kind() == ptrace.SpanKindServer {

					urlScheme, schemeOk := span.Attributes().Get(urlSchemeAttributeKey)
					if !schemeOk {
						continue
					}

					// Get the http target attribute
					urlPath, pathOk := span.Attributes().Get(urlPathAttributeKey)
					if !pathOk {
						continue
					}

					var query string
					urlQuery, queryOk := span.Attributes().Get(urlQueryAttributeKey)
					if queryOk {
						query = urlQuery.AsString()
					}

					fullTarget = buildFullTarget(urlScheme.AsString(), host, urlPath.AsString(), query)

				} else {
					urlFullAttr, urlFullOk := span.Attributes().Get(urlFullAttributeKey)
					if !urlFullOk {
						continue
					}

					fullTarget = urlFullAttr.AsString()
				}

				// Get the router based on the host
				routers := oap.loadRouters()
				router, ok := (*routers)[host]
				if !ok {
					continue
				}

				// Get the route based on the target
				// Create a dummy http request
				req, err := http.NewRequest(httpRequestMethodAttr.AsString(), fullTarget, nil)
				if err != nil {
					continue
				}

				route, _, err := (*router).FindRoute(req)
				if err != nil {
					continue
				}

				span.Attributes().PutStr(httpRouteAttributeKey, route.Path)

				// Set the operation id attribute
				span.Attributes().PutStr(openAPIOperationIDAttributeKey, route.Operation.OperationID)
				span.Attributes().PutBool(openAPIDeprecatedAttributeKey, route.Operation.Deprecated)

				// Extract and add extensions
				oap.extractAndAddExtensions(span, route)
			}
		}
	}
	return nil
}

func (oap *openAPIProcessor) extractAndAddExtensions(span ptrace.Span, route *routers.Route) {

	if (oap.cfg.Extensions == nil) || (len(oap.cfg.Extensions) == 0) {
		return
	}

	// Get the extensions
	for _, extension := range oap.cfg.Extensions {
		extensionValue, ok := route.Operation.Extensions[extension]
		if ok {
			if str, ok := extensionValue.(string); ok {
				span.Attributes().PutStr(openAPIAttributePrefix+extension, str)
			} else {
				oap.logger.Error("Extension value is not a string", zap.String("extension", extension))
			}
		}
	}
}

func (oap *openAPIProcessor) addRouter(doc *openapi3.T, routers *map[string]*Router, serviceHostMapping *map[string]string) {

	err := addRoutersFromAPI(doc, routers, serviceHostMapping, oap.cfg.AllowHTTPAndHTTPS)
	if err != nil {
		oap.logger.Error("Error creating router", zap.Error(err))
		return
	}
}

func (oap *openAPIProcessor) loadLocalApis(routers *map[string]*Router, serviceHostMapping *map[string]string) {

	if len(oap.cfg.OpenAPISpecs) > 0 {
		for _, spec := range oap.cfg.OpenAPISpecs {
			doc := oap.loadAPIFromData([]byte(spec))
			if doc != nil {
				oap.addRouter(doc, routers, serviceHostMapping)
			}
		}
	}

	if len(oap.cfg.OpenAPIFilePaths) > 0 {
		for _, path := range oap.cfg.OpenAPIFilePaths {
			doc := oap.loadAPIFromFile(path)
			if doc != nil {
				oap.addRouter(doc, routers, serviceHostMapping)
			}
		}
	}
}

func (oap *openAPIProcessor) loadRemoteApis(routers *map[string]*Router, serviceHostMapping *map[string]string) {

	if len(oap.cfg.OpenAPIEndpoints) > 0 {
		for _, endpoint := range oap.cfg.OpenAPIEndpoints {
			doc := oap.loadAPIFromURI(endpoint)
			if doc != nil {
				oap.addRouter(doc, routers, serviceHostMapping)
			}
		}
	}

	if len(oap.cfg.OpenAPIDirectories) > 0 {
		for _, directory := range oap.cfg.OpenAPIDirectories {
			oap.loadAPIDirectory(directory, routers, serviceHostMapping)
		}
	}
}

func (oap *openAPIProcessor) loadApis() {

	routers := make(map[string]*Router)
	serviceHostMapping := make(map[string]string)

	oap.loadRemoteApis(&routers, &serviceHostMapping)
	oap.loadLocalApis(&routers, &serviceHostMapping)

	oap.swapRouters(&routers)
	oap.swapServiceHostMapping(&serviceHostMapping)
}

func (oap *openAPIProcessor) loadAPIDirectory(endpoint string, routers *map[string]*Router, serviceHostMapping *map[string]string) {

	res, err := http.Get(endpoint) //nolint
	if err != nil {
		oap.logger.Error("Error loading OpenAPI directory", zap.Error(err))
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)

	if err != nil {
		oap.logger.Error("Error reading OpenAPI directory response", zap.Error(err))
		return
	}

	var apiDirectory apiDirectoryResponse
	if err := json.Unmarshal(body, &apiDirectory); err != nil {
		oap.logger.Error("Error parsing OpenAPI directory response", zap.Error(err))
		return
	}

	for _, item := range apiDirectory.Items {
		doc := oap.loadAPIFromURI(item)
		if doc != nil {
			oap.addRouter(doc, routers, serviceHostMapping)
		}
	}
}

func (oap *openAPIProcessor) loadAPIFromData(data []byte) *openapi3.T {

	ctx := context.Background()
	loader := &openapi3.Loader{Context: ctx}

	doc, err := loader.LoadFromData(data)

	if err != nil {
		oap.logger.Error("Error loading OpenAPI document", zap.Error(err))
		return nil
	}

	return doc
}

func (oap *openAPIProcessor) loadAPIFromFile(path string) *openapi3.T {

	ctx := context.Background()
	loader := &openapi3.Loader{Context: ctx}

	doc, err := loader.LoadFromFile(path)
	if err != nil {
		oap.logger.Error("Error loading OpenAPI document", zap.Error(err))
		return nil
	}

	return doc
}

func (oap *openAPIProcessor) loadAPIFromURI(endpoint string) *openapi3.T {

	ctx := context.Background()
	loader := &openapi3.Loader{Context: ctx}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		oap.logger.Error("Invalid URL", zap.Error(err))
		return nil
	}

	doc, err := loader.LoadFromURI(parsedURL)
	if err != nil {
		oap.logger.Error("Error loading OpenAPI document", zap.Error(err))
		return nil
	}

	return doc
}

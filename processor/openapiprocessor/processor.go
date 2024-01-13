// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"context"
	"encoding/json"
	"io"
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
	httpSchemeAttribute         = "http.scheme"
	httpHostAttribute           = "http.host"
	httpTargetAttribute         = "http.target"
	httpMethodAttribute         = "http.method"
	httpRouteAttribute          = "http.route"
	openAPIAttributePrefix      = "openapi."
	openAPIOperationIDAttribute = "openapi.operation_id"
	openAPIDeprecatedAttribute  = "openapi.deprecated"
)

func buildFullTarget(scheme string, host string, target string) string {

	// Check if the target already has a scheme
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}

	// Check if the target already has a host
	if strings.HasPrefix(target, host) {
		return scheme + "://" + target
	}

	return scheme + "://" + host + target
}

type apiDirectoryResponse struct {
	Items []string `json:"items"`
}

type openAPIProcessor struct {
	ctx               context.Context
	nextConsumer      consumer.Traces
	logger            *zap.Logger
	apiReloadTicker   timeutils.TTicker
	apiReloadInterval time.Duration
	routers           unsafe.Pointer
	cfg               Config
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

	oap.loadApis()
	if oap.hasRemoteApis() && oap.apiReloadInterval > 0 {
		oap.apiReloadTicker = &timeutils.PolicyTicker{OnTickFunc: oap.loadApis}
	}

	return oap, nil
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
		oap.apiReloadTicker.Start(oap.apiReloadInterval)
	}
	return nil
}

// Shutdown is invoked during service shutdown.
func (oap *openAPIProcessor) Shutdown(context.Context) error {
	if oap.apiReloadTicker != nil {
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

				httpScheme, ok := span.Attributes().Get(httpSchemeAttribute)
				if !ok {
					continue
				}

				// Get the http host attribute
				httpHost, ok := span.Attributes().Get(httpHostAttribute)
				if !ok {
					continue
				}

				// Get the http target attribute
				httpTarget, ok := span.Attributes().Get(httpTargetAttribute)
				if !ok {
					continue
				}

				// Get the http method attribute
				httpMethod, ok := span.Attributes().Get(httpMethodAttribute)
				if !ok {
					continue
				}

				host := httpHost.AsString()
				// Get the router based on the host
				routers := oap.loadRouters()
				router, ok := (*routers)[host]
				if !ok {
					continue
				}

				fullTarget := buildFullTarget(httpScheme.AsString(), host, httpTarget.AsString())

				// Get the route based on the target
				// Create a dummy http request
				req, err := http.NewRequest(httpMethod.AsString(), fullTarget, nil)
				if err != nil {
					continue
				}

				route, _, err := (*router).FindRoute(req)
				if err != nil {
					continue
				}

				span.Attributes().PutStr(httpRouteAttribute, route.Path)

				// Set the operation id attribute
				span.Attributes().PutStr(openAPIOperationIDAttribute, route.Operation.OperationID)
				span.Attributes().PutBool(openAPIDeprecatedAttribute, route.Operation.Deprecated)

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

func (oap *openAPIProcessor) addRouter(doc *openapi3.T, routers *map[string]*Router) {

	// Get list of servers
	servers := doc.Servers
	if len(servers) == 0 {
		oap.logger.Error("No servers found in OpenAPI document")
		return
	}

	err := addRoutersFromAPI(doc, routers, oap.cfg.AllowHTTPAndHTTPS)
	if err != nil {
		oap.logger.Error("Error creating router", zap.Error(err))
		return
	}
}

func (oap *openAPIProcessor) loadLocalApis(routers *map[string]*Router) {

	if len(oap.cfg.OpenAPISpecs) > 0 {
		for _, spec := range oap.cfg.OpenAPISpecs {
			doc := oap.loadAPIFromData([]byte(spec))
			if doc != nil {
				oap.addRouter(doc, routers)
			}
		}
	}

	if len(oap.cfg.OpenAPIFilePaths) > 0 {
		for _, path := range oap.cfg.OpenAPIFilePaths {
			doc := oap.loadAPIFromFile(path)
			if doc != nil {
				oap.addRouter(doc, routers)
			}
		}
	}
}

func (oap *openAPIProcessor) loadRemoteApis(routers *map[string]*Router) {

	if len(oap.cfg.OpenAPIEndpoints) > 0 {
		for _, endpoint := range oap.cfg.OpenAPIEndpoints {
			doc := oap.loadAPIFromURI(endpoint)
			if doc != nil {
				oap.addRouter(doc, routers)
			}
		}
	}

	if len(oap.cfg.OpenAPIDirectories) > 0 {
		for _, directory := range oap.cfg.OpenAPIDirectories {
			oap.loadAPIDirectory(directory, routers)
		}
	}
}

func (oap *openAPIProcessor) loadApis() {

	routers := make(map[string]*Router)

	oap.loadRemoteApis(&routers)
	oap.loadLocalApis(&routers)

	oap.swapRouters(&routers)
}

func (oap *openAPIProcessor) loadAPIDirectory(endpoint string, routers *map[string]*Router) {

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
			oap.addRouter(doc, routers)
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

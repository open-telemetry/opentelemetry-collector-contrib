// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

// Currently only v1.37.0 is supported
const supportedSemconvVersion = "1.37.0"

type setSemconvSpanNameArguments struct {
	SemconvVersion            string
	OriginalSpanNameAttribute ottl.Optional[string]
}

func NewSetSemconvSpanNameFactoryLegacy() ottl.Factory[ottlspan.TransformContext] {
	return ottl.NewFactory("set_semconv_span_name", &setSemconvSpanNameArguments{}, createSetSemconvSpanNameFunctionLegacy)
}

func createSetSemconvSpanNameFunctionLegacy(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlspan.TransformContext], error) {
	args, ok := oArgs.(*setSemconvSpanNameArguments)

	if !ok {
		return nil, errors.New("NewSetSemconvSpanNameFactory args must be of type *setSemconvSpanNameArguments")
	}
	if args.SemconvVersion != supportedSemconvVersion {
		return nil, fmt.Errorf("unsupported semconv version: %s, supported version: %s", args.SemconvVersion, supportedSemconvVersion)
	}

	if !args.OriginalSpanNameAttribute.IsEmpty() && args.OriginalSpanNameAttribute.Get() == "" {
		return nil, errors.New("originalSpanNameAttribute cannot be an empty string")
	}

	return func(_ context.Context, tCtx ottlspan.TransformContext) (any, error) {
		setSemconvSpanName(args.OriginalSpanNameAttribute, tCtx.GetSpan())
		return nil, nil
	}, nil
}

func NewSetSemconvSpanNameFactory() ottl.Factory[*ottlspan.TransformContext] {
	return ottl.NewFactory("set_semconv_span_name", &setSemconvSpanNameArguments{}, createSetSemconvSpanNameFunction)
}

func createSetSemconvSpanNameFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlspan.TransformContext], error) {
	args, ok := oArgs.(*setSemconvSpanNameArguments)

	if !ok {
		return nil, errors.New("NewSetSemconvSpanNameFactory args must be of type *setSemconvSpanNameArguments")
	}
	if args.SemconvVersion != supportedSemconvVersion {
		return nil, fmt.Errorf("unsupported semconv version: %s, supported version: %s", args.SemconvVersion, supportedSemconvVersion)
	}

	if !args.OriginalSpanNameAttribute.IsEmpty() && args.OriginalSpanNameAttribute.Get() == "" {
		return nil, errors.New("originalSpanNameAttribute cannot be an empty string")
	}

	return func(_ context.Context, tCtx *ottlspan.TransformContext) (any, error) {
		setSemconvSpanName(args.OriginalSpanNameAttribute, tCtx.GetSpan())
		return nil, nil
	}, nil
}

func setSemconvSpanName(originalSpanNameAttribute ottl.Optional[string], span ptrace.Span) {
	originalSpanName := span.Name()
	semConvSpanName := deriveSemconvSpanName(span)
	span.SetName(semConvSpanName)
	if originalSpanName != semConvSpanName && !originalSpanNameAttribute.IsEmpty() {
		span.Attributes().PutStr(originalSpanNameAttribute.Get(), originalSpanName)
	}
}

func deriveSemconvSpanName(span ptrace.Span) string {
	switch span.Kind() {
	case ptrace.SpanKindServer:
		spanName := httpSpanName(span, conventions.HTTPRouteKey)
		if spanName != "" {
			return spanName
		}
		spanName = rpcSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindConsumer:
		spanName := messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindClient:
		spanName := httpSpanName(span, conventions.URLTemplateKey)
		if spanName != "" {
			return spanName
		}
		spanName = rpcSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = dbSpanName(span)
		if spanName != "" {
			return spanName
		}
		spanName = messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	case ptrace.SpanKindProducer:
		spanName := messagingSpanName(span)
		if spanName != "" {
			return spanName
		}
	}
	// If no semantic convention defines the span name, default to the original span name
	return span.Name()
}

// https://opentelemetry.io/docs/specs/semconv/http/http-spans/
func httpSpanName(span ptrace.Span, subject attribute.Key) string {
	if method, ok := attributeValue(span, conventions.HTTPRequestMethodKey, "http.method"); ok {
		if subjectVal, ok := span.Attributes().Get(string(subject)); ok {
			return method.AsString() + " " + subjectVal.AsString()
		}
		return method.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/rpc/rpc-spans/
func rpcSpanName(span ptrace.Span) string {
	if system, ok := span.Attributes().Get(string(conventions.RPCSystemKey)); ok {
		method, okMethod := attributeValue(span, conventions.RPCMethodKey, "rpc.grpc.method")
		service, okService := attributeValue(span, conventions.RPCServiceKey, "rpc.grpc.service")

		if okMethod && okService {
			return service.AsString() + "/" + method.AsString()
		}
		if okMethod {
			return method.AsString()
		}
		if okService {
			return service.AsString() + "/*"
		}
		return system.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/database/database-spans/
func dbSpanName(span ptrace.Span) string {
	if system, ok := attributeValue(span, conventions.DBSystemNameKey, "db.system"); ok {
		if summary, ok := span.Attributes().Get(string(conventions.DBQuerySummaryKey)); ok {
			return summary.AsString()
		}
		operationName := ""
		if operation, ok := attributeValue(span, conventions.DBOperationNameKey, "db.operation"); ok {
			operationName = operation.AsString()
		}

		target := databaseTarget(span)

		if operationName != "" && target != "" {
			return operationName + " " + target
		}
		if operationName != "" {
			return operationName
		}
		if target != "" {
			return target
		}
		return system.AsString()
	}
	return ""
}

func databaseTarget(span ptrace.Span) string {
	dbNamespace := ""
	if namespace, ok := attributeValue(span, conventions.DBNamespaceKey, "db.name"); ok {
		dbNamespace = namespace.AsString()
	}

	if collection, ok := span.Attributes().Get(string(conventions.DBCollectionNameKey)); ok {
		if dbNamespace != "" {
			return dbNamespace + "." + collection.AsString()
		}
		return collection.AsString()
	}

	if storedProcedure, ok := span.Attributes().Get(string(conventions.DBStoredProcedureNameKey)); ok {
		if dbNamespace != "" {
			return dbNamespace + "." + storedProcedure.AsString()
		}
		return storedProcedure.AsString()
	}

	if dbNamespace != "" {
		return dbNamespace
	}

	if serverAddress, ok := span.Attributes().Get(string(conventions.ServerAddressKey)); ok {
		if serverPort, ok := span.Attributes().Get(string(conventions.ServerPortKey)); ok {
			return serverAddress.AsString() + ":" + serverPort.AsString()
		}
		return serverAddress.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/#span-name
func messagingSpanName(span ptrace.Span) string {
	if system, ok := span.Attributes().Get(string(conventions.MessagingSystemKey)); ok {
		operation, okOperation := attributeValue(span, conventions.MessagingOperationNameKey, "messaging.operation")
		destination := messagingDestination(span)

		if okOperation && destination != "" {
			return operation.AsString() + " " + destination
		}
		if destination != "" {
			return destination
		}
		if okOperation {
			return operation.AsString()
		}
		return system.AsString()
	}
	return ""
}

// https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/#span-name
func messagingDestination(span ptrace.Span) string {
	if temporaryDestination, ok := span.Attributes().Get(string(conventions.MessagingDestinationTemporaryKey)); ok {
		if temporaryDestination.Bool() {
			return "(temporary)"
		}
	}
	if anonymousDestination, ok := span.Attributes().Get(string(conventions.MessagingDestinationAnonymousKey)); ok {
		// anonymous destinations should also be marked as temporary by the messaging instrumentation
		// double check in case the instrumentation forgot to mark the anonymous destination as temporary
		if anonymousDestination.Bool() {
			return "(anonymous)"
		}
	}
	if destinationTemplate, ok := span.Attributes().Get(string(conventions.MessagingDestinationTemplateKey)); ok {
		return destinationTemplate.AsString()
	}
	if destinationName, ok := attributeValue(span, conventions.MessagingDestinationNameKey, "messaging.destination"); ok {
		return destinationName.AsString()
	}
	if serverAddress, ok := span.Attributes().Get(string(conventions.ServerAddressKey)); ok {
		if serverPort, ok := span.Attributes().Get(string(conventions.ServerPortKey)); ok {
			return serverAddress.AsString() + ":" + serverPort.AsString()
		}
		return serverAddress.AsString()
	}
	return ""
}

func attributeValue(span ptrace.Span, name attribute.Key, alias string) (pcommon.Value, bool) {
	if val, ok := span.Attributes().Get(string(name)); ok {
		return val, true
	}
	if alias != "" {
		if val, ok := span.Attributes().Get(alias); ok {
			return val, true
		}
	}
	return pcommon.Value{}, false
}

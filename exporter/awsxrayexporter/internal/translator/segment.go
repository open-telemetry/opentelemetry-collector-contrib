// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	awsP "github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

// AWS X-Ray acceptable values for origin field.
const (
	OriginEC2        = "AWS::EC2::Instance"
	OriginECS        = "AWS::ECS::Container"
	OriginECSEC2     = "AWS::ECS::EC2"
	OriginECSFargate = "AWS::ECS::Fargate"
	OriginEB         = "AWS::ElasticBeanstalk::Environment"
	OriginEKS        = "AWS::EKS::Container"
	OriginAppRunner  = "AWS::AppRunner::Service"
)

// x-ray only span attributes - https://github.com/open-telemetry/opentelemetry-java-contrib/pull/802
const (
	awsLocalService    = "aws.local.service"
	awsRemoteService   = "aws.remote.service"
	awsLocalOperation  = "aws.local.operation"
	awsRemoteOperation = "aws.remote.operation"
	remoteTarget       = "remoteTarget"
	awsSpanKind        = "aws.span.kind"
	k8sRemoteNamespace = "K8s.RemoteNamespace"
)

var (
	// reInvalidSpanCharacters defines the invalid letters in a span name as per
	// Allowed characters for X-Ray Segment Name:
	// Unicode letters, numbers, and whitespace, and the following symbols: _, ., :, /, %, &, #, =, +, \, -, @
	// Doc: https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
	reInvalidSpanCharacters = regexp.MustCompile(`[^ 0-9\p{L}N_.:/%&#=+\-@]`)
)

var (
	remoteXrayExporterDotConverter = featuregate.GlobalRegistry().MustRegister(
		"exporter.xray.allowDot",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("X-Ray Exporter will no longer convert . to _ in annotation keys when this feature gate is enabled. "),
		featuregate.WithRegisterFromVersion("v0.97.0"),
	)
)

const (
	// defaultMetadataNamespace is used for non-namespaced non-indexed attributes.
	defaultMetadataNamespace = "default"
	// defaultSpanName will be used if there are no valid xray characters in the span name
	defaultSegmentName = "span"
	// maxSegmentNameLength the maximum length of a Segment name
	maxSegmentNameLength = 200
	// rpc.system value for AWS service remotes
	awsAPIRPCSystem = "aws-api"
)

const (
	traceIDLength    = 35 // fixed length of aws trace id
	identifierOffset = 11 // offset of identifier within traceID
)

const (
	localRoot = "LOCAL_ROOT"
)

var removeAnnotationsFromServiceSegment = []string{
	awsRemoteService,
	awsRemoteOperation,
	remoteTarget,
	k8sRemoteNamespace,
}

var (
	writers = newWriterPool(2048)
)

// MakeSegmentDocuments converts spans to json documents
func MakeSegmentDocuments(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) ([]string, error) {
	segments, err := MakeSegmentsFromSpan(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err == nil {
		var documents []string

		for _, v := range segments {
			document, documentErr := MakeDocumentFromSegment(v)
			if documentErr != nil {
				return nil, documentErr
			}

			documents = append(documents, document)
		}

		return documents, nil
	}

	return nil, err
}

func isLocalRootSpanADependencySpan(span ptrace.Span) bool {
	return span.Kind() != ptrace.SpanKindServer &&
		span.Kind() != ptrace.SpanKindInternal
}

// isLocalRoot - we will move to using isRemote once the collector supports deserializing it. Until then, we will rely on aws.span.kind.
func isLocalRoot(span ptrace.Span) bool {
	if myAwsSpanKind, ok := span.Attributes().Get(awsSpanKind); ok {
		return localRoot == myAwsSpanKind.Str()
	}

	return false
}

func addNamespaceToSubsegmentWithRemoteService(span ptrace.Span, segment *awsxray.Segment) {
	if (span.Kind() == ptrace.SpanKindClient ||
		span.Kind() == ptrace.SpanKindConsumer ||
		span.Kind() == ptrace.SpanKindProducer) &&
		segment.Type != nil &&
		segment.Namespace == nil {
		if _, ok := span.Attributes().Get(awsRemoteService); ok {
			segment.Namespace = awsxray.String("remote")
		}
	}
}

func MakeDependencySubsegmentForLocalRootDependencySpan(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool, serviceSegmentID pcommon.SpanID) (*awsxray.Segment, error) {
	var dependencySpan = ptrace.NewSpan()
	span.CopyTo(dependencySpan)

	dependencySpan.SetParentSpanID(serviceSegmentID)

	dependencySubsegment, err := MakeSegment(dependencySpan, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err != nil {
		return nil, err
	}

	// Make this a subsegment
	dependencySubsegment.Type = awsxray.String("subsegment")

	if dependencySubsegment.Namespace == nil {
		dependencySubsegment.Namespace = awsxray.String("remote")
	}

	// Remove span links from consumer spans
	if span.Kind() == ptrace.SpanKindConsumer {
		dependencySubsegment.Links = nil
	}

	if myAwsRemoteService, ok := span.Attributes().Get(awsRemoteService); ok {
		subsegmentName := myAwsRemoteService.Str()
		dependencySubsegment.Name = awsxray.String(trimAwsSdkPrefix(subsegmentName, span))
	}

	return dependencySubsegment, err
}

func MakeServiceSegmentForLocalRootDependencySpan(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool, serviceSegmentID pcommon.SpanID) (*awsxray.Segment, error) {
	// We always create a segment for the service
	var serviceSpan ptrace.Span = ptrace.NewSpan()
	span.CopyTo(serviceSpan)

	// Set the span id to the one internally generated
	serviceSpan.SetSpanID(serviceSegmentID)

	for _, v := range removeAnnotationsFromServiceSegment {
		serviceSpan.Attributes().Remove(v)
	}

	serviceSegment, err := MakeSegment(serviceSpan, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err != nil {
		return nil, err
	}

	// Set the name
	if myAwsLocalService, ok := span.Attributes().Get(awsLocalService); ok {
		serviceSegment.Name = awsxray.String(myAwsLocalService.Str())
	}

	// Remove the HTTP field
	serviceSegment.HTTP = nil

	// Remove AWS subsegment fields
	serviceSegment.AWS.Operation = nil
	serviceSegment.AWS.AccountID = nil
	serviceSegment.AWS.RemoteRegion = nil
	serviceSegment.AWS.RequestID = nil
	serviceSegment.AWS.QueueURL = nil
	serviceSegment.AWS.TableName = nil
	serviceSegment.AWS.TableNames = nil

	// Delete all metadata that does not start with 'otel.resource.'
	for _, metaDataEntry := range serviceSegment.Metadata {
		for key := range metaDataEntry {
			if !strings.HasPrefix(key, "otel.resource.") {
				delete(metaDataEntry, key)
			}
		}
	}

	// Make it a segment
	serviceSegment.Type = nil

	// Remote namespace
	serviceSegment.Namespace = nil

	// Remove span links from non-consumer spans
	if span.Kind() != ptrace.SpanKindConsumer {
		serviceSegment.Links = nil
	}

	return serviceSegment, nil
}

func MakeServiceSegmentForLocalRootSpanWithoutDependency(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) ([]*awsxray.Segment, error) {
	segment, err := MakeSegment(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err != nil {
		return nil, err
	}

	segment.Type = nil
	segment.Namespace = nil

	return []*awsxray.Segment{segment}, err
}

func MakeNonLocalRootSegment(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) ([]*awsxray.Segment, error) {
	segment, err := MakeSegment(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err != nil {
		return nil, err
	}

	addNamespaceToSubsegmentWithRemoteService(span, segment)

	return []*awsxray.Segment{segment}, nil
}

func MakeServiceSegmentAndDependencySubsegment(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) ([]*awsxray.Segment, error) {
	// If it is a local root span and a dependency span, we need to make a segment and subsegment representing the local service and remote service, respectively.
	var serviceSegmentID = newSegmentID()
	var segments []*awsxray.Segment

	// Make Dependency Subsegment
	dependencySubsegment, err := MakeDependencySubsegmentForLocalRootDependencySpan(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation, serviceSegmentID)
	if err != nil {
		return nil, err
	}
	segments = append(segments, dependencySubsegment)

	// Make Service Segment
	serviceSegment, err := MakeServiceSegmentForLocalRootDependencySpan(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation, serviceSegmentID)
	if err != nil {
		return nil, err
	}
	segments = append(segments, serviceSegment)

	return segments, err
}

// MakeSegmentsFromSpan creates one or more segments from a span
func MakeSegmentsFromSpan(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) ([]*awsxray.Segment, error) {
	if !isLocalRoot(span) {
		return MakeNonLocalRootSegment(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)
	}

	if !isLocalRootSpanADependencySpan(span) {
		return MakeServiceSegmentForLocalRootSpanWithoutDependency(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)
	}

	return MakeServiceSegmentAndDependencySubsegment(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)
}

// MakeSegmentDocumentString converts an OpenTelemetry Span to an X-Ray Segment and then serializes to JSON
// MakeSegmentDocumentString will be deprecated in the future
func MakeSegmentDocumentString(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) (string, error) {
	segment, err := MakeSegment(span, resource, indexedAttrs, indexAllAttrs, logGroupNames, skipTimestampValidation)

	if err != nil {
		return "", err
	}

	return MakeDocumentFromSegment(segment)
}

// MakeDocumentFromSegment converts a segment into a JSON document
func MakeDocumentFromSegment(segment *awsxray.Segment) (string, error) {
	w := writers.borrow()
	if err := w.Encode(*segment); err != nil {
		return "", err
	}
	jsonStr := w.String()
	writers.release(w)
	return jsonStr, nil
}

func isAwsSdkSpan(span ptrace.Span) bool {
	attributes := span.Attributes()
	if rpcSystem, ok := attributes.Get(conventions.AttributeRPCSystem); ok {
		return rpcSystem.Str() == awsAPIRPCSystem
	}
	return false
}

// MakeSegment converts an OpenTelemetry Span to an X-Ray Segment
func MakeSegment(span ptrace.Span, resource pcommon.Resource, indexedAttrs []string, indexAllAttrs bool, logGroupNames []string, skipTimestampValidation bool) (*awsxray.Segment, error) {
	var segmentType string

	storeResource := true
	if span.Kind() != ptrace.SpanKindServer &&
		!span.ParentSpanID().IsEmpty() {
		segmentType = "subsegment"
		// We only store the resource information for segments, the local root.
		storeResource = false
	}

	// convert trace id
	traceID, err := convertToAmazonTraceID(span.TraceID(), skipTimestampValidation)
	if err != nil {
		return nil, err
	}

	attributes := span.Attributes()

	var (
		startTime                                          = timestampToFloatSeconds(span.StartTimestamp())
		endTime                                            = timestampToFloatSeconds(span.EndTimestamp())
		httpfiltered, http                                 = makeHTTP(span)
		isError, isFault, isThrottle, causefiltered, cause = makeCause(span, httpfiltered, resource)
		origin                                             = determineAwsOrigin(resource)
		awsfiltered, aws                                   = makeAws(causefiltered, resource, logGroupNames)
		service                                            = makeService(resource)
		sqlfiltered, sql                                   = makeSQL(span, awsfiltered)
		additionalAttrs                                    = addSpecialAttributes(sqlfiltered, indexedAttrs, attributes)
		user, annotations, metadata                        = makeXRayAttributes(additionalAttrs, resource, storeResource, indexedAttrs, indexAllAttrs)
		spanLinks, makeSpanLinkErr                         = makeSpanLinks(span.Links(), skipTimestampValidation)
		name                                               string
		namespace                                          string
	)

	if makeSpanLinkErr != nil {
		return nil, makeSpanLinkErr
	}

	// X-Ray segment names are service names, unlike span names which are methods. Try to find a service name.

	// support x-ray specific service name attributes as segment name if it exists
	if span.Kind() == ptrace.SpanKindServer {
		if localServiceName, ok := attributes.Get(awsLocalService); ok {
			name = localServiceName.Str()
		}
	}

	myAwsSpanKind, _ := span.Attributes().Get(awsSpanKind)
	if span.Kind() == ptrace.SpanKindInternal && myAwsSpanKind.Str() == localRoot {
		if localServiceName, ok := attributes.Get(awsLocalService); ok {
			name = localServiceName.Str()
		}
	}

	if span.Kind() == ptrace.SpanKindClient || span.Kind() == ptrace.SpanKindProducer || span.Kind() == ptrace.SpanKindConsumer {
		if remoteServiceName, ok := attributes.Get(awsRemoteService); ok {
			name = remoteServiceName.Str()
			// only strip the prefix for AWS spans
			name = trimAwsSdkPrefix(name, span)
		}
	}

	// peer.service should always be prioritized for segment names when it set by users and
	// the new x-ray specific service name attributes are not found
	if name == "" {
		if peerService, ok := attributes.Get(conventions.AttributePeerService); ok {
			name = peerService.Str()
		}
	}

	if namespace == "" {
		if isAwsSdkSpan(span) {
			namespace = conventions.AttributeCloudProviderAWS
		}
	}

	if name == "" {
		if awsService, ok := attributes.Get(awsxray.AWSServiceAttribute); ok {
			// Generally spans are named something like "Method" or "Service.Method" but for AWS spans, X-Ray expects spans
			// to be named "Service"
			name = awsService.Str()

			if namespace == "" {
				namespace = conventions.AttributeCloudProviderAWS
			}
		}
	}

	if name == "" {
		if dbInstance, ok := attributes.Get(conventions.AttributeDBName); ok {
			// For database queries, the segment name convention is <db name>@<db host>
			name = dbInstance.Str()
			if dbURL, ok := attributes.Get(conventions.AttributeDBConnectionString); ok {
				// Trim JDBC connection string if starts with "jdbc:", otherwise no change
				// jdbc:mysql://db.dev.example.com:3306
				dbURLStr := strings.TrimPrefix(dbURL.Str(), "jdbc:")
				if parsed, _ := url.Parse(dbURLStr); parsed != nil {
					if parsed.Hostname() != "" {
						name += "@" + parsed.Hostname()
					}
				}
			}
		}
	}

	if name == "" && span.Kind() == ptrace.SpanKindServer {
		// Only for a server span, we can use the resource.
		if service, ok := resource.Attributes().Get(conventions.AttributeServiceName); ok {
			name = service.Str()
		}
	}

	if name == "" {
		if rpcservice, ok := attributes.Get(conventions.AttributeRPCService); ok {
			name = rpcservice.Str()
		}
	}

	if name == "" {
		if host, ok := attributes.Get(conventions.AttributeHTTPHost); ok {
			name = host.Str()
		}
	}

	if name == "" {
		if peer, ok := attributes.Get(conventions.AttributeNetPeerName); ok {
			name = peer.Str()
		}
	}

	if name == "" {
		name = fixSegmentName(span.Name())
	}

	if namespace == "" && span.Kind() == ptrace.SpanKindClient {
		namespace = "remote"
	}

	return &awsxray.Segment{
		ID:          awsxray.String(traceutil.SpanIDToHexOrEmptyString(span.SpanID())),
		TraceID:     awsxray.String(traceID),
		Name:        awsxray.String(name),
		StartTime:   awsP.Float64(startTime),
		EndTime:     awsP.Float64(endTime),
		ParentID:    awsxray.String(traceutil.SpanIDToHexOrEmptyString(span.ParentSpanID())),
		Fault:       awsP.Bool(isFault),
		Error:       awsP.Bool(isError),
		Throttle:    awsP.Bool(isThrottle),
		Cause:       cause,
		Origin:      awsxray.String(origin),
		Namespace:   awsxray.String(namespace),
		User:        awsxray.String(user),
		HTTP:        http,
		AWS:         aws,
		Service:     service,
		SQL:         sql,
		Annotations: annotations,
		Metadata:    metadata,
		Type:        awsxray.String(segmentType),
		Links:       spanLinks,
	}, nil
}

// newSegmentID generates a new valid X-Ray SegmentID
func newSegmentID() pcommon.SpanID {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return pcommon.SpanID(r)
}

func determineAwsOrigin(resource pcommon.Resource) string {
	if resource.Attributes().Len() == 0 {
		return ""
	}

	if provider, ok := resource.Attributes().Get(conventions.AttributeCloudProvider); ok {
		if provider.Str() != conventions.AttributeCloudProviderAWS {
			return ""
		}
	}

	if is, present := resource.Attributes().Get(conventions.AttributeCloudPlatform); present {
		switch is.Str() {
		case conventions.AttributeCloudPlatformAWSAppRunner:
			return OriginAppRunner
		case conventions.AttributeCloudPlatformAWSEKS:
			return OriginEKS
		case conventions.AttributeCloudPlatformAWSElasticBeanstalk:
			return OriginEB
		case conventions.AttributeCloudPlatformAWSECS:
			lt, present := resource.Attributes().Get(conventions.AttributeAWSECSLaunchtype)
			if !present {
				return OriginECS
			}
			switch lt.Str() {
			case conventions.AttributeAWSECSLaunchtypeEC2:
				return OriginECSEC2
			case conventions.AttributeAWSECSLaunchtypeFargate:
				return OriginECSFargate
			default:
				return OriginECS
			}
		case conventions.AttributeCloudPlatformAWSEC2:
			return OriginEC2

		// If cloud_platform is defined with a non-AWS value, we should not assign it an AWS origin
		default:
			return ""
		}
	}

	return ""
}

// convertToAmazonTraceID converts a trace ID to the Amazon format.
//
// A trace ID unique identifier that connects all segments and subsegments
// originating from a single client request.
//   - A trace_id consists of three numbers separated by hyphens. For example,
//     1-58406520-a006649127e371903a2de979. This includes:
//   - The version number, that is, 1.
//   - The time of the original request, in Unix epoch time, in 8 hexadecimal digits.
//   - For example, 10:00AM December 2nd, 2016 PST in epoch time is 1480615200 seconds,
//     or 58406520 in hexadecimal.
//   - A 96-bit identifier for the trace, globally unique, in 24 hexadecimal digits.
func convertToAmazonTraceID(traceID pcommon.TraceID, skipTimestampValidation bool) (string, error) {
	const (
		// maxAge of 28 days.  AWS has a 30 day limit, let's be conservative rather than
		// hit the limit
		maxAge = 60 * 60 * 24 * 28

		// maxSkew allows for 5m of clock skew
		maxSkew = 60 * 5
	)

	var (
		content      = [traceIDLength]byte{}
		epochNow     = time.Now().Unix()
		traceIDBytes = traceID
		epoch        = int64(binary.BigEndian.Uint32(traceIDBytes[0:4]))
		b            = [4]byte{}
	)

	// If feature gate is enabled, skip the timestamp validation logic
	if !skipTimestampValidation {
		// If AWS traceID originally came from AWS, no problem.  However, if oc generated
		// the traceID, then the epoch may be outside the accepted AWS range of within the
		// past 30 days.
		//
		// In that case, we return invalid traceid error
		if delta := epochNow - epoch; delta > maxAge || delta < -maxSkew {
			return "", fmt.Errorf("invalid xray traceid: %s", traceID)
		}
	}

	binary.BigEndian.PutUint32(b[0:4], uint32(epoch))

	content[0] = '1'
	content[1] = '-'
	hex.Encode(content[2:10], b[0:4])
	content[10] = '-'
	hex.Encode(content[identifierOffset:], traceIDBytes[4:16]) // overwrite with identifier

	return string(content[0:traceIDLength]), nil
}

func timestampToFloatSeconds(ts pcommon.Timestamp) float64 {
	return float64(ts) / float64(time.Second)
}

func addSpecialAttributes(attributes map[string]pcommon.Value, indexedAttrs []string, unfilteredAttributes pcommon.Map) map[string]pcommon.Value {
	for _, name := range indexedAttrs {
		// Allow attributes that have been filtered out before if explicitly added to be annotated/indexed
		_, isAnnotatable := attributes[name]
		unfilteredAttribute, attributeExists := unfilteredAttributes.Get(name)
		if !isAnnotatable && attributeExists {
			attributes[name] = unfilteredAttribute
		}
	}

	return attributes
}

func makeXRayAttributes(attributes map[string]pcommon.Value, resource pcommon.Resource, storeResource bool, indexedAttrs []string, indexAllAttrs bool) (
	string, map[string]any, map[string]map[string]any) {
	var (
		annotations = map[string]any{}
		metadata    = map[string]map[string]any{}
		user        string
	)
	userid, ok := attributes[conventions.AttributeEnduserID]
	if ok {
		user = userid.Str()
		delete(attributes, conventions.AttributeEnduserID)
	}

	if len(attributes) == 0 && (!storeResource || resource.Attributes().Len() == 0) {
		return user, nil, nil
	}

	defaultMetadata := map[string]any{}

	indexedKeys := map[string]bool{}
	if !indexAllAttrs {
		for _, name := range indexedAttrs {
			indexedKeys[name] = true
		}
	}

	annotationKeys, ok := attributes[awsxray.AWSXraySegmentAnnotationsAttribute]
	if ok && annotationKeys.Type() == pcommon.ValueTypeSlice {
		slice := annotationKeys.Slice()
		for i := 0; i < slice.Len(); i++ {
			value := slice.At(i)
			if value.Type() != pcommon.ValueTypeStr {
				continue
			}
			key := value.AsString()
			indexedKeys[key] = true
		}
		delete(attributes, awsxray.AWSXraySegmentAnnotationsAttribute)
	}

	if storeResource {
		resource.Attributes().Range(func(key string, value pcommon.Value) bool {
			key = "otel.resource." + key
			annoVal := annotationValue(value)
			indexed := indexAllAttrs || indexedKeys[key]
			if annoVal != nil && indexed {
				key = fixAnnotationKey(key)
				annotations[key] = annoVal
			} else {
				metaVal := value.AsRaw()
				if metaVal != nil {
					defaultMetadata[key] = metaVal
				}
			}
			return true
		})
	}

	if indexAllAttrs {
		for key, value := range attributes {
			key = fixAnnotationKey(key)
			annoVal := annotationValue(value)
			if annoVal != nil {
				annotations[key] = annoVal
			}
		}
	} else {
		for key, value := range attributes {
			switch {
			case indexedKeys[key]:
				key = fixAnnotationKey(key)
				annoVal := annotationValue(value)
				if annoVal != nil {
					annotations[key] = annoVal
				}
			case strings.HasPrefix(key, awsxray.AWSXraySegmentMetadataAttributePrefix) && value.Type() == pcommon.ValueTypeStr:
				namespace := strings.TrimPrefix(key, awsxray.AWSXraySegmentMetadataAttributePrefix)
				var metaVal map[string]any
				err := json.Unmarshal([]byte(value.Str()), &metaVal)
				switch {
				case err != nil:
					// if unable to unmarshal, keep the original key/value
					defaultMetadata[key] = value.Str()
				case strings.EqualFold(namespace, defaultMetadataNamespace):
					for k, v := range metaVal {
						defaultMetadata[k] = v
					}
				default:
					metadata[namespace] = metaVal
				}
			default:
				metaVal := value.AsRaw()
				if metaVal != nil {
					defaultMetadata[key] = metaVal
				}
			}
		}
	}

	if len(defaultMetadata) > 0 {
		metadata[defaultMetadataNamespace] = defaultMetadata
	}

	return user, annotations, metadata
}

func annotationValue(value pcommon.Value) any {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str()
	case pcommon.ValueTypeInt:
		return value.Int()
	case pcommon.ValueTypeDouble:
		return value.Double()
	case pcommon.ValueTypeBool:
		return value.Bool()
	}
	return nil
}

// fixSegmentName removes any invalid characters from the span name.  AWS X-Ray defines
// the list of valid characters here:
// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
func fixSegmentName(name string) string {
	if reInvalidSpanCharacters.MatchString(name) {
		// only allocate for ReplaceAllString if we need to
		name = reInvalidSpanCharacters.ReplaceAllString(name, "")
	}

	if length := len(name); length > maxSegmentNameLength {
		name = name[0:maxSegmentNameLength]
	} else if length == 0 {
		name = defaultSegmentName
	}

	return name
}

// fixAnnotationKey removes any invalid characters from the annotaiton key.  AWS X-Ray defines
// the list of valid characters here:
// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
func fixAnnotationKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case '0' <= r && r <= '9':
			fallthrough
		case 'A' <= r && r <= 'Z':
			fallthrough
		case 'a' <= r && r <= 'z':
			return r
		case remoteXrayExporterDotConverter.IsEnabled() && r == '.':
			return r
		default:
			return '_'
		}
	}, key)
}

func trimAwsSdkPrefix(name string, span ptrace.Span) string {
	if isAwsSdkSpan(span) {
		if strings.HasPrefix(name, "AWS.SDK.") {
			return strings.TrimPrefix(name, "AWS.SDK.")
		} else if strings.HasPrefix(name, "AWS::") {
			return strings.TrimPrefix(name, "AWS::")
		}
	}
	return name
}

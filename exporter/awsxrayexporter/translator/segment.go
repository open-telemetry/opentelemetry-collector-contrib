// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"time"

	awsP "github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

// AWS X-Ray acceptable values for origin field.
const (
	OriginEC2 = "AWS::EC2::Instance"
	OriginECS = "AWS::ECS::Container"
	OriginEB  = "AWS::ElasticBeanstalk::Environment"
	OriginEKS = "AWS::EKS::Container"
)

var (
	zeroSpanID = []byte{0, 0, 0, 0, 0, 0, 0, 0}
)

var (
	// reInvalidSpanCharacters defines the invalid letters in a span name as per
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
	reInvalidSpanCharacters = regexp.MustCompile(`[^ 0-9\p{L}N_.:/%&#=+,\-@]`)
)

const (
	// defaultSpanName will be used if there are no valid xray characters in the span name
	defaultSegmentName = "span"
	// maxSegmentNameLength the maximum length of a Segment name
	maxSegmentNameLength = 200
)

const (
	traceIDLength    = 35 // fixed length of aws trace id
	identifierOffset = 11 // offset of identifier within traceID
)

var (
	writers = newWriterPool(2048)
)

// MakeSegmentDocumentString converts an OpenTelemetry Span to an X-Ray Segment and then serialzies to JSON
func MakeSegmentDocumentString(span pdata.Span, resource pdata.Resource, indexedAttrs []string, indexAllAttrs bool) (string, error) {
	segment := MakeSegment(span, resource, indexedAttrs, indexAllAttrs)
	w := writers.borrow()
	if err := w.Encode(segment); err != nil {
		return "", err
	}
	jsonStr := w.String()
	writers.release(w)
	return jsonStr, nil
}

// MakeSegment converts an OpenTelemetry Span to an X-Ray Segment
func MakeSegment(span pdata.Span, resource pdata.Resource, indexedAttrs []string, indexAllAttrs bool) awsxray.Segment {
	var (
		traceID                                = convertToAmazonTraceID(span.TraceID())
		startTime                              = timestampToFloatSeconds(span.StartTime())
		endTime                                = timestampToFloatSeconds(span.EndTime())
		httpfiltered, http                     = makeHTTP(span)
		isError, isFault, causefiltered, cause = makeCause(span, httpfiltered, resource)
		isThrottled                            = !span.Status().IsNil() && span.Status().Code() == pdata.StatusCodeResourceExhausted
		origin                                 = determineAwsOrigin(resource)
		awsfiltered, aws                       = makeAws(causefiltered, resource)
		service                                = makeService(resource)
		sqlfiltered, sql                       = makeSQL(awsfiltered)
		user, annotations, metadata            = makeXRayAttributes(sqlfiltered, indexedAttrs, indexAllAttrs)
		name                                   string
		namespace                              string
		segmentType                            string
	)

	// X-Ray segment names are service names, unlike span names which are methods. Try to find a service name.

	attributes := span.Attributes()

	// peer.service should always be prioritized for segment names when set because it is what the user decided.
	if peerService, ok := attributes.Get(semconventions.AttributePeerService); ok {
		name = peerService.StringVal()
	}

	if name == "" {
		if awsService, ok := attributes.Get(awsxray.AWSServiceAttribute); ok {
			// Generally spans are named something like "Method" or "Service.Method" but for AWS spans, X-Ray expects spans
			// to be named "Service"
			name = awsService.StringVal()

			namespace = "aws"
		}
	}

	if name == "" {
		if dbInstance, ok := attributes.Get(semconventions.AttributeDBName); ok {
			// For database queries, the segment name convention is <db name>@<db host>
			name = dbInstance.StringVal()
			if dbURL, ok := attributes.Get(semconventions.AttributeDBConnectionString); ok {
				if parsed, _ := url.Parse(dbURL.StringVal()); parsed != nil {
					if parsed.Hostname() != "" {
						name += "@" + parsed.Hostname()
					}
				}
			}
		}
	}

	if name == "" && span.Kind() == pdata.SpanKindSERVER && !resource.IsNil() {
		// Only for a server span, we can use the resource.
		if service, ok := resource.Attributes().Get(semconventions.AttributeServiceName); ok {
			name = service.StringVal()
		}
	}

	if name == "" {
		if rpcservice, ok := attributes.Get(semconventions.AttributeRPCService); ok {
			name = rpcservice.StringVal()
		}
	}

	if name == "" {
		if host, ok := attributes.Get(semconventions.AttributeHTTPHost); ok {
			name = host.StringVal()
		}
	}

	if name == "" {
		if peer, ok := attributes.Get(semconventions.AttributeNetPeerName); ok {
			name = peer.StringVal()
		}
	}

	if name == "" {
		name = fixSegmentName(span.Name())
	}

	if namespace == "" && span.Kind() == pdata.SpanKindCLIENT {
		namespace = "remote"
	}

	if span.Kind() != pdata.SpanKindSERVER {
		segmentType = "subsegment"
	}

	return awsxray.Segment{
		ID:          awsxray.String(convertToAmazonSpanID(span.SpanID().Bytes())),
		TraceID:     awsxray.String(traceID),
		Name:        awsxray.String(name),
		StartTime:   awsP.Float64(startTime),
		EndTime:     awsP.Float64(endTime),
		ParentID:    awsxray.String(convertToAmazonSpanID(span.ParentSpanID().Bytes())),
		Fault:       awsP.Bool(isFault),
		Error:       awsP.Bool(isError),
		Throttle:    awsP.Bool(isThrottled),
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
	}
}

// newTraceID generates a new valid X-Ray TraceID
func newTraceID() pdata.TraceID {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return pdata.NewTraceID(r[:])
}

// newSegmentID generates a new valid X-Ray SegmentID
func newSegmentID() pdata.SpanID {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return pdata.NewSpanID(r[:])
}

func determineAwsOrigin(resource pdata.Resource) string {
	if resource.IsNil() {
		return ""
	}

	if provider, ok := resource.Attributes().Get(semconventions.AttributeCloudProvider); ok {
		if provider.StringVal() != "aws" {
			return ""
		}
	}
	// EKS > EB > ECS > EC2
	_, eks := resource.Attributes().Get(semconventions.AttributeK8sCluster)
	if eks {
		return OriginEKS
	}
	_, eb := resource.Attributes().Get(semconventions.AttributeServiceInstance)
	if eb {
		return OriginEB
	}
	_, ecs := resource.Attributes().Get(semconventions.AttributeContainerName)
	if ecs {
		return OriginECS
	}
	return OriginEC2
}

// convertToAmazonTraceID converts a trace ID to the Amazon format.
//
// A trace ID unique identifier that connects all segments and subsegments
// originating from a single client request.
//  * A trace_id consists of three numbers separated by hyphens. For example,
//    1-58406520-a006649127e371903a2de979. This includes:
//  * The version number, that is, 1.
//  * The time of the original request, in Unix epoch time, in 8 hexadecimal digits.
//  * For example, 10:00AM December 2nd, 2016 PST in epoch time is 1480615200 seconds,
//    or 58406520 in hexadecimal.
//  * A 96-bit identifier for the trace, globally unique, in 24 hexadecimal digits.
func convertToAmazonTraceID(traceID pdata.TraceID) string {
	const (
		// maxAge of 28 days.  AWS has a 30 day limit, let's be conservative rather than
		// hit the limit
		maxAge = 60 * 60 * 24 * 28

		// maxSkew allows for 5m of clock skew
		maxSkew = 60 * 5
	)

	var (
		content  = [traceIDLength]byte{}
		epochNow = time.Now().Unix()
		epoch    = int64(binary.BigEndian.Uint32(traceID.Bytes()[0:4]))
		b        = [4]byte{}
	)

	// If AWS traceID originally came from AWS, no problem.  However, if oc generated
	// the traceID, then the epoch may be outside the accepted AWS range of within the
	// past 30 days.
	//
	// In that case, we use the current time as the epoch and accept that a new span
	// may be created
	if delta := epochNow - epoch; delta > maxAge || delta < -maxSkew {
		epoch = epochNow
	}

	binary.BigEndian.PutUint32(b[0:4], uint32(epoch))

	content[0] = '1'
	content[1] = '-'
	hex.Encode(content[2:10], b[0:4])
	content[10] = '-'
	hex.Encode(content[identifierOffset:], traceID.Bytes()[4:16]) // overwrite with identifier

	return string(content[0:traceIDLength])
}

// convertToAmazonSpanID generates an Amazon spanID from a trace.SpanID - a 64-bit identifier
// for the Segment, unique among segments in the same trace, in 16 hexadecimal digits.
func convertToAmazonSpanID(v []byte) string {
	if v == nil || bytes.Equal(v, zeroSpanID) {
		return ""
	}
	return hex.EncodeToString(v[0:8])
}

func timestampToFloatSeconds(ts pdata.TimestampUnixNano) float64 {
	return float64(ts) / float64(time.Second)
}

func makeXRayAttributes(attributes map[string]string, indexedAttrs []string, indexAllAttrs bool) (
	string, map[string]interface{}, map[string]map[string]interface{}) {
	var (
		annotations = map[string]interface{}{}
		metadata    = map[string]map[string]interface{}{}
		user        string
	)
	delete(attributes, semconventions.AttributeComponent)
	userid, ok := attributes[semconventions.AttributeEnduserID]
	if ok {
		user = userid
		delete(attributes, semconventions.AttributeEnduserID)
	}

	if len(attributes) == 0 {
		return user, nil, nil
	}

	if indexAllAttrs {
		for key, value := range attributes {
			key = fixAnnotationKey(key)
			annotations[key] = value
		}
	} else {
		defaultMetadata := map[string]interface{}{}
		indexedKeys := map[string]interface{}{}
		for _, name := range indexedAttrs {
			indexedKeys[name] = true
		}

		for key, value := range attributes {
			if _, ok := indexedKeys[key]; ok {
				key = fixAnnotationKey(key)
				annotations[key] = value
			} else {
				defaultMetadata[key] = value
			}
		}
		if len(defaultMetadata) > 0 {
			metadata["default"] = defaultMetadata
		}
	}

	return user, annotations, metadata
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
		default:
			return '_'
		}
	}, key)
}

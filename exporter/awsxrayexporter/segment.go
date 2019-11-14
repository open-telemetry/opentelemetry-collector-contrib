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

package awsxrayexporter

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"math/rand"
	"reflect"
	"regexp"
	"sync"
	"time"
)

// origin contains the support aws origin values,
// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
type origin string

const (
	// Attributes recorded on the span for the requests.
	// Only trace exporters will need them.
	ComponentAttribute   = "component"
	HttpComponentType    = "http"
	RpcComponentType     = "rpc"
	DbComponentType      = "db"
	MsgComponentType     = "messaging"
	PeerAddressAttribute = "peer.address"
	PeerHostAttribute    = "peer.hostname"
	PeerIpv4Attribute    = "peer.ipv4"
	PeerIpv6Attribute    = "peer.ipv6"
	PeerPortAttribute    = "peer.port"
	PeerServiceAttribute = "peer.service"
	DbTypeAttribute      = "db.type"
	DbInstanceAttribute  = "db.instance"
	DbStatementAttribute = "db.statement"
	DbUserAttribute      = "db.user"
)

const (
	ServiceNameAttribute      = "service.name"
	ServiceNamespaceAttribute = "service.namespace"
	ServiceInstanceAttribute  = "service.instance.id"
	ServiceVersionAttribute   = "service.version"
	ContainerNameAttribute    = "container.name"
	ContainerImageAttribute   = "container.image.name"
	ContainerTagAttribute     = "container.image.tag"
	K8sClusterAttribute       = "k8s.cluster.name"
	K8sNamespaceAttribute     = "k8s.namespace.name"
	K8sPodAttribute           = "k8s.pod.name"
	K8sDeploymentAttribute    = "k8s.deployment.name"
	HostHostnameAttribute     = "host.hostname"
	HostIdAttribute           = "host.id"
	HostNameAttribute         = "host.name"
	HostTypeAttribute         = "host.type"
	CloudProviderAttribute    = "cloud.provider"
	CloudAccountAttribute     = "cloud.account.id"
	CloudRegionAttribute      = "cloud.region"
	CloudZoneAttribute        = "cloud.zone"
)

const (
	// OriginEC2 span originated from EC2
	OriginEC2 origin = "AWS::EC2::Instance"

	// OriginECS span originated from Elastic Container Service (ECS)
	OriginECS origin = "AWS::ECS::Container"

	// OriginEB span originated from Elastic Beanstalk (EB)
	OriginEB origin = "AWS::ElasticBeanstalk::Environment"
)

const (
	httpHeaderMaxSize = 200
	httpHeader        = `X-Amzn-Trace-Id`
	prefixRoot        = "Root="
	prefixParent      = "Parent="
	prefixSampled     = "Sampled="
	separator         = ";" // separator used by x-ray to split parts of X-Amzn-Trace-Id header
)

var (
	zeroSpanID = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	r          = rand.New(rand.NewSource(time.Now().UnixNano())) // random, not secure
	mutex      = &sync.Mutex{}
)

var (
	// reInvalidSpanCharacters defines the invalid letters in a span name as per
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
	reInvalidSpanCharacters = regexp.MustCompile(`[^ 0-9\p{L}N_.:/%&#=+,\-@]`)
	// reInvalidAnnotationCharacters defines the invalid letters in an annotation key as per
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
	reInvalidAnnotationCharacters = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

const (
	// defaultSpanName will be used if there are no valid xray characters in the
	// span name
	defaultSegmentName = "span"

	// maxSegmentNameLength the maximum length of a segment name
	maxSegmentNameLength = 200
)

const (
	traceIDLength    = 35 // fixed length of aws trace id
	spanIDLength     = 16 // fixed length of aws span id
	epochOffset      = 2  // offset of epoch secs
	identifierOffset = 11 // offset of identifier within traceID
)

type segment struct {
	// ID - A 64-bit identifier for the segment, unique among segments in the same trace,
	// in 16 hexadecimal digits.
	ID string `json:"id"`

	// Name - The logical name of the service that handled the request, up to 200 characters.
	// For example, your application's name or domain name. Names can contain Unicode
	// letters, numbers, and whitespace, and the following symbols: _, ., :, /, %, &, #, =,
	// +, \, -, @
	Name string `json:"name,omitempty"`

	// StartTime - number that is the time the segment was created, in floating point seconds
	// in epoch time.. For example, 1480615200.010 or 1.480615200010E9. Use as many decimal
	// places as you need. Microsecond resolution is recommended when available.
	StartTime float64 `json:"start_time"`

	// TraceID - A unique identifier that connects all segments and subsegments originating
	// from a single client request.
	//	* The version number, that is, 1.
	//	* The time of the original request, in Unix epoch time, in 8 hexadecimal digits.
	//	* For example, 10:00AM December 2nd, 2016 PST in epoch time is 1480615200 seconds, or 58406520 in hexadecimal.
	//	* A 96-bit identifier for the trace, globally unique, in 24 hexadecimal digits.
	TraceID string `json:"trace_id,omitempty"`

	// EndTime - number that is the time the segment was closed. For example, 1480615200.090
	// or 1.480615200090E9. Specify either an end_time or in_progress.
	EndTime float64 `json:"end_time"`

	/* ---------------------------------------------------- */

	// Service - An object with information about your application.
	//Service service `json:"service,omitempty"`

	// User - A string that identifies the user who sent the request.
	//User string `json:"user,omitempty"`

	// Origin - The type of AWS resource running your application.
	Origin string `json:"origin,omitempty"`

	// Namespace - aws for AWS SDK calls; remote for other downstream calls.
	Namespace string `json:"namespace,omitempty"`

	// ParentID – A subsegment ID you specify if the request originated from an instrumented
	// application. The X-Ray SDK adds the parent subsegment ID to the tracing header for
	// downstream HTTP calls.
	ParentID string `json:"parent_id,omitempty"`

	// Annotations - object with key-value pairs that you want X-Ray to index for search
	Annotations map[string]interface{} `json:"annotations,omitempty"`

	// SubSegments contains the list of child segments
	SubSegments []*segment `json:"subsegments,omitempty"`

	// Service - optional service definition
	Service *service `json:"service,omitempty"`

	// Http - optional xray specific http settings
	Http *HTTPData `json:"http,omitempty"`

	// Error - boolean indicating that a client error occurred
	// (response status code was 4XX Client Error).
	Error bool `json:"error,omitempty"`

	// Fault - boolean indicating that a server error occurred
	// (response status code was 5XX Server Error).
	Fault bool `json:"fault,omitempty"`

	// Cause
	Cause *CauseData `json:"cause,omitempty"`

	/* -- Used by SubSegments only ------------------------ */

	// Type indicates span is a subsegment; should either be subsegment or blank
	Type string `json:"type,omitempty"`
}

type service struct {
	// Version - A string that identifies the version of your application that served the request.
	Version string `json:"version,omitempty"`
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
func convertToAmazonTraceID(traceID []byte) string {
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
		epoch    = int64(binary.BigEndian.Uint32(traceID[0:4]))
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
	hex.Encode(content[identifierOffset:], traceID[4:16]) // overwrite with identifier

	return string(content[0:traceIDLength])
}

// convertToAmazonSpanID generates an Amazon spanID from a trace.SpanID - a 64-bit identifier
// for the segment, unique among segments in the same trace, in 16 hexadecimal digits.
func convertToAmazonSpanID(v []byte) string {
	if reflect.DeepEqual(v, zeroSpanID) {
		return ""
	}
	return hex.EncodeToString(v[0:8])
}

func mergeAnnotations(dest map[string]interface{}, src map[string]string) {
	for key, value := range src {
		key = fixAnnotationKey(key)
		dest[key] = value
	}
}

func makeAnnotations(attributes map[string]string) map[string]interface{} {
	var result = map[string]interface{}{}

	mergeAnnotations(result, attributes)

	if len(result) == 0 {
		return nil
	}
	return result
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
	if reInvalidAnnotationCharacters.MatchString(key) {
		// only allocate for ReplaceAllString if we need to
		key = reInvalidAnnotationCharacters.ReplaceAllString(key, "_")
	}

	return key
}

func rawSegment(name string, span *tracepb.Span) segment {
	var (
		traceID                 = convertToAmazonTraceID(span.TraceId)
		startMicros             = span.StartTime.Nanos / int32(time.Microsecond)
		startTime               = float64(startMicros) / 1e6
		endMicros               = span.EndTime.Nanos / int32(time.Microsecond)
		endTime                 = float64(endMicros) / 1e6
		filtered, http          = makeHttp(span.Kind, span.Status.Code, span.Attributes.AttributeMap)
		isError, isFault, cause = makeCause(span.Status, filtered)
		annotations             = makeAnnotations(span.Resource.GetLabels())
		namespace               string
	)

	if name == "" {
		name = fixSegmentName(span.Name.String())
	}
	if span.ParentSpanId != nil {
		namespace = "remote"
	}

	return segment{
		ID:          convertToAmazonSpanID(span.SpanId),
		TraceID:     traceID,
		Name:        name,
		StartTime:   startTime,
		EndTime:     endTime,
		Namespace:   namespace,
		ParentID:    convertToAmazonSpanID(span.ParentSpanId),
		Annotations: annotations,
		Http:        http,
		Error:       isError,
		Fault:       isFault,
		Cause:       cause,
	}
}

type writer struct {
	buffer  *bytes.Buffer
	encoder *json.Encoder
}

func (w *writer) Reset() {
	w.buffer.Reset()
}

func (w *writer) Encode(v interface{}) error {
	return w.encoder.Encode(v)
}

func (w *writer) String() string {
	return w.buffer.String()
}

const (
	maxBufSize = 256e3
)

var (
	writers = &sync.Pool{
		New: func() interface{} {
			var (
				buffer  = bytes.NewBuffer(make([]byte, 0, 8192))
				encoder = json.NewEncoder(buffer)
			)

			return &writer{
				buffer:  buffer,
				encoder: encoder,
			}
		},
	}
)

func borrow() *writer {
	return writers.Get().(*writer)
}

func release(w *writer) {
	if w.buffer.Cap() < maxBufSize {
		w.buffer.Reset()
		writers.Put(w)
	}
}

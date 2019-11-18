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
	"encoding/binary"
	"encoding/hex"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	tracetranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace"
	"math/rand"
	"reflect"
	"regexp"
	"sync"
	"time"
)

const (
	// Attributes recorded on the span for the requests.
	// Only trace exporters will need them.
	ComponentAttribute   = "component"
	HttpComponentType    = "http"
	GrpcComponentType    = "grpc"
	DbComponentType      = "db"
	MsgComponentType     = "messaging"
	PeerAddressAttribute = "peer.address"
	PeerHostAttribute    = "peer.hostname"
	PeerIpv4Attribute    = "peer.ipv4"
	PeerIpv6Attribute    = "peer.ipv6"
	PeerPortAttribute    = "peer.port"
	PeerServiceAttribute = "peer.service"
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
	OriginEC2 = "AWS::EC2::Instance"
	OriginECS = "AWS::ECS::Container"
	OriginEB  = "AWS::ElasticBeanstalk::Environment"
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

	// maxSegmentNameLength the maximum length of a Segment name
	maxSegmentNameLength = 200
)

const (
	traceIDLength    = 35 // fixed length of aws trace id
	spanIDLength     = 16 // fixed length of aws span id
	epochOffset      = 2  // offset of epoch secs
	identifierOffset = 11 // offset of identifier within traceID
)

type Segment struct {
	// Required
	TraceID   string  `json:"trace_id,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time,omitempty"`

	// Optional
	InProgress  bool       `json:"in_progress,omitempty"`
	ParentID    string     `json:"parent_id,omitempty"`
	Fault       bool       `json:"fault,omitempty"`
	Error       bool       `json:"error,omitempty"`
	Throttle    bool       `json:"throttle,omitempty"`
	Cause       *CauseData `json:"cause,omitempty"`
	ResourceARN string     `json:"resource_arn,omitempty"`
	Origin      string     `json:"origin,omitempty"`

	Type         string   `json:"type,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	User         string   `json:"user,omitempty"`
	PrecursorIDs []string `json:"precursor_ids,omitempty"`

	HTTP *HTTPData `json:"http,omitempty"`
	AWS  *AWSData  `json:"aws,omitempty"`

	Service *ServiceData `json:"service,omitempty"`

	// SQL
	SQL *SQLData `json:"sql,omitempty"`

	// Metadata
	Annotations map[string]interface{}            `json:"annotations,omitempty"`
	Metadata    map[string]map[string]interface{} `json:"metadata,omitempty"`
}

func MakeSegment(name string, span *tracepb.Span) Segment {
	var (
		traceID                 = convertToAmazonTraceID(span.TraceId)
		startTime               = timestampToFloatSeconds(span.StartTime, span.StartTime)
		endTime                 = timestampToFloatSeconds(span.EndTime, span.StartTime)
		httpfiltered, http      = makeHttp(span)
		isError, isFault, cause = makeCause(span.Status, httpfiltered)
		isThrottled             = span.Status.Code == tracetranslator.OCResourceExhausted
		origin                  = determineAwsOrigin(span.Resource)
		aws                     = makeAws(span.Resource)
		service                 = makeService(span.Resource)
		sqlfiltered, sql        = makeSql(httpfiltered)
		annotations             = makeAnnotations(sqlfiltered)
		namespace               string
	)

	if name == "" {
		name = fixSegmentName(span.Name.String())
	}
	if span.ParentSpanId != nil {
		namespace = "remote"
	}

	return Segment{
		ID:          convertToAmazonSpanID(span.SpanId),
		TraceID:     traceID,
		Name:        name,
		StartTime:   startTime,
		EndTime:     endTime,
		ParentID:    convertToAmazonSpanID(span.ParentSpanId),
		Fault:       isFault,
		Error:       isError,
		Throttle:    isThrottled,
		Cause:       cause,
		Origin:      origin,
		Namespace:   namespace,
		HTTP:        http,
		AWS:         aws,
		Service:     service,
		SQL:         sql,
		Annotations: annotations,
		Metadata:    nil,
	}
}

func determineAwsOrigin(resource *resourcepb.Resource) string {
	origin := OriginEC2
	if resource == nil {
		return origin
	}
	_, ok := resource.Labels[ContainerNameAttribute]
	if ok {
		origin = OriginECS
	}
	return origin
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
// for the Segment, unique among segments in the same trace, in 16 hexadecimal digits.
func convertToAmazonSpanID(v []byte) string {
	if v == nil || reflect.DeepEqual(v, zeroSpanID) {
		return ""
	}
	return hex.EncodeToString(v[0:8])
}

func timestampToFloatSeconds(ts *timestamp.Timestamp, startTs *timestamp.Timestamp) float64 {
	var (
		t time.Time
	)
	if ts == nil {
		t = time.Now()
	} else if startTs == nil {
		t = time.Unix(ts.Seconds, int64(ts.Nanos))
	} else {
		t = time.Unix(startTs.Seconds, int64(ts.Nanos))
	}
	return float64(t.UnixNano()) / 1e9
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

func newTraceID() []byte {
	var r [16]byte
	epoch := time.Now().Unix()
	binary.BigEndian.PutUint32(r[0:4], uint32(epoch))
	_, err := rand.Read(r[4:])
	if err != nil {
		panic(err)
	}
	return r[:]
}

func newSegmentID() []byte {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		panic(err)
	}
	return r[:]
}

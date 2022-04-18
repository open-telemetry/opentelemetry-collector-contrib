// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"fmt"
	"io"
	"time"

	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var statusCodeMap = map[PICTInputStatus]ptrace.StatusCode{
	SpanStatusUnset: ptrace.StatusCodeUnset,
	SpanStatusOk:    ptrace.StatusCodeOk,
	SpanStatusError: ptrace.StatusCodeError,
}

var statusMsgMap = map[PICTInputStatus]string{
	SpanStatusUnset: "Unset",
	SpanStatusOk:    "Ok",
	SpanStatusError: "Error",
}

// appendSpans appends to the ptrace.SpanSlice objects the number of spans specified by the count input
// parameter. The random parameter injects the random number generator to use in generating IDs and other random values.
// Using a random number generator with the same seed value enables reproducible tests.
//
// If err is not nil, the spans slice will have nil values.
func appendSpans(count int, pictFile string, random io.Reader, spanList ptrace.SpanSlice) error {
	pairsData, err := loadPictOutputFile(pictFile)
	if err != nil {
		return err
	}
	pairsTotal := len(pairsData)
	index := 1
	var inputs []string
	var spanInputs *PICTSpanInputs
	var traceID pcommon.TraceID
	var parentID pcommon.SpanID
	for i := 0; i < count; i++ {
		if index >= pairsTotal {
			index = 1
		}
		inputs = pairsData[index]
		spanInputs = &PICTSpanInputs{
			Parent:     PICTInputParent(inputs[SpansColumnParent]),
			Tracestate: PICTInputTracestate(inputs[SpansColumnTracestate]),
			Kind:       PICTInputKind(inputs[SpansColumnKind]),
			Attributes: PICTInputAttributes(inputs[SpansColumnAttributes]),
			Events:     PICTInputSpanChild(inputs[SpansColumnEvents]),
			Links:      PICTInputSpanChild(inputs[SpansColumnLinks]),
			Status:     PICTInputStatus(inputs[SpansColumnStatus]),
		}
		switch spanInputs.Parent {
		case SpanParentRoot:
			traceID = generateTraceID(random)
			parentID = pcommon.NewSpanID([8]byte{})
		case SpanParentChild:
			// use existing if available
			if traceID.IsEmpty() {
				traceID = generateTraceID(random)
			}
			if parentID.IsEmpty() {
				parentID = generateSpanID(random)
			}
		}
		spanName := generateSpanName(spanInputs)
		fillSpan(traceID, parentID, spanName, spanInputs, random, spanList.AppendEmpty())
		index++
	}
	return nil
}

func generateSpanName(spanInputs *PICTSpanInputs) string {
	return fmt.Sprintf("/%s/%s/%s/%s/%s/%s/%s", spanInputs.Parent, spanInputs.Tracestate, spanInputs.Kind,
		spanInputs.Attributes, spanInputs.Events, spanInputs.Links, spanInputs.Status)
}

// fillSpan generates a single ptrace.Span based on the input values provided. They are:
//   traceID - the trace ID to use, should not be nil
//   parentID - the parent span ID or nil if it is a root span
//   spanName - the span name, should not be blank
//   spanInputs - the pairwise combination of field value variations for this span
//   random - the random number generator to use in generating ID values
//
// The generated span is returned.
func fillSpan(traceID pcommon.TraceID, parentID pcommon.SpanID, spanName string, spanInputs *PICTSpanInputs, random io.Reader, span ptrace.Span) {
	endTime := time.Now().Add(-50 * time.Microsecond)
	span.SetTraceID(traceID)
	span.SetSpanID(generateSpanID(random))
	span.SetTraceState(generateTraceState(spanInputs.Tracestate))
	span.SetParentSpanID(parentID)
	span.SetName(spanName)
	span.SetKind(lookupSpanKind(spanInputs.Kind))
	span.SetStartTimestamp(pcommon.Timestamp(endTime.Add(-215 * time.Millisecond).UnixNano()))
	span.SetEndTimestamp(pcommon.Timestamp(endTime.UnixNano()))
	appendSpanAttributes(spanInputs.Attributes, spanInputs.Status, span.Attributes())
	span.SetDroppedAttributesCount(0)
	appendSpanEvents(spanInputs.Events, span.Events())
	span.SetDroppedEventsCount(0)
	appendSpanLinks(spanInputs.Links, random, span.Links())
	span.SetDroppedLinksCount(0)
	fillStatus(spanInputs.Status, span.Status())
}

func generateTraceState(tracestate PICTInputTracestate) ptrace.TraceState {
	switch tracestate {
	case TraceStateOne:
		return "lasterror=f39cd56cc44274fd5abd07ef1164246d10ce2955"
	case TraceStateFour:
		return "err@ck=80ee5638,rate@ck=1.62,rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"
	case TraceStateEmpty:
		fallthrough
	default:
		return ""
	}
}

func lookupSpanKind(kind PICTInputKind) ptrace.SpanKind {
	switch kind {
	case SpanKindClient:
		return ptrace.SpanKindClient
	case SpanKindServer:
		return ptrace.SpanKindServer
	case SpanKindProducer:
		return ptrace.SpanKindProducer
	case SpanKindConsumer:
		return ptrace.SpanKindConsumer
	case SpanKindInternal:
		return ptrace.SpanKindInternal
	case SpanKindUnspecified:
		fallthrough
	default:
		return ptrace.SpanKindUnspecified
	}
}

func appendSpanAttributes(spanTypeID PICTInputAttributes, statusStr PICTInputStatus, attrMap pcommon.Map) {
	includeStatus := statusStr != SpanStatusUnset
	switch spanTypeID {
	case SpanAttrEmpty:
		return
	case SpanAttrDatabaseSQL:
		appendDatabaseSQLAttributes(attrMap)
	case SpanAttrDatabaseNoSQL:
		appendDatabaseNoSQLAttributes(attrMap)
	case SpanAttrFaaSDatasource:
		appendFaaSDatasourceAttributes(attrMap)
	case SpanAttrFaaSHTTP:
		appendFaaSHTTPAttributes(includeStatus, attrMap)
	case SpanAttrFaaSPubSub:
		appendFaaSPubSubAttributes(attrMap)
	case SpanAttrFaaSTimer:
		appendFaaSTimerAttributes(attrMap)
	case SpanAttrFaaSOther:
		appendFaaSOtherAttributes(attrMap)
	case SpanAttrHTTPClient:
		appendHTTPClientAttributes(includeStatus, attrMap)
	case SpanAttrHTTPServer:
		appendHTTPServerAttributes(includeStatus, attrMap)
	case SpanAttrMessagingProducer:
		appendMessagingProducerAttributes(attrMap)
	case SpanAttrMessagingConsumer:
		appendMessagingConsumerAttributes(attrMap)
	case SpanAttrGRPCClient:
		appendGRPCClientAttributes(attrMap)
	case SpanAttrGRPCServer:
		appendGRPCServerAttributes(attrMap)
	case SpanAttrInternal:
		appendInternalAttributes(attrMap)
	case SpanAttrMaxCount:
		appendMaxCountAttributes(includeStatus, attrMap)
	default:
		appendGRPCClientAttributes(attrMap)
	}
}

func fillStatus(statusStr PICTInputStatus, spanStatus ptrace.SpanStatus) {
	if statusStr == SpanStatusUnset {
		return
	}
	spanStatus.SetCode(statusCodeMap[statusStr])
	spanStatus.SetMessage(statusMsgMap[statusStr])
}

func appendDatabaseSQLAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeDBSystem, "mysql")
	attrMap.UpsertString(conventions.AttributeDBConnectionString, "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;")
	attrMap.UpsertString(conventions.AttributeDBUser, "billing_user")
	attrMap.UpsertString(conventions.AttributeNetHostIP, "192.0.3.122")
	attrMap.UpsertInt(conventions.AttributeNetHostPort, 51306)
	attrMap.UpsertString(conventions.AttributeNetPeerName, "shopdb.example.com")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "192.0.2.12")
	attrMap.UpsertInt(conventions.AttributeNetPeerPort, 3306)
	attrMap.UpsertString(conventions.AttributeNetTransport, "IP.TCP")
	attrMap.UpsertString(conventions.AttributeDBName, "shopdb")
	attrMap.UpsertString(conventions.AttributeDBStatement, "SELECT * FROM orders WHERE order_id = 'o4711'")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendDatabaseNoSQLAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeDBSystem, "mongodb")
	attrMap.UpsertString(conventions.AttributeDBUser, "the_user")
	attrMap.UpsertString(conventions.AttributeNetPeerName, "mongodb0.example.com")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "192.0.2.14")
	attrMap.UpsertInt(conventions.AttributeNetPeerPort, 27017)
	attrMap.UpsertString(conventions.AttributeNetTransport, "IP.TCP")
	attrMap.UpsertString(conventions.AttributeDBName, "shopDb")
	attrMap.UpsertString(conventions.AttributeDBOperation, "findAndModify")
	attrMap.UpsertString(conventions.AttributeDBMongoDBCollection, "products")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSDatasourceAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerDatasource)
	attrMap.UpsertString(conventions.AttributeFaaSExecution, "DB85AF51-5E13-473D-8454-1E2D59415EAB")
	attrMap.UpsertString(conventions.AttributeFaaSDocumentCollection, "faa-flight-delay-information-incoming")
	attrMap.UpsertString(conventions.AttributeFaaSDocumentOperation, "insert")
	attrMap.UpsertString(conventions.AttributeFaaSDocumentTime, "2020-05-09T19:50:06Z")
	attrMap.UpsertString(conventions.AttributeFaaSDocumentName, "delays-20200509-13.csv")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSHTTPAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerHTTP)
	attrMap.UpsertString(conventions.AttributeHTTPMethod, "POST")
	attrMap.UpsertString(conventions.AttributeHTTPScheme, "https")
	attrMap.UpsertString(conventions.AttributeHTTPHost, "api.opentelemetry.io")
	attrMap.UpsertString(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.UpsertString(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.UpsertInt(conventions.AttributeHTTPStatusCode, 201)
	}
	attrMap.UpsertString(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1 Safari/605.1.15")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSPubSubAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerPubsub)
	attrMap.UpsertString(conventions.AttributeMessagingSystem, "sqs")
	attrMap.UpsertString(conventions.AttributeMessagingDestination, "video-views-au")
	attrMap.UpsertString(conventions.AttributeMessagingOperation, "process")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSTimerAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerTimer)
	attrMap.UpsertString(conventions.AttributeFaaSExecution, "73103A4C-E22F-4493-BDE8-EAE5CAB37B50")
	attrMap.UpsertString(conventions.AttributeFaaSTime, "2020-05-09T20:00:08Z")
	attrMap.UpsertString(conventions.AttributeFaaSCron, "0/15 * * * *")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSOtherAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerOther)
	attrMap.UpsertInt("processed.count", 256)
	attrMap.UpsertDouble("processed.data", 14.46)
	attrMap.UpsertBool("processed.errors", false)
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendHTTPClientAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeHTTPMethod, "GET")
	attrMap.UpsertString(conventions.AttributeHTTPURL, "https://opentelemetry.io/registry/")
	if includeStatus {
		attrMap.UpsertInt(conventions.AttributeHTTPStatusCode, 200)
		attrMap.UpsertString("http.status_text", "More Than OK")
	}
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendHTTPServerAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeHTTPMethod, "POST")
	attrMap.UpsertString(conventions.AttributeHTTPScheme, "https")
	attrMap.UpsertString(conventions.AttributeHTTPServerName, "api22.opentelemetry.io")
	attrMap.UpsertInt(conventions.AttributeNetHostPort, 443)
	attrMap.UpsertString(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.UpsertString(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.UpsertInt(conventions.AttributeHTTPStatusCode, 201)
	}
	attrMap.UpsertString(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	attrMap.UpsertString(conventions.AttributeHTTPRoute, "/blog/posts")
	attrMap.UpsertString(conventions.AttributeHTTPClientIP, "2001:506:71f0:16e::1")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendMessagingProducerAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeMessagingSystem, "nats")
	attrMap.UpsertString(conventions.AttributeMessagingDestination, "time.us.east.atlanta")
	attrMap.UpsertString(conventions.AttributeMessagingDestinationKind, "topic")
	attrMap.UpsertString(conventions.AttributeMessagingMessageID, "AA7C5438-D93A-43C8-9961-55613204648F")
	attrMap.UpsertInt("messaging.sequence", 1)
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "10.10.212.33")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendMessagingConsumerAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeMessagingSystem, "kafka")
	attrMap.UpsertString(conventions.AttributeMessagingDestination, "infrastructure-events-zone1")
	attrMap.UpsertString(conventions.AttributeMessagingOperation, "receive")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:4de0:c223:a800:4e87")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendGRPCClientAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeRPCService, "PullRequestsService")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:4de0:c223:a800:4e87")
	attrMap.UpsertInt(conventions.AttributeNetHostPort, 8443)
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendGRPCServerAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeRPCService, "PullRequestsService")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "192.168.1.70")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendInternalAttributes(attrMap pcommon.Map) {
	attrMap.UpsertString("parameters", "account=7310,amount=1817.10")
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
}

func appendMaxCountAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.UpsertString(conventions.AttributeHTTPMethod, "POST")
	attrMap.UpsertString(conventions.AttributeHTTPScheme, "https")
	attrMap.UpsertString(conventions.AttributeHTTPHost, "api.opentelemetry.io")
	attrMap.UpsertString(conventions.AttributeNetHostName, "api22.opentelemetry.io")
	attrMap.UpsertString(conventions.AttributeNetHostIP, "2600:1700:1f00:11c0:1ced:afa5:fd88:9d48")
	attrMap.UpsertInt(conventions.AttributeNetHostPort, 443)
	attrMap.UpsertString(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.UpsertString(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.UpsertInt(conventions.AttributeHTTPStatusCode, 201)
		attrMap.UpsertString("http.status_text", "Created")
	}
	attrMap.UpsertString(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	attrMap.UpsertString(conventions.AttributeHTTPRoute, "/blog/posts")
	attrMap.UpsertString(conventions.AttributeHTTPClientIP, "2600:1700:1f00:11c0:1ced:afa5:fd77:9d01")
	attrMap.UpsertString(conventions.AttributePeerService, "IdentifyImageService")
	attrMap.UpsertString(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:1ced:afa5:fd77:9ddc")
	attrMap.UpsertInt(conventions.AttributeNetPeerPort, 39111)
	attrMap.UpsertDouble("ai-sampler.weight", 0.07)
	attrMap.UpsertBool("ai-sampler.absolute", false)
	attrMap.UpsertInt("ai-sampler.maxhops", 6)
	attrMap.UpsertString("application.create.location", "https://api.opentelemetry.io/blog/posts/806673B9-4F4D-4284-9635-3A3E3E3805BE")
	stages := pcommon.NewValueSlice()
	stages.SliceVal().AppendEmpty().SetStringVal("Launch")
	stages.SliceVal().AppendEmpty().SetStringVal("Injestion")
	stages.SliceVal().AppendEmpty().SetStringVal("Validation")
	attrMap.Upsert("application.stages", stages)
	subMap := pcommon.NewValueMap()
	subMap.MapVal().InsertBool("UIx", false)
	subMap.MapVal().InsertBool("UI4", true)
	subMap.MapVal().InsertBool("flow-alt3", false)
	attrMap.Upsert("application.abflags", subMap)
	attrMap.UpsertString("application.thread", "proc-pool-14")
	attrMap.UpsertString("application.session", "")
	attrMap.UpsertInt("application.persist.size", 1172184)
	attrMap.UpsertInt("application.queue.size", 0)
	attrMap.UpsertString("application.job.id", "0E38800B-9C4C-484E-8F2B-C7864D854321")
	attrMap.UpsertDouble("application.service.sla", 0.34)
	attrMap.UpsertDouble("application.service.slo", 0.55)
	attrMap.UpsertString(conventions.AttributeEnduserID, "unittest")
	attrMap.UpsertString(conventions.AttributeEnduserRole, "poweruser")
	attrMap.UpsertString(conventions.AttributeEnduserScope, "email profile administrator")
}

func appendSpanEvents(eventCnt PICTInputSpanChild, spanEvents ptrace.SpanEventSlice) {
	listSize := calculateListSize(eventCnt)
	for i := 0; i < listSize; i++ {
		appendSpanEvent(i, spanEvents)
	}
}

func appendSpanLinks(linkCnt PICTInputSpanChild, random io.Reader, spanLinks ptrace.SpanLinkSlice) {
	listSize := calculateListSize(linkCnt)
	for i := 0; i < listSize; i++ {
		appendSpanLink(random, i, spanLinks)
	}
}

func calculateListSize(listCnt PICTInputSpanChild) int {
	switch listCnt {
	case SpanChildCountOne:
		return 1
	case SpanChildCountTwo:
		return 2
	case SpanChildCountEight:
		return 8
	case SpanChildCountEmpty:
		fallthrough
	default:
		return 0
	}
}

func appendSpanEvent(index int, spanEvents ptrace.SpanEventSlice) {
	spanEvent := spanEvents.AppendEmpty()
	t := time.Now().Add(-75 * time.Microsecond)
	spanEvent.SetTimestamp(pcommon.Timestamp(t.UnixNano()))
	switch index % 4 {
	case 0, 3:
		spanEvent.SetName("message")
		attrMap := spanEvent.Attributes()
		if index%2 == 0 {
			attrMap.UpsertString("message.type", "SENT")
		} else {
			attrMap.UpsertString("message.type", "RECEIVED")
		}
		attrMap.UpsertInt(conventions.AttributeMessagingMessageID, int64(index/4))
		attrMap.UpsertInt(conventions.AttributeMessagingMessagePayloadCompressedSizeBytes, int64(17*index))
		attrMap.UpsertInt(conventions.AttributeMessagingMessagePayloadSizeBytes, int64(24*index))
	case 1:
		spanEvent.SetName("custom")
		attrMap := spanEvent.Attributes()
		attrMap.UpdateBool("app.inretry", true)
		attrMap.UpsertDouble("app.progress", 0.6)
		attrMap.UpsertString("app.statemap", "14|5|202")
	default:
		spanEvent.SetName("annotation")
	}

	spanEvent.SetDroppedAttributesCount(0)
}

func appendSpanLink(random io.Reader, index int, spanLinks ptrace.SpanLinkSlice) {
	spanLink := spanLinks.AppendEmpty()
	spanLink.SetTraceID(generateTraceID(random))
	spanLink.SetSpanID(generateSpanID(random))
	spanLink.SetTraceState("")
	if index%4 != 2 {
		attrMap := spanLink.Attributes()
		appendMessagingConsumerAttributes(attrMap)
		if index%4 == 1 {
			attrMap.UpdateBool("app.inretry", true)
			attrMap.UpsertDouble("app.progress", 0.6)
			attrMap.UpsertString("app.statemap", "14|5|202")
		}
	}
	spanLink.SetDroppedAttributesCount(0)
}

func generateTraceID(random io.Reader) pcommon.TraceID {
	var r [16]byte
	_, err := random.Read(r[:])
	if err != nil {
		panic(err)
	}
	return pcommon.NewTraceID(r)
}

func generateSpanID(random io.Reader) pcommon.SpanID {
	var r [8]byte
	_, err := random.Read(r[:])
	if err != nil {
		panic(err)
	}
	return pcommon.NewSpanID(r)
}

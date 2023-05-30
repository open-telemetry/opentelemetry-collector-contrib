// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
			parentID = pcommon.SpanID([8]byte{})
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
//
//	traceID - the trace ID to use, should not be nil
//	parentID - the parent span ID or nil if it is a root span
//	spanName - the span name, should not be blank
//	spanInputs - the pairwise combination of field value variations for this span
//	random - the random number generator to use in generating ID values
//
// The generated span is returned.
func fillSpan(traceID pcommon.TraceID, parentID pcommon.SpanID, spanName string, spanInputs *PICTSpanInputs, random io.Reader, span ptrace.Span) {
	endTime := time.Now().Add(-50 * time.Microsecond)
	span.SetTraceID(traceID)
	span.SetSpanID(generateSpanID(random))
	span.TraceState().FromRaw(generateTraceState(spanInputs.Tracestate))
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

func generateTraceState(tracestate PICTInputTracestate) string {
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

func fillStatus(statusStr PICTInputStatus, spanStatus ptrace.Status) {
	if statusStr == SpanStatusUnset {
		return
	}
	spanStatus.SetCode(statusCodeMap[statusStr])
	spanStatus.SetMessage(statusMsgMap[statusStr])
}

func appendDatabaseSQLAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeDBSystem, "mysql")
	attrMap.PutStr(conventions.AttributeDBConnectionString, "Server=shopdb.example.com;Database=ShopDb;Uid=billing_user;TableCache=true;UseCompression=True;MinimumPoolSize=10;MaximumPoolSize=50;")
	attrMap.PutStr(conventions.AttributeDBUser, "billing_user")
	attrMap.PutStr(conventions.AttributeNetHostIP, "192.0.3.122")
	attrMap.PutInt(conventions.AttributeNetHostPort, 51306)
	attrMap.PutStr(conventions.AttributeNetPeerName, "shopdb.example.com")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "192.0.2.12")
	attrMap.PutInt(conventions.AttributeNetPeerPort, 3306)
	attrMap.PutStr(conventions.AttributeNetTransport, "IP.TCP")
	attrMap.PutStr(conventions.AttributeDBName, "shopdb")
	attrMap.PutStr(conventions.AttributeDBStatement, "SELECT * FROM orders WHERE order_id = 'o4711'")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendDatabaseNoSQLAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeDBSystem, "mongodb")
	attrMap.PutStr(conventions.AttributeDBUser, "the_user")
	attrMap.PutStr(conventions.AttributeNetPeerName, "mongodb0.example.com")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "192.0.2.14")
	attrMap.PutInt(conventions.AttributeNetPeerPort, 27017)
	attrMap.PutStr(conventions.AttributeNetTransport, "IP.TCP")
	attrMap.PutStr(conventions.AttributeDBName, "shopDb")
	attrMap.PutStr(conventions.AttributeDBOperation, "findAndModify")
	attrMap.PutStr(conventions.AttributeDBMongoDBCollection, "products")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSDatasourceAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerDatasource)
	attrMap.PutStr(conventions.AttributeFaaSExecution, "DB85AF51-5E13-473D-8454-1E2D59415EAB")
	attrMap.PutStr(conventions.AttributeFaaSDocumentCollection, "faa-flight-delay-information-incoming")
	attrMap.PutStr(conventions.AttributeFaaSDocumentOperation, "insert")
	attrMap.PutStr(conventions.AttributeFaaSDocumentTime, "2020-05-09T19:50:06Z")
	attrMap.PutStr(conventions.AttributeFaaSDocumentName, "delays-20200509-13.csv")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSHTTPAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerHTTP)
	attrMap.PutStr(conventions.AttributeHTTPMethod, "POST")
	attrMap.PutStr(conventions.AttributeHTTPScheme, "https")
	attrMap.PutStr(conventions.AttributeHTTPHost, "api.opentelemetry.io")
	attrMap.PutStr(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.PutStr(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.PutInt(conventions.AttributeHTTPStatusCode, 201)
	}
	attrMap.PutStr(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1 Safari/605.1.15")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSPubSubAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerPubsub)
	attrMap.PutStr(conventions.AttributeMessagingSystem, "sqs")
	attrMap.PutStr(conventions.AttributeMessagingDestination, "video-views-au")
	attrMap.PutStr(conventions.AttributeMessagingOperation, "process")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSTimerAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerTimer)
	attrMap.PutStr(conventions.AttributeFaaSExecution, "73103A4C-E22F-4493-BDE8-EAE5CAB37B50")
	attrMap.PutStr(conventions.AttributeFaaSTime, "2020-05-09T20:00:08Z")
	attrMap.PutStr(conventions.AttributeFaaSCron, "0/15 * * * *")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendFaaSOtherAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSTrigger, conventions.AttributeFaaSTriggerOther)
	attrMap.PutInt("processed.count", 256)
	attrMap.PutDouble("processed.data", 14.46)
	attrMap.PutBool("processed.errors", false)
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendHTTPClientAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeHTTPMethod, "GET")
	attrMap.PutStr(conventions.AttributeHTTPURL, "https://opentelemetry.io/registry/")
	if includeStatus {
		attrMap.PutInt(conventions.AttributeHTTPStatusCode, 200)
		attrMap.PutStr("http.status_text", "More Than OK")
	}
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendHTTPServerAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeHTTPMethod, "POST")
	attrMap.PutStr(conventions.AttributeHTTPScheme, "https")
	attrMap.PutStr(conventions.AttributeHTTPServerName, "api22.opentelemetry.io")
	attrMap.PutInt(conventions.AttributeNetHostPort, 443)
	attrMap.PutStr(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.PutStr(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.PutInt(conventions.AttributeHTTPStatusCode, 201)
	}
	attrMap.PutStr(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	attrMap.PutStr(conventions.AttributeHTTPRoute, "/blog/posts")
	attrMap.PutStr(conventions.AttributeHTTPClientIP, "2001:506:71f0:16e::1")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendMessagingProducerAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeMessagingSystem, "nats")
	attrMap.PutStr(conventions.AttributeMessagingDestination, "time.us.east.atlanta")
	attrMap.PutStr(conventions.AttributeMessagingDestinationKind, "topic")
	attrMap.PutStr(conventions.AttributeMessagingMessageID, "AA7C5438-D93A-43C8-9961-55613204648F")
	attrMap.PutInt("messaging.sequence", 1)
	attrMap.PutStr(conventions.AttributeNetPeerIP, "10.10.212.33")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendMessagingConsumerAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeMessagingSystem, "kafka")
	attrMap.PutStr(conventions.AttributeMessagingDestination, "infrastructure-events-zone1")
	attrMap.PutStr(conventions.AttributeMessagingOperation, "receive")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:4de0:c223:a800:4e87")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendGRPCClientAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeRPCService, "PullRequestsService")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:4de0:c223:a800:4e87")
	attrMap.PutInt(conventions.AttributeNetHostPort, 8443)
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendGRPCServerAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeRPCService, "PullRequestsService")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "192.168.1.70")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendInternalAttributes(attrMap pcommon.Map) {
	attrMap.PutStr("parameters", "account=7310,amount=1817.10")
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
}

func appendMaxCountAttributes(includeStatus bool, attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeHTTPMethod, "POST")
	attrMap.PutStr(conventions.AttributeHTTPScheme, "https")
	attrMap.PutStr(conventions.AttributeHTTPHost, "api.opentelemetry.io")
	attrMap.PutStr(conventions.AttributeNetHostName, "api22.opentelemetry.io")
	attrMap.PutStr(conventions.AttributeNetHostIP, "2600:1700:1f00:11c0:1ced:afa5:fd88:9d48")
	attrMap.PutInt(conventions.AttributeNetHostPort, 443)
	attrMap.PutStr(conventions.AttributeHTTPTarget, "/blog/posts")
	attrMap.PutStr(conventions.AttributeHTTPFlavor, "2")
	if includeStatus {
		attrMap.PutInt(conventions.AttributeHTTPStatusCode, 201)
		attrMap.PutStr("http.status_text", "Created")
	}
	attrMap.PutStr(conventions.AttributeHTTPUserAgent,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	attrMap.PutStr(conventions.AttributeHTTPRoute, "/blog/posts")
	attrMap.PutStr(conventions.AttributeHTTPClientIP, "2600:1700:1f00:11c0:1ced:afa5:fd77:9d01")
	attrMap.PutStr(conventions.AttributePeerService, "IdentifyImageService")
	attrMap.PutStr(conventions.AttributeNetPeerIP, "2600:1700:1f00:11c0:1ced:afa5:fd77:9ddc")
	attrMap.PutInt(conventions.AttributeNetPeerPort, 39111)
	attrMap.PutDouble("ai-sampler.weight", 0.07)
	attrMap.PutBool("ai-sampler.absolute", false)
	attrMap.PutInt("ai-sampler.maxhops", 6)
	attrMap.PutStr("application.create.location", "https://api.opentelemetry.io/blog/posts/806673B9-4F4D-4284-9635-3A3E3E3805BE")
	stages := attrMap.PutEmptySlice("application.stages")
	stages.AppendEmpty().SetStr("Launch")
	stages.AppendEmpty().SetStr("Injestion")
	stages.AppendEmpty().SetStr("Validation")
	subMap := attrMap.PutEmptyMap("application.abflags")
	subMap.PutBool("UIx", false)
	subMap.PutBool("UI4", true)
	subMap.PutBool("flow-alt3", false)
	attrMap.PutStr("application.thread", "proc-pool-14")
	attrMap.PutStr("application.session", "")
	attrMap.PutInt("application.persist.size", 1172184)
	attrMap.PutInt("application.queue.size", 0)
	attrMap.PutStr("application.job.id", "0E38800B-9C4C-484E-8F2B-C7864D854321")
	attrMap.PutDouble("application.service.sla", 0.34)
	attrMap.PutDouble("application.service.slo", 0.55)
	attrMap.PutStr(conventions.AttributeEnduserID, "unittest")
	attrMap.PutStr(conventions.AttributeEnduserRole, "poweruser")
	attrMap.PutStr(conventions.AttributeEnduserScope, "email profile administrator")
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
			attrMap.PutStr("message.type", "SENT")
		} else {
			attrMap.PutStr("message.type", "RECEIVED")
		}
		attrMap.PutInt(conventions.AttributeMessagingMessageID, int64(index/4))
		attrMap.PutInt(conventions.AttributeMessagingMessagePayloadCompressedSizeBytes, int64(17*index))
		attrMap.PutInt(conventions.AttributeMessagingMessagePayloadSizeBytes, int64(24*index))
	case 1:
		spanEvent.SetName("custom")
		attrMap := spanEvent.Attributes()
		attrMap.PutBool("app.inretry", true)
		attrMap.PutDouble("app.progress", 0.6)
		attrMap.PutStr("app.statemap", "14|5|202")
	default:
		spanEvent.SetName("annotation")
	}

	spanEvent.SetDroppedAttributesCount(0)
}

func appendSpanLink(random io.Reader, index int, spanLinks ptrace.SpanLinkSlice) {
	spanLink := spanLinks.AppendEmpty()
	spanLink.SetTraceID(generateTraceID(random))
	spanLink.SetSpanID(generateSpanID(random))
	spanLink.TraceState().FromRaw("")
	if index%4 != 2 {
		attrMap := spanLink.Attributes()
		appendMessagingConsumerAttributes(attrMap)
		if index%4 == 1 {
			attrMap.PutBool("app.inretry", true)
			attrMap.PutDouble("app.progress", 0.6)
			attrMap.PutStr("app.statemap", "14|5|202")
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
	return pcommon.TraceID(r)
}

func generateSpanID(random io.Reader) pcommon.SpanID {
	var r [8]byte
	_, err := random.Read(r[:])
	if err != nil {
		panic(err)
	}
	return pcommon.SpanID(r)
}

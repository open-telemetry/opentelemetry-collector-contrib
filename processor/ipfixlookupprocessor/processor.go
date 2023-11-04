// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// Question: Is there a better way of doing this ?
// Needed for testing. Monkey patching
var randRead = rand.Read // Todo: potentially fix by controlling time
var searchElasticFunc = searchElastic

// schema for processor
type processorImp struct {
	config         Config
	tracesConsumer consumer.Traces
	logger         *zap.Logger
	es             *elasticsearch.Client
}

// Network Quartet
type NetworkQuartet struct {
	SourceIP        net.IP
	SourcePort      uint16
	DestinationIP   net.IP
	DestinationPort uint16
	elasticQuery    string
}

// Check if all required information is available
func (nq *NetworkQuartet) isValid() bool {
	return !nq.SourceIP.Equal(net.IP{}) && nq.SourceIP != nil &&
		nq.SourcePort > 0 &&
		!nq.DestinationIP.Equal(net.IP{}) && nq.DestinationIP != nil &&
		nq.DestinationPort > 0
}

// isEqual compares two NetworkQuartet instances for equality.
func (nq *NetworkQuartet) isEqual(other *NetworkQuartet) bool {
	return nq.SourceIP.Equal(other.SourceIP) &&
		nq.SourcePort == other.SourcePort &&
		nq.DestinationIP.Equal(other.DestinationIP) &&
		nq.DestinationPort == other.DestinationPort
}

func (nq *NetworkQuartet) String() string {
	return fmt.Sprintf("%s:%d-%s:%d",
		nq.SourceIP.String(), nq.SourcePort, nq.DestinationIP.String(), nq.DestinationPort)
}

// newConnector is a function to create a new connector
func newIPFIXLookupProcessor(logger *zap.Logger, config component.Config) *processorImp {
	logger.Info("Building IPFIXLookupProcessor")
	cfg := config.(*Config)

	return &processorImp{
		config: *cfg,
		logger: logger,
	}
}

// Capabilities implements the consumer interface.
func (p *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *processorImp) Start(_ context.Context, _ component.Host) error {
	p.logger.Info("Starting spanmetrics processor")
	p.es = connectToElasticSearch(p.logger, &p.config)
	return nil
}

func (p *processorImp) Shutdown(context.Context) error {
	p.logger.Info("Shutting down spanmetrics processor")
	return nil
}

func connectToElasticSearch(logger *zap.Logger, config *Config) *elasticsearch.Client {
	elasticConfig := elasticsearch.Config{
		Addresses:              config.Elasticsearch.Connection.Addresses,
		Username:               config.Elasticsearch.Connection.Username,
		Password:               config.Elasticsearch.Connection.Password,
		CertificateFingerprint: config.Elasticsearch.Connection.CertificateFingerprint,
	}
	es, err := elasticsearch.NewClient(elasticConfig)
	if err != nil {
		logger.Error("Failed to create Elasticsearch client", zap.String("Error", err.Error()))
	}

	response, err := es.Info()
	if err != nil {
		logger.Debug(fmt.Sprintf("Elasticsearch Error: %v", err))
	} else {
		logger.Debug(fmt.Sprintf("Elasticsearch Response: %+v", response))
	}
	return es
}

// ConsumeTraces method is called for each instance of a trace sent to the processor
func (p *processorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	p.logger.Info("ConsumeTraces: Checking Trace now")
	// loop through the levels of spans of the one trace consumed
	ipfixresourceSpan := td.ResourceSpans().AppendEmpty()
	ipfixresourceSpan.Resource().Attributes().PutStr("service.name", "IPFIX") // TODO: add config parameter
	ipfixscopedSpan := ipfixresourceSpan.ScopeSpans().AppendEmpty()
	ipfixscopedSpan.Scope().SetName("ipfix")

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				err := p.findAndHandleSpan(span, ipfixscopedSpan, td) // TODO: better method name
				if err != nil {
					p.logger.Error("Error finding and handling span", zap.Error(err))
				}

			}
		}
	}
	p.ipfixLookup(ipfixscopedSpan)
	return p.tracesConsumer.ConsumeTraces(ctx, td)
}

func (p *processorImp) findAndHandleSpan(span ptrace.Span, ipfixscopedSpan ptrace.ScopeSpans, td ptrace.Traces) error {
	p.logger.Info("ConsumeTraces: Checking Span ...", zap.String("spanid", span.SpanID().String()))

	validNetworkQuartet, networkQuartet := p.extractNetworkQuartet(span)

	// check parent for the same IP and Port Quartet
	if validNetworkQuartet {
		parentFound, parent := findSpanByID(span.ParentSpanID(), td)
		if parentFound {
			validParentNetworkQuartet, parentNetworkQuartet := p.extractNetworkQuartet(parent)
			if validParentNetworkQuartet && parentNetworkQuartet.isEqual(networkQuartet) {
				return p.findAndHandleSpan(parent, ipfixscopedSpan, td)
			}
		}
		createSummarySpan(span, networkQuartet, ipfixscopedSpan)
	} else {
		p.logger.Info("Unable to find all required information to lookup IPFIX information")
	}
	return nil
}

func findSpanByID(spanID pcommon.SpanID, td ptrace.Traces) (bool, ptrace.Span) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				if span.SpanID() == spanID {
					return true, span
				}

			}
		}
	}
	return false, ptrace.Span{}
}

// ipfixLookup performs IPFIX lookup for the given network quartet.
func (p *processorImp) ipfixLookup(ipfixscopedSpan ptrace.ScopeSpans) {
	var requestNetworkQuartet *NetworkQuartet
	for i := 0; i < ipfixscopedSpan.Spans().Len(); i++ {
		summarySpan := ipfixscopedSpan.Spans().At(i)
		var valid bool
		valid, requestNetworkQuartet = p.extractNetworkQuartet(summarySpan)
		if !valid {
			p.logger.Error("Unable to find all required information to lookup IPFIX information")
			return
		}

		requestSpans, requestSpanCount := p.performIPFIXLookup(summarySpan, requestNetworkQuartet, ipfixscopedSpan)
		// Create response network quartet
		responseNetworkQuartet := &NetworkQuartet{
			SourceIP:        requestNetworkQuartet.DestinationIP,
			SourcePort:      requestNetworkQuartet.DestinationPort,
			DestinationIP:   requestNetworkQuartet.SourceIP,
			DestinationPort: requestNetworkQuartet.SourcePort,
		}
		responseSpans, responeSpanCount := p.performIPFIXLookup(summarySpan, responseNetworkQuartet, ipfixscopedSpan)
		summarySpan.Attributes().PutInt("ipfix.request.flows", requestSpanCount)
		summarySpan.Attributes().PutInt("ipfix.response.flows", responeSpanCount)

		p.setNameOfIPFIXSpan(requestSpans, "Request - ")
		p.setNameOfIPFIXSpan(responseSpans, "Response - ")

		earliest, latest := findMinMaxTimestamp(append(requestSpans, responseSpans...))
		summarySpan.SetStartTimestamp(earliest)
		summarySpan.SetEndTimestamp(latest)

		requestQueryField := summarySpan.Attributes().PutEmptyMap("z.elasticQuery.request")
		if err := requestQueryField.FromRaw(gjson.Parse(requestNetworkQuartet.elasticQuery).Value().(map[string]any)); err != nil {
			p.logger.Error("Error parsing request query field", zap.Error(err))
		}
		responseQueryField := summarySpan.Attributes().PutEmptyMap("z.elasticQuery.response")
		if err := responseQueryField.FromRaw(gjson.Parse(responseNetworkQuartet.elasticQuery).Value().(map[string]any)); err != nil {
			p.logger.Error("Error parsing response query field", zap.Error(err))
		}
	}

}

func findMinMaxTimestamp(spans []ptrace.Span) (pcommon.Timestamp, pcommon.Timestamp) {
	var earliestTimestamp pcommon.Timestamp
	var latestTimestamp pcommon.Timestamp
	for _, span := range spans {
		startTimestamp := span.StartTimestamp()
		if earliestTimestamp == 0 || startTimestamp < earliestTimestamp {
			earliestTimestamp = startTimestamp
		}
		endTimestamp := span.EndTimestamp()
		if latestTimestamp == 0 || endTimestamp > latestTimestamp {
			latestTimestamp = endTimestamp
		}
	}
	return earliestTimestamp, latestTimestamp
}

func (p *processorImp) setNameOfIPFIXSpan(spans []ptrace.Span, prefix string) {
	for _, span := range spans {
		name, found := span.Attributes().Get(p.config.QueryParameters.DeviceIdentifier)
		if found {
			span.SetName(prefix + name.AsString())
		} else {
			span.SetName(prefix + "no IPFIX logs found")
		}
	}
}

// createSummarySpan creates a summary span for the given network quartet.
func createSummarySpan(span ptrace.Span, requestNetworkQuartet *NetworkQuartet, ipfixscopedSpan ptrace.ScopeSpans) ptrace.Span {
	summarySpan := findSummarySpan(ipfixscopedSpan, requestNetworkQuartet.String())
	if summarySpan != (ptrace.Span{}) {
		return summarySpan
	}
	summarySpan = createNetworkSpan(ipfixscopedSpan, span)
	summarySpan.SetName(requestNetworkQuartet.String())
	summarySpan.SetParentSpanID(span.ParentSpanID())

	summarySpan.Attributes().PutStr("src.ip", requestNetworkQuartet.SourceIP.String())
	summarySpan.Attributes().PutStr("src.port", strconv.Itoa(int(requestNetworkQuartet.SourcePort)))
	summarySpan.Attributes().PutStr("dst.ip", requestNetworkQuartet.DestinationIP.String())
	summarySpan.Attributes().PutStr("dst.port", strconv.Itoa(int(requestNetworkQuartet.DestinationPort)))
	summarySpan.Attributes().PutInt("ipfix.request.flows", 0)
	summarySpan.Attributes().PutInt("ipfix.response.flows", 0)
	return summarySpan
}

// findSummarySpan finds the summary span for the given network quartet in the scope spans.
func findSummarySpan(ipfixscopedSpan ptrace.ScopeSpans, networkQuartetName string) ptrace.Span {
	for i := 0; i < ipfixscopedSpan.Spans().Len(); i++ {
		span := ipfixscopedSpan.Spans().At(i)
		if span.Name() == networkQuartetName {
			return span
		}
	}
	return ptrace.Span{}
}

// performIPFIXLookup performs IPFIX lookup for the given network quartet.
func (p *processorImp) performIPFIXLookup(summarySpan ptrace.Span, networkQuartet *NetworkQuartet, ipfixscopedSpan ptrace.ScopeSpans) ([]ptrace.Span, int64) {
	// Search IPFIX events for the network quartet
	elasticResponse := p.lookupSpanInElasticSearch(summarySpan, networkQuartet)
	var networkSpans []ptrace.Span
	hits := gjson.GetBytes(elasticResponse, "hits.hits")
	if hits.IsArray() {
		hitsArray := hits.Array()
		if len(hitsArray) == 0 {
			networkSpan := createNetworkSpan(ipfixscopedSpan, summarySpan)
			networkSpan.SetParentSpanID(summarySpan.SpanID())
			summarySpan.Attributes().PutStr("NoHits-Warning", "No hits were found when searching!\nThis could be due to:\n- Sampling rate\n- Bad time settings\n- Others")
			networkSpans = append(networkSpans, networkSpan)
			return networkSpans, 0

		}
		for i := 0; i < len(hitsArray); i++ {
			// Create IPFIX span
			networkSpan := createNetworkSpan(ipfixscopedSpan, summarySpan)
			networkSpan.SetParentSpanID(summarySpan.SpanID())
			if addAttributesErr := p.addAttributesToNetworkSpan(networkSpan, hitsArray[i]); addAttributesErr != nil {
				p.logger.Error("Error adding attributes to span", zap.Error(addAttributesErr))
			}
			networkSpans = append(networkSpans, networkSpan)

		}

	}

	return networkSpans, int64(len((networkSpans)))
}

func (p *processorImp) extractNetworkQuartet(span ptrace.Span) (bool, *NetworkQuartet) {
	// NetworkQuartet represents a quartet of network information.
	attrs := span.Attributes()
	mapping := attrs.AsRaw()

	// Extracting needed information
	sourceIPValue, foundSourceIP := getStringValue(mapping, p.config.Spans.SpanFields.SourceIPs...)
	sourcePortValue, foundSourcePort := getStringValue(mapping, p.config.Spans.SpanFields.SourcePorts...)
	destinationIPandPortValue, foundDestinationIPandPort := getStringValue(mapping, p.config.Spans.SpanFields.DestinationIPandPort...)
	destinationIPValue, foundDestinationIP := getStringValue(mapping, p.config.Spans.SpanFields.DestinationIPs...)
	destinationPortValue, foundDestinationPort := getStringValue(mapping, p.config.Spans.SpanFields.DestinationPorts...)

	// Converting concatenated IP and port, e.g: 192.168.1.0:443
	if !foundDestinationIP && !foundDestinationPort && foundDestinationIPandPort {
		destinationIPandPortArray := strings.Split(destinationIPandPortValue, ":")
		destinationIPValue = destinationIPandPortArray[0]
		foundDestinationIP = true
		destinationPortValue = destinationIPandPortArray[1]
		foundDestinationPort = true
	}

	networkQuartet := NetworkQuartet{}
	if foundSourceIP {
		networkQuartet.SourceIP = p.convertToIP(sourceIPValue)
	}
	if foundSourcePort {
		networkQuartet.SourcePort = p.convertToPort(sourcePortValue)
	}
	if foundDestinationIP {
		networkQuartet.DestinationIP = p.convertToIP(destinationIPValue)
	}
	if foundDestinationPort {
		networkQuartet.DestinationPort = p.convertToPort(destinationPortValue)
	}
	return networkQuartet.isValid(), &networkQuartet
}

func getStringValue(mapping map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value := mapping[key]
		switch v := value.(type) {
		case string:
			return v, true
		case int64:
			return strconv.FormatInt(v, 10), true
		}
	}
	return "", false
}

func (p *processorImp) convertToIP(ipString string) net.IP {
	ip := net.ParseIP(ipString)
	if ip == nil {
		p.logger.Debug(fmt.Sprintf("failed to Parse IP from : %v", ipString))
	}
	return ip
}

func (p *processorImp) convertToPort(sourcePortValue string) uint16 {
	sourcePort, err := strconv.ParseUint(sourcePortValue, 10, 16)
	if err == nil {
		p.logger.Debug(fmt.Sprintf("sourcePort: %v", err))
	}
	return uint16(sourcePort)
}

func (p *processorImp) addAttributesToNetworkSpan(networkSpan ptrace.Span, hit gjson.Result) error {
	errHitsAttribute := AddAttributeByJSONPath(networkSpan, hit, "hits.total.value")
	if errHitsAttribute != nil {
		p.logger.Error("Error adding attribute to span", zap.Error(errHitsAttribute))
	}

	// TODO: make configurable in config
	attributePaths := p.config.SpanAttributeFields

	for _, path := range attributePaths {
		err := AddAttributeByJSONPath(networkSpan, hit, path)
		if err != nil {
			p.logger.Error("Error adding attribute to span", zap.Error(err))
		}
	}

	renameAttribute(networkSpan, "@this", "z.elasticResponse")

	// Set time
	eventStart, err := extractTimeStamp(hit, `fields.event\.start.0`)
	if err != nil {
		return err
	}
	eventEnd, err := extractTimeStamp(hit, `fields.event\.end.0`)
	if err != nil {
		return err
	}
	networkSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(eventStart))
	networkSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(eventEnd))
	return nil
}

func renameAttribute(networkSpan ptrace.Span, oldAttributeName string, newAttributeName string) {
	attribute, _ := networkSpan.Attributes().Get(oldAttributeName)
	attribute.Map().CopyTo(networkSpan.Attributes().PutEmptyMap(newAttributeName))
	networkSpan.Attributes().Remove(oldAttributeName)
}

func createNetworkSpan(ipfixscopedSpan ptrace.ScopeSpans, span ptrace.Span) ptrace.Span {
	networkSpan := ipfixscopedSpan.Spans().AppendEmpty()
	networkSpan.SetTraceID(span.TraceID())
	networkSpan.SetSpanID(generateRandomSpanID())
	networkSpan.SetName("injected")

	networkSpan.SetStartTimestamp(span.EndTimestamp())
	networkSpan.SetEndTimestamp(span.EndTimestamp())

	return networkSpan
}

func (p *processorImp) lookupSpanInElasticSearch(span ptrace.Span, networkQuartet *NetworkQuartet) []byte {
	p.logger.Info("ConsumeTraces: attributes found in Span", zap.String("spanid", span.SpanID().String()))

	elasticQuery := p.generateElasticsearchQuery(
		networkQuartet.SourceIP,
		networkQuartet.SourcePort,
		networkQuartet.DestinationIP,
		networkQuartet.DestinationPort,
		span.EndTimestamp().AsTime(),
		time.Second*time.Duration(p.config.Timing.LookupWindow),
	)
	networkQuartet.elasticQuery = elasticQuery
	p.logger.Debug("elasticsearch query", zap.String("query", elasticQuery))

	elasticResponse := p.lookupIpfixInElastic(elasticQuery)

	return elasticResponse
}

func (p *processorImp) lookupIpfixInElastic(query string) []byte {

	searchResults, err := searchElasticFunc(query, p.es)
	if err != nil {
		p.logger.Error("Elasticsearch Result Error", zap.Error(err))
		return nil
	}

	responseBody, err := io.ReadAll(searchResults.Body)
	if err != nil {
		p.logger.Error("Error reading elastic response body", zap.Error(err))
	}

	p.logger.Debug("Elasticsearch Response", zap.ByteString("elasticsearch.Body", responseBody))
	return responseBody
}

func (p *processorImp) generateElasticsearchQuery(sourceIP net.IP, sourcePort uint16, destinationIP net.IP, destinationPort uint16, startTime time.Time, duration time.Duration) string {
	queryTemplate := `
	{
		"query": {
		  "bool": { 
			"must": [
			  { "match": { "%s": "%s" }},
			  { "match": { "%s": "%s" }},
			  { "match": { "%s": "%d" }},
			  { "match": { "%s": "%s" }},
			  { "match": { "%s": "%d" }},
			  { "range": { "@timestamp": { "gte": "%s", "lte": "%s" }}}
			]
		  }
		},
		"_source": false,
		"fields": [
		  {
			"field": "*",
			"include_unmapped": "true"
		  }
		]
	  }`

	endTime := startTime.Add(duration)
	startTime = startTime.Add(-duration)

	query := fmt.Sprintf(
		queryTemplate,
		p.config.QueryParameters.BaseQuery.FieldName,
		p.config.QueryParameters.BaseQuery.FieldValue,
		p.config.QueryParameters.LookupFields.SourceIP,
		sourceIP.String(),
		p.config.QueryParameters.LookupFields.SourcePort,
		sourcePort,
		p.config.QueryParameters.LookupFields.DestinationIP,
		destinationIP.String(),
		p.config.QueryParameters.LookupFields.DestinationPort,
		destinationPort,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)

	return query
}

func searchElastic(query string, es *elasticsearch.Client) (*esapi.Response, error) {
	searchResults, err := es.Search(
		es.Search.WithIndex("logs-*"),
		es.Search.WithBody(strings.NewReader(query)),
	)
	return searchResults, err
}

func generateRandomSpanID() pcommon.SpanID {
	var spanID pcommon.SpanID
	_, err := randRead(spanID[:])
	if err != nil {
		return pcommon.SpanID{}
	}
	return spanID
}

func AddAttributeByJSONPath(span ptrace.Span, json gjson.Result, jsonPath string) error {
	result := json.Get(jsonPath)

	switch result.Type {
	case gjson.Number:
		span.Attributes().PutInt(jsonPath, result.Int())
	case gjson.String:
		span.Attributes().PutStr(jsonPath, result.String())
	case gjson.True, gjson.False:
		span.Attributes().PutBool(jsonPath, result.Bool())
	case gjson.JSON:
		switch {
		case result.IsArray():
			array, ok := result.Value().([]any)
			if !ok {
				return nil
			}
			attributeArray := span.Attributes().PutEmptySlice(jsonPath)
			if err := attributeArray.FromRaw(array); err != nil {
				return err
			}
		case result.IsObject():
			m, ok := result.Value().(map[string]any)
			if !ok {
				return nil
			}
			attributeMap := span.Attributes().PutEmptyMap(jsonPath)
			if err := attributeMap.FromRaw(m); err != nil {
				return err
			}
		default:
			return errors.New("unsupported JSON type")
		}
	case gjson.Null:
		span.Attributes().PutStr(jsonPath, "null")
	default:
		return errors.New("unsupported JSON type")
	}

	return nil
}

func extractTimeStamp(elasticResponse gjson.Result, jsonPath string) (time.Time, error) {
	timeStamp, err := time.Parse(time.RFC3339, elasticResponse.Get(jsonPath).String())
	return timeStamp, err
}

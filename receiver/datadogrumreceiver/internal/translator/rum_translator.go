// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func PutBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

type RUMPayload struct {
	Type string
}

func parseW3CTraceContext(traceparent string) (traceID pcommon.TraceID, spanID pcommon.SpanID, err error) {
	// W3C traceparent format: version-traceID-spanID-flags
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid traceparent format: %s", traceparent)
	}

	// Parse trace ID (32 hex characters)
	traceIDBytes, err := hex.DecodeString(parts[1])
	if err != nil || len(traceIDBytes) != 16 {
		return pcommon.NewTraceIDEmpty(), pcommon.SpanID{}, fmt.Errorf("invalid trace ID: %s", parts[1])
	}
	copy(traceID[:], traceIDBytes)

	// Parse span ID
	spanIDBytes, err := hex.DecodeString(parts[2])
	if err != nil || len(spanIDBytes) != 8 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid parent ID: %s", parts[2])
	}
	copy(spanID[:], spanIDBytes)

	return traceID, spanID, nil
}

func parseIDs(payload map[string]any, req *http.Request) (pcommon.TraceID, pcommon.SpanID, error) {
	ddMetadata, ok := payload["_dd"].(map[string]any)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to find _dd metadata in payload")
	}

	traceIDString, ok := ddMetadata["trace_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve traceID from payload")
	}
	traceID, err := strconv.ParseUint(traceIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse traceID: %w", err)
	}

	spanIDString, ok := ddMetadata["span_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve spanID from payload")
	}
	spanID, err := strconv.ParseUint(spanIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse spanID: %w", err)
	}

	return uInt64ToTraceID(0, traceID), uInt64ToSpanID(spanID), nil
}

func parseRUMRequestIntoResource(resource pcommon.Resource, payload map[string]any, req *http.Request, rawRequestBody []byte) {
	resource.Attributes().PutStr(semconv.AttributeServiceName, "browser-rum-sdk")

	prettyPayload, _ := json.MarshalIndent(payload, "", "\t")
	resource.Attributes().PutStr("pretty_payload", string(prettyPayload))

	bodyDump := resource.Attributes().PutEmptyBytes("request_body_dump")
	bodyDump.FromRaw(rawRequestBody)

	// Store URL query parameters as attributes
	queryAttrs := resource.Attributes().PutEmptyMap("request_query")
	for paramName, paramValues := range req.URL.Query() {
		paramValueList := queryAttrs.PutEmptySlice(paramName)
		for _, paramValue := range paramValues {
			paramValueList.AppendEmpty().SetStr(paramValue)
		}
	}

	resource.Attributes().PutStr("request_ddforward", req.URL.Query().Get("ddforward"))
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[0:8], high)
	binary.BigEndian.PutUint64(traceID[8:16], low)
	return pcommon.TraceID(traceID)
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}

type AttributeType string

const (
	StringAttribute        AttributeType = "str"
	BoolAttribute          AttributeType = "bool"
	NumberAttribute        AttributeType = "num"
	IntegerAttribute       AttributeType = "int"
	ObjectAttribute        AttributeType = "obj"
	ArrayAttribute         AttributeType = "arr"
	StringOrArrayAttribute AttributeType = "str_or_arr"
)

type AttributeMeta struct {
	OTLPName string
	Type     AttributeType
}

var attributeMetaMap = map[string]AttributeMeta{
	// _common-schema.json
	"date":                                  {OTLPName: "datadog.date", Type: IntegerAttribute},
	"application.id":                        {OTLPName: "datadog.application.id", Type: StringAttribute},
	"application.current_locale":            {OTLPName: "datadog.application.current_locale", Type: StringAttribute},
	"service":                               {OTLPName: "service.name", Type: StringAttribute},
	"version":                               {OTLPName: "service.version", Type: StringAttribute},
	"build_version":                         {OTLPName: "datadog.build_version", Type: StringAttribute},
	"build_id":                              {OTLPName: "datadog.build_id", Type: StringAttribute},
	"session.id":                            {OTLPName: "session.id", Type: StringAttribute},
	"session.type":                          {OTLPName: "datadog.session.type", Type: StringAttribute},
	"session.has_replay":                    {OTLPName: "datadog.session.has_replay", Type: BoolAttribute},
	"source":                                {OTLPName: "datadog.source", Type: StringAttribute},
	"view.id":                               {OTLPName: "datadog.view.id", Type: StringAttribute},
	"view.referrer":                         {OTLPName: "datadog.view.referrer", Type: StringAttribute},
	"view.url":                              {OTLPName: "datadog.view.url", Type: StringAttribute},
	"view.name":                             {OTLPName: "datadog.view.name", Type: StringAttribute},
	"usr.id":                                {OTLPName: "user.id", Type: StringAttribute},
	"usr.name":                              {OTLPName: "user.full_name", Type: StringAttribute},
	"usr.email":                             {OTLPName: "user.email", Type: StringAttribute},
	"usr.anonymoud_id":                      {OTLPName: "user.hash", Type: StringAttribute},
	"account.id":                            {OTLPName: "datadog.account.id", Type: StringAttribute},
	"account.name":                          {OTLPName: "user.name", Type: StringAttribute},
	"connectivity.status":                   {OTLPName: "datadog.connectivity.status", Type: StringAttribute},
	"connectivity.interfaces":               {OTLPName: "datadog.connectivity.interfaces", Type: ArrayAttribute},
	"connectivity.effective_type":           {OTLPName: "datadog.connectivity.effective_type", Type: StringAttribute},
	"connectivity.cellular.technology":      {OTLPName: "datadog.connectivity.cellular.technology", Type: StringAttribute},
	"connectivity.cellular.carrier_name":    {OTLPName: "datadog.connectivity.cellular.carrier_name", Type: StringAttribute},
	"display.viewport.width":                {OTLPName: "datadog.display.viewport.width", Type: NumberAttribute},
	"display.viewport.height":               {OTLPName: "datadog.display.viewport.height", Type: NumberAttribute},
	"synthetics.test_id":                    {OTLPName: "datadog.synthetics.test_id", Type: StringAttribute},
	"synthetics.result_id":                  {OTLPName: "datadog.synthetics.result_id", Type: StringAttribute},
	"synthetics.injected":                   {OTLPName: "datadog.synthetics.injected", Type: BoolAttribute},
	"ci_test.test_execution_id":             {OTLPName: "datadog.ci_test.test_execution_id", Type: StringAttribute},
	"os.name":                               {OTLPName: "os.name", Type: StringAttribute},
	"os.version":                            {OTLPName: "os.version", Type: StringAttribute},
	"os.build":                              {OTLPName: "os.build_id", Type: StringAttribute},
	"os.version_major":                      {OTLPName: "datadog.os.version_major", Type: StringAttribute},
	"device.type":                           {OTLPName: "datadog.device.type", Type: StringAttribute},
	"device.name":                           {OTLPName: "device.model.name", Type: StringAttribute},
	"device.model":                          {OTLPName: "device.model.identifier", Type: StringAttribute},
	"device.brand":                          {OTLPName: "device.manufacturer", Type: StringAttribute},
	"device.architecture":                   {OTLPName: "datadog.device.architecture", Type: StringAttribute},
	"device.locale":                         {OTLPName: "datadog.device.locale", Type: StringAttribute},
	"device.locales":                        {OTLPName: "datadog.device.locales", Type: ArrayAttribute},
	"device.time_zone":                      {OTLPName: "datadog.device.time_zone", Type: StringAttribute},
	"device.battery_level":                  {OTLPName: "datadog.device.battery_level", Type: NumberAttribute},
	"device.power_saving_mode":              {OTLPName: "datadog.device.power_saving_mode", Type: BoolAttribute},
	"device.brightness_level":               {OTLPName: "datadog.device.brightness_level", Type: IntegerAttribute},
	"_dd.format_version":                    {OTLPName: "datadog._dd.format_version", Type: NumberAttribute},
	"_dd.session.plan":                      {OTLPName: "datadog._dd.session.plan", Type: NumberAttribute},
	"_dd.session.session_precondition":      {OTLPName: "datadog._dd.session.session_precondition", Type: StringAttribute},
	"_dd.configuration.session_sample_rate": {OTLPName: "datadog._dd.configuration.session_sample_rate", Type: NumberAttribute},
	"_dd.configuration.session_replay_sample_rate": {OTLPName: "datadog._dd.configuration.session_replay_sample_rate", Type: NumberAttribute},
	"_dd.configuration.profiling_sample_rate":      {OTLPName: "datadog._dd.configuration.profiling_sample_rate", Type: NumberAttribute},
	"_dd.browser_sdk_version":                      {OTLPName: "datadog._dd.browser_sdk_version", Type: StringAttribute},
	"_dd.sdk_name":                                 {OTLPName: "datadog._dd.sdk_name", Type: StringAttribute},
	"context":                                      {OTLPName: "datadog.context", Type: ObjectAttribute},

	// _view-container-schema.json
	"container.view.id": {OTLPName: "datadog.container.view.id", Type: StringAttribute},
	"container.source":  {OTLPName: "datadog.container.source", Type: StringAttribute},

	// _action-child-schema.json
	"action.id": {OTLPName: "datadog.action.id", Type: StringOrArrayAttribute},

	// action-schema.json
	"action.type":                {OTLPName: "datadog.action.type", Type: StringAttribute},
	"action.loading_time":        {OTLPName: "datadog.action.loading_time", Type: IntegerAttribute},
	"action.target.name":         {OTLPName: "datadog.action.target.name", Type: StringAttribute},
	"action.frustration.type":    {OTLPName: "datadog.action.frustration.type", Type: ArrayAttribute},
	"action.error.count":         {OTLPName: "datadog.action.error.count", Type: IntegerAttribute},
	"action.crash.count":         {OTLPName: "datadog.action.crash.count", Type: IntegerAttribute},
	"action.long_task.count":     {OTLPName: "datadog.action.long_task.count", Type: IntegerAttribute},
	"action.resource.count":      {OTLPName: "datadog.action.resource.count", Type: IntegerAttribute},
	"view.in_foreground":         {OTLPName: "datadog.view.in_foreground", Type: BoolAttribute},
	"_dd.action.position.x":      {OTLPName: "datadog._dd.action.position.x", Type: IntegerAttribute},
	"_dd.action.position.y":      {OTLPName: "datadog._dd.action.position.y", Type: IntegerAttribute},
	"_dd.action.target.selector": {OTLPName: "datadog._dd.action.target.selector", Type: StringAttribute},
	"_dd.action.target.width":    {OTLPName: "datadog._dd.action.target.width", Type: IntegerAttribute},
	"_dd.action.target.height":   {OTLPName: "datadog._dd.action.target.height", Type: IntegerAttribute},
	"_dd.action.name_source":     {OTLPName: "datadog._dd.action.name_source", Type: StringAttribute},

	// error-schema.json
	"error.id":                         {OTLPName: "datadog.error.id", Type: StringAttribute},
	"error.message":                    {OTLPName: "error.message", Type: StringAttribute},
	"error.source":                     {OTLPName: "datadog.error.source", Type: StringAttribute},
	"error.stack":                      {OTLPName: "datadog.error.stack", Type: StringAttribute},
	"error.causes":                     {OTLPName: "datadog.error.causes", Type: ArrayAttribute},
	"error.causes.message":             {OTLPName: "datadog.error.causes.message", Type: StringAttribute},
	"error.causes.type":                {OTLPName: "datadog.error.causes.type", Type: StringAttribute},
	"error.causes.stack":               {OTLPName: "datadog.error.causes.stack", Type: StringAttribute},
	"error.causes.source":              {OTLPName: "datadog.error.causes.source", Type: StringAttribute},
	"error.is_crash":                   {OTLPName: "datadog.error.is_crash", Type: BoolAttribute},
	"error.fingerprint":                {OTLPName: "datadog.error.fingerprint", Type: StringAttribute},
	"error.type":                       {OTLPName: "error.type", Type: StringAttribute},
	"error.category":                   {OTLPName: "datadog.error.category", Type: StringAttribute},
	"error.handling":                   {OTLPName: "datadog.error.handling", Type: StringAttribute},
	"error.handling_stack":             {OTLPName: "datadog.error.handling_stack", Type: StringAttribute},
	"error.source_type":                {OTLPName: "datadog.error.source_type", Type: StringAttribute},
	"error.resource.method":            {OTLPName: "datadog.error.resource.method", Type: StringAttribute},
	"error.resource.status_code":       {OTLPName: "datadog.error.resource.status_code", Type: IntegerAttribute},
	"error.resource.url":               {OTLPName: "datadog.error.resource.url", Type: StringAttribute},
	"error.resource.provider.domain":   {OTLPName: "datadog.error.resource.provider.domain", Type: StringAttribute},
	"error.resource.provider.name":     {OTLPName: "datadog.error.resource.provider.name", Type: StringAttribute},
	"error.resource.provider.type":     {OTLPName: "datadog.error.resource.provider.type", Type: StringAttribute},
	"error.threads":                    {OTLPName: "datadog.error.threads", Type: ArrayAttribute},
	"error.threads.name":               {OTLPName: "datadog.error.threads.name", Type: StringAttribute},
	"error.threads.crashed":            {OTLPName: "datadog.error.threads.crashed", Type: BoolAttribute},
	"error.threads.stack":              {OTLPName: "datadog.error.threads.stack", Type: StringAttribute},
	"error.threads.state":              {OTLPName: "datadog.error.threads.state", Type: StringAttribute},
	"error.binary_images":              {OTLPName: "datadog.error.binary_images", Type: ArrayAttribute},
	"error.binary_images.uuid":         {OTLPName: "datadog.error.binary_images.uuid", Type: StringAttribute},
	"error.binary_images.name":         {OTLPName: "datadog.error.binary_images.name", Type: StringAttribute},
	"error.binary_images.is_system":    {OTLPName: "datadog.error.binary_images.is_system", Type: BoolAttribute},
	"error.binary_images.load_address": {OTLPName: "datadog.error.binary_images.load_address", Type: StringAttribute},
	"error.binary_images.max_address":  {OTLPName: "datadog.error.binary_images.max_address", Type: StringAttribute},
	"error.binary_images.arch":         {OTLPName: "datadog.error.binary_images.arch", Type: StringAttribute},
	"error.was_truncated":              {OTLPName: "datadog.error.was_truncated", Type: BoolAttribute},
	"error.meta.code_type":             {OTLPName: "datadog.error.meta.code_type", Type: StringAttribute},
	"error.meta.parent_process":        {OTLPName: "datadog.error.meta.parent_process", Type: StringAttribute},
	"error.meta.incident_identifier":   {OTLPName: "datadog.error.meta.incident_identifier", Type: StringAttribute},
	"error.meta.process":               {OTLPName: "datadog.error.meta.process", Type: StringAttribute},
	"error.meta.exception_type":        {OTLPName: "datadog.error.meta.exception_type", Type: StringAttribute},
	"error.meta.exception_codes":       {OTLPName: "datadog.error.meta.exception_codes", Type: StringAttribute},
	"error.meta.path":                  {OTLPName: "datadog.error.meta.path", Type: StringAttribute},
	"error.csp.disposition":            {OTLPName: "datadog.error.csp.disposition", Type: StringAttribute},
	"error.time_since_app_start":       {OTLPName: "datadog.error.time_since_app_start", Type: IntegerAttribute},
	"freeze.duration":                  {OTLPName: "datadog.freeze.duration", Type: IntegerAttribute},
	"feature_flags":                    {OTLPName: "datadog.feature_flags", Type: ObjectAttribute},

	// long_task-schema.json
	"long_task.id":                                       {OTLPName: "datadog.long_task.id", Type: StringAttribute},
	"long_task.start_time":                               {OTLPName: "datadog.long_task.start_time", Type: NumberAttribute},
	"long_task.entry_type":                               {OTLPName: "datadog.long_task.entry_type", Type: StringAttribute},
	"long_task.duration":                                 {OTLPName: "datadog.long_task.duration", Type: IntegerAttribute},
	"long_task.blocking_duration":                        {OTLPName: "datadog.long_task.blocking_duration", Type: IntegerAttribute},
	"long_task.render_start":                             {OTLPName: "datadog.long_task.render_start", Type: NumberAttribute},
	"long_task.style_and_layout_start":                   {OTLPName: "datadog.long_task.style_and_layout_start", Type: NumberAttribute},
	"long_task.first_ui_event_timestamp":                 {OTLPName: "datadog.long_task.first_ui_event_timestamp", Type: NumberAttribute},
	"long_task.is_frozen_frame":                          {OTLPName: "datadog.long_task.is_frozen_frame", Type: BoolAttribute},
	"long_task.scripts":                                  {OTLPName: "datadog.long_task.scripts", Type: ArrayAttribute},
	"long_task.scripts.duration":                         {OTLPName: "datadog.long_task.scripts.duration", Type: IntegerAttribute},
	"long_task.scripts.pause_duration":                   {OTLPName: "datadog.long_task.scripts.pause_duration", Type: IntegerAttribute},
	"long_task.scripts.forced_style_and_layout_duration": {OTLPName: "datadog.long_task.scripts.forced_style_and_layout_duration", Type: IntegerAttribute},
	"long_task.scripts.start_time":                       {OTLPName: "datadog.long_task.scripts.start_time", Type: NumberAttribute},
	"long_task.scripts.execution_start":                  {OTLPName: "datadog.long_task.scripts.execution_start", Type: NumberAttribute},
	"long_task.scripts.souce_url":                        {OTLPName: "datadog.long_task.scripts.souce_url", Type: StringAttribute},
	"long_task.scripts.source_function_name":             {OTLPName: "datadog.long_task.scripts.source_function_name", Type: StringAttribute},
	"long_task.scripts.source_char_position":             {OTLPName: "datadog.long_task.scripts.source_char_position", Type: IntegerAttribute},
	"long_task.scripts.invoker":                          {OTLPName: "datadog.long_task.scripts.invoker", Type: StringAttribute},
	"long_task.scripts.invoker_type":                     {OTLPName: "datadog.long_task.scripts.invoker_type", Type: StringAttribute},
	"long_task.scripts.window_attribution":               {OTLPName: "datadog.long_task.scripts.window_attribution", Type: StringAttribute},
	"_dd.discarded":                                      {OTLPName: "datadog._dd.discarded", Type: BoolAttribute},
	"_dd.profiling":                                      {OTLPName: "datadog._dd.profiling", Type: ObjectAttribute},

	// resource-schema.json
	"resource.id":                     {OTLPName: "datadog.resource.id", Type: StringAttribute},
	"resource.type":                   {OTLPName: "datadog.resource.type", Type: StringAttribute},
	"resource.method":                 {OTLPName: "datadog.resource.method", Type: StringAttribute},
	"resource.url":                    {OTLPName: "datadog.resource.url", Type: StringAttribute},
	"resource.status_code":            {OTLPName: "datadog.resource.status_code", Type: IntegerAttribute},
	"resource.duration":               {OTLPName: "datadog.resource.duration", Type: IntegerAttribute},
	"resource.size":                   {OTLPName: "datadog.resource.size", Type: IntegerAttribute},
	"resource.encoded_body_size":      {OTLPName: "datadog.resource.encoded_body_size", Type: IntegerAttribute},
	"resource.decoded_body_size":      {OTLPName: "datadog.resource.decoded_body_size", Type: IntegerAttribute},
	"resource.transfer_size":          {OTLPName: "datadog.resource.transfer_size", Type: IntegerAttribute},
	"resource.render_blocking_status": {OTLPName: "datadog.resource.render_blocking_status", Type: StringAttribute},
	"resource.worker.duration":        {OTLPName: "datadog.resource.worker.duration", Type: IntegerAttribute},
	"resource.worker.start":           {OTLPName: "datadog.resource.worker.start", Type: IntegerAttribute},
	"resource.redirect.duration":      {OTLPName: "datadog.resource.redirect.duration", Type: IntegerAttribute},
	"resource.redirect.start":         {OTLPName: "datadog.resource.redirect.start", Type: IntegerAttribute},
	"resource.dns.duration":           {OTLPName: "datadog.resource.dns.duration", Type: IntegerAttribute},
	"resource.dns.start":              {OTLPName: "datadog.resource.dns.start", Type: IntegerAttribute},
	"resource.connect.duration":       {OTLPName: "datadog.resource.connect.duration", Type: IntegerAttribute},
	"resource.connect.start":          {OTLPName: "datadog.resource.connect.start", Type: IntegerAttribute},
	"resource.ssl.duration":           {OTLPName: "datadog.resource.ssl.duration", Type: IntegerAttribute},
	"resource.ssl.start":              {OTLPName: "datadog.resource.ssl.start", Type: IntegerAttribute},
	"resource.first_byte.duration":    {OTLPName: "datadog.resource.first_byte.duration", Type: IntegerAttribute},
	"resource.first_byte.start":       {OTLPName: "datadog.resource.first_byte.start", Type: IntegerAttribute},
	"resource.download.duration":      {OTLPName: "datadog.resource.download.duration", Type: IntegerAttribute},
	"resource.download.start":         {OTLPName: "datadog.resource.download.start", Type: IntegerAttribute},
	"resource.protocol":               {OTLPName: "datadog.resource.protocol", Type: StringAttribute},
	"resource.delivery_type":          {OTLPName: "datadog.resource.delivery_type", Type: StringAttribute},
	"resource.provider.domain":        {OTLPName: "datadog.resource.provider.domain", Type: StringAttribute},
	"resource.provider.name":          {OTLPName: "datadog.resource.provider.name", Type: StringAttribute},
	"resource.provider.type":          {OTLPName: "datadog.resource.provider.type", Type: StringAttribute},
	"resource.graphql.operationType":  {OTLPName: "datadog.resource.graphql.operationType", Type: StringAttribute},
	"resource.graphql.operationName":  {OTLPName: "datadog.resource.graphql.operationName", Type: StringAttribute},
	"resource.graphql.payload":        {OTLPName: "datadog.resource.graphql.payload", Type: StringAttribute},
	"resource.graphql.variables":      {OTLPName: "datadog.resource.graphql.variables", Type: StringAttribute},
	"_dd.span_id":                     {OTLPName: "datadog._dd.span_id", Type: StringAttribute},
	"_dd.parent_span_id":              {OTLPName: "datadog._dd.parent_span_id", Type: StringAttribute},
	"_dd.trace_id":                    {OTLPName: "datadog._dd.trace_id", Type: StringAttribute},
	"_dd.rule_psr":                    {OTLPName: "datadog._dd.rule_psr", Type: NumberAttribute},
	"_dd.profiling.status":            {OTLPName: "datadog._dd.profiling.status", Type: StringAttribute},
	"_dd.profiling.error_reason":      {OTLPName: "datadog._dd.profiling.error_reason", Type: StringAttribute},

	// _view-schema.json
	"view.loading_time":                                         {OTLPName: "datadog.view.loading_time", Type: IntegerAttribute},
	"view.network_settled_time":                                 {OTLPName: "datadog.view.network_settled_time", Type: IntegerAttribute},
	"view.interaction_to_next_view_time":                        {OTLPName: "datadog.view.interaction_to_next_view_time", Type: IntegerAttribute},
	"view.loading_type":                                         {OTLPName: "datadog.view.loading_type", Type: StringAttribute},
	"view.time_spent":                                           {OTLPName: "datadog.view.time_spent", Type: IntegerAttribute},
	"view.first_contentful_paint":                               {OTLPName: "datadog.view.first_contentful_paint", Type: IntegerAttribute},
	"view.largest_contentful_paint":                             {OTLPName: "datadog.view.largest_contentful_paint", Type: IntegerAttribute},
	"view.largest_contentful_paint_target_selector":             {OTLPName: "datadog.view.largest_contentful_paint_target_selector", Type: StringAttribute},
	"view.first_input_delay":                                    {OTLPName: "datadog.view.first_input_delay", Type: IntegerAttribute},
	"view.first_input_time":                                     {OTLPName: "datadog.view.first_input_time", Type: IntegerAttribute},
	"view.first_input_target_selector":                          {OTLPName: "datadog.view.first_input_target_selector", Type: StringAttribute},
	"view.interaction_to_next_paint":                            {OTLPName: "datadog.view.interaction_to_next_paint", Type: IntegerAttribute},
	"view.interaction_to_next_paint_time":                       {OTLPName: "datadog.view.interaction_to_next_paint_time", Type: IntegerAttribute},
	"view.interaction_to_next_paint_target_selector":            {OTLPName: "datadog.view.interaction_to_next_paint_target_selector", Type: StringAttribute},
	"view.cumulative_layout_shift":                              {OTLPName: "datadog.view.cumulative_layout_shift", Type: NumberAttribute},
	"view.cumulative_layout_shift_time":                         {OTLPName: "datadog.view.cumulative_layout_shift_time", Type: IntegerAttribute},
	"view.cumulative_layout_shift_target_selector":              {OTLPName: "datadog.view.cumulative_layout_shift_target_selector", Type: StringAttribute},
	"view.dom_complete":                                         {OTLPName: "datadog.view.dom_complete", Type: IntegerAttribute},
	"view.dom_content_loaded":                                   {OTLPName: "datadog.view.dom_content_loaded", Type: IntegerAttribute},
	"view.dom_interactive":                                      {OTLPName: "datadog.view.dom_interactive", Type: IntegerAttribute},
	"view.load_event":                                           {OTLPName: "datadog.view.load_event", Type: IntegerAttribute},
	"view.first_byte":                                           {OTLPName: "datadog.view.first_byte", Type: IntegerAttribute},
	"view.custom_timings":                                       {OTLPName: "datadog.view.custom_timings", Type: ObjectAttribute},
	"view.is_active":                                            {OTLPName: "datadog.view.is_active", Type: BoolAttribute},
	"view.is_slow_rendered":                                     {OTLPName: "datadog.view.is_slow_rendered", Type: BoolAttribute},
	"view.action.count":                                         {OTLPName: "datadog.view.action.count", Type: IntegerAttribute},
	"view.error.count":                                          {OTLPName: "datadog.view.error.count", Type: IntegerAttribute},
	"view.crash.count":                                          {OTLPName: "datadog.view.crash.count", Type: IntegerAttribute},
	"view.long_task.count":                                      {OTLPName: "datadog.view.long_task.count", Type: IntegerAttribute},
	"view.frozen_frame.count":                                   {OTLPName: "datadog.view.frozen_frame.count", Type: IntegerAttribute},
	"view.slow_frames":                                          {OTLPName: "datadog.view.slow_frames", Type: ArrayAttribute},
	"view.slow_frames.start":                                    {OTLPName: "datadog.view.slow_frames.start", Type: IntegerAttribute},
	"view.slow_frames.duration":                                 {OTLPName: "datadog.view.slow_frames.duration", Type: IntegerAttribute},
	"view.resource.count":                                       {OTLPName: "datadog.view.resource.count", Type: IntegerAttribute},
	"view.frustration.count":                                    {OTLPName: "datadog.view.frustration.count", Type: IntegerAttribute},
	"view.in_foreground_periods":                                {OTLPName: "datadog.view.in_foreground_periods", Type: ArrayAttribute},
	"view.in_foreground_periods.start":                          {OTLPName: "datadog.view.in_foreground_periods.start", Type: IntegerAttribute},
	"view.in_foreground_periods.duration":                       {OTLPName: "datadog.view.in_foreground_periods.duration", Type: IntegerAttribute},
	"view.memory_average":                                       {OTLPName: "datadog.view.memory_average", Type: NumberAttribute},
	"view.memory_max":                                           {OTLPName: "datadog.view.memory_max", Type: NumberAttribute},
	"view.cpu_ticks_count":                                      {OTLPName: "datadog.view.cpu_ticks_count", Type: NumberAttribute},
	"view.cpu_ticks_per_second":                                 {OTLPName: "datadog.view.cpu_ticks_per_second", Type: NumberAttribute},
	"view.refresh_rate_average":                                 {OTLPName: "datadog.view.refresh_rate_average", Type: NumberAttribute},
	"view.refresh_rate_min":                                     {OTLPName: "datadog.view.refresh_rate_min", Type: NumberAttribute},
	"view.slow_frames_rate":                                     {OTLPName: "datadog.view.slow_frames_rate", Type: NumberAttribute},
	"view.freeze_rate":                                          {OTLPName: "datadog.view.freeze_rate", Type: NumberAttribute},
	"view.flutter_build_time.min":                               {OTLPName: "datadog.view.flutter_build_time.min", Type: NumberAttribute},
	"view.flutter_build_time.max":                               {OTLPName: "datadog.view.flutter_build_time.max", Type: NumberAttribute},
	"view.flutter_build_time.average":                           {OTLPName: "datadog.view.flutter_build_time.average", Type: NumberAttribute},
	"view.flutter_build_time.metric_max":                        {OTLPName: "datadog.view.flutter_build_time.metric_max", Type: NumberAttribute},
	"view.flutter_raster_time.min":                              {OTLPName: "datadog.view.flutter_raster_time.min", Type: NumberAttribute},
	"view.flutter_raster_time.max":                              {OTLPName: "datadog.view.flutter_raster_time.max", Type: NumberAttribute},
	"view.flutter_raster_time.average":                          {OTLPName: "datadog.view.flutter_raster_time.average", Type: NumberAttribute},
	"view.flutter_raster_time.metric_max":                       {OTLPName: "datadog.view.flutter_raster_time.metric_max", Type: NumberAttribute},
	"view.js_refresh_rate.min":                                  {OTLPName: "datadog.view.js_refresh_rate.min", Type: NumberAttribute},
	"view.js_refresh_rate.max":                                  {OTLPName: "datadog.view.js_refresh_rate.max", Type: NumberAttribute},
	"view.js_refresh_rate.average":                              {OTLPName: "datadog.view.js_refresh_rate.average", Type: NumberAttribute},
	"view.js_refresh_rate.metric_max":                           {OTLPName: "datadog.view.js_refresh_rate.metric_max", Type: NumberAttribute},
	"view.performance.cls.score":                                {OTLPName: "datadog.view.performance.cls.score", Type: NumberAttribute},
	"view.performance.cls.timestamp":                            {OTLPName: "datadog.view.performance.cls.timestamp", Type: IntegerAttribute},
	"view.performance.cls.target_selector":                      {OTLPName: "datadog.view.performance.cls.target_selector", Type: StringAttribute},
	"view.performance.cls.previous_rect.x":                      {OTLPName: "datadog.view.performance.cls.previous_rect.x", Type: NumberAttribute},
	"view.performance.cls.previous_rect.y":                      {OTLPName: "datadog.view.performance.cls.previous_rect.y", Type: NumberAttribute},
	"view.performance.cls.previous_rect.width":                  {OTLPName: "datadog.view.performance.cls.previous_rect.width", Type: NumberAttribute},
	"view.performance.cls.previous_rect.height":                 {OTLPName: "datadog.view.performance.cls.previous_rect.height", Type: NumberAttribute},
	"view.performance.cls.current_rect.x":                       {OTLPName: "datadog.view.performance.cls.current_rect.x", Type: NumberAttribute},
	"view.performance.cls.current_rect.y":                       {OTLPName: "datadog.view.performance.cls.current_rect.y", Type: NumberAttribute},
	"view.performance.cls.current_rect.width":                   {OTLPName: "datadog.view.performance.cls.current_rect.width", Type: NumberAttribute},
	"view.performance.cls.current_rect.height":                  {OTLPName: "datadog.view.performance.cls.current_rect.height", Type: NumberAttribute},
	"view.performance.fcp.timestamp":                            {OTLPName: "datadog.view.performance.fcp.timestamp", Type: IntegerAttribute},
	"view.performance.fid.duration":                             {OTLPName: "datadog.view.performance.fid.duration", Type: IntegerAttribute},
	"view.performance.fid.timestamp":                            {OTLPName: "datadog.view.performance.fid.timestamp", Type: IntegerAttribute},
	"view.performance.fid.target_selector":                      {OTLPName: "datadog.view.performance.fid.target_selector", Type: StringAttribute},
	"view.performance.inp.duration":                             {OTLPName: "datadog.view.performance.inp.duration", Type: IntegerAttribute},
	"view.performance.inp.timestamp":                            {OTLPName: "datadog.view.performance.inp.timestamp", Type: IntegerAttribute},
	"view.performance.inp.target_selector":                      {OTLPName: "datadog.view.performance.inp.target_selector", Type: StringAttribute},
	"view.performance.lcp.timestamp":                            {OTLPName: "datadog.view.performance.lcp.timestamp", Type: IntegerAttribute},
	"view.performance.lcp.target_selector":                      {OTLPName: "datadog.view.performance.lcp.target_selector", Type: StringAttribute},
	"view.performance.lcp.resource_url":                         {OTLPName: "datadog.view.performance.lcp.resource_url", Type: StringAttribute},
	"view.performance.fbc.timestamp":                            {OTLPName: "datadog.view.performance.fbc.timestamp", Type: IntegerAttribute},
	"session.is_active":                                         {OTLPName: "datadog.session.is_active", Type: BoolAttribute},
	"session.sampled_for_replay":                                {OTLPName: "datadog.session.sampled_for_replay", Type: BoolAttribute},
	"privacy.replay_level":                                      {OTLPName: "datadog.privacy.replay_level", Type: StringAttribute},
	"_dd.document_version":                                      {OTLPName: "datadog._dd.document_version", Type: IntegerAttribute},
	"_dd.page_states":                                           {OTLPName: "datadog._dd.page_states", Type: ArrayAttribute},
	"_dd.page_states.state":                                     {OTLPName: "datadog._dd.page_states.state", Type: StringAttribute},
	"_dd.page_states.start":                                     {OTLPName: "datadog._dd.page_states.start", Type: IntegerAttribute},
	"_dd.replay_stats.records_count":                            {OTLPName: "datadog._dd.replay_stats.records_count", Type: IntegerAttribute},
	"_dd.replay_stats.segments_count":                           {OTLPName: "datadog._dd.replay_stats.segments_count", Type: IntegerAttribute},
	"_dd.replay_stats.segments_total_raw_size":                  {OTLPName: "datadog._dd.replay_stats.segments_total_raw_size", Type: IntegerAttribute},
	"_dd.cls.device_pixel_ratio":                                {OTLPName: "datadog._dd.cls.device_pixel_ratio", Type: NumberAttribute},
	"_dd.configuration.start_session_replay_recording_manually": {OTLPName: "datadog._dd.configuration.start_session_replay_recording_manually", Type: BoolAttribute},
	"display.scroll.max_depth":                                  {OTLPName: "datadog.display.scroll.max_depth", Type: NumberAttribute},
	"display.scroll.max_depth_scroll_top":                       {OTLPName: "datadog.display.scroll.max_depth_scroll_top", Type: NumberAttribute},
	"display.scroll.max_scroll_height":                          {OTLPName: "datadog.display.scroll.max_scroll_height", Type: NumberAttribute},
	"display.scroll.max_scroll_height_time":                     {OTLPName: "datadog.display.scroll.max_scroll_height_time", Type: NumberAttribute},

	// vitals-schema.json
	"vital.type":               {OTLPName: "datadog.vital.type", Type: StringAttribute},
	"vital.id":                 {OTLPName: "datadog.vital.id", Type: StringAttribute},
	"vital.name":               {OTLPName: "datadog.vital.name", Type: StringAttribute},
	"vital.description":        {OTLPName: "datadog.vital.description", Type: StringAttribute},
	"vital.duration":           {OTLPName: "datadog.vital.duration", Type: NumberAttribute},
	"vital.custom":             {OTLPName: "datadog.vital.custom", Type: ObjectAttribute},
	"_dd.vital.computed_value": {OTLPName: "datadog._dd.vital.computed_value", Type: BoolAttribute},
}

func flattenJSON(payload map[string]any) map[string]any {
	flat := make(map[string]any)
	var recurse func(map[string]any, string)
	recurse = func(m map[string]any, prefix string) {
		for k, v := range m {
			fullKey := k
			if prefix != "" {
				fullKey = prefix + "." + k
			}
			if nested, ok := v.(map[string]any); ok {
				recurse(nested, fullKey)
			} else {
				flat[fullKey] = v
			}
		}
	}
	recurse(payload, "")
	return flat
}

func setAttributes(flatPayload map[string]any, attributes pcommon.Map) {
	for rumKey, meta := range attributeMetaMap {
		val, exists := flatPayload[rumKey]
		if !exists {
			continue
		}

		switch meta.Type {
		case StringAttribute:
			if s, ok := val.(string); ok {
				attributes.PutStr(meta.OTLPName, s)
			}
		case BoolAttribute:
			if b, ok := val.(bool); ok {
				attributes.PutBool(meta.OTLPName, b)
			}
		case NumberAttribute:
			if f, ok := val.(float64); ok {
				attributes.PutDouble(meta.OTLPName, f)
			}
		case IntegerAttribute:
			if i, ok := val.(int64); ok {
				attributes.PutInt(meta.OTLPName, i)
			} else if f, ok := val.(float64); ok {
				i := int64(f)
				attributes.PutInt(meta.OTLPName, i)
			}
		case ObjectAttribute:
			if o, ok := val.(map[string]any); ok {
				objVal := attributes.PutEmptyMap(meta.OTLPName)
				for k, v := range o {
					objVal.PutStr(k, fmt.Sprintf("%v", v))
				}
			}
		case ArrayAttribute:
			if a, ok := val.([]any); ok {
				arrVal := attributes.PutEmptySlice(meta.OTLPName)
				for _, v := range a {
					arrVal.AppendEmpty().SetStr(fmt.Sprintf("%v", v))
				}
			}
		case StringOrArrayAttribute:
			if s, ok := val.(string); ok {
				attributes.PutStr(meta.OTLPName, s)
			} else if a, ok := val.([]any); ok {
				arrVal := attributes.PutEmptySlice(meta.OTLPName)
				for _, v := range a {
					arrVal.AppendEmpty().SetStr(fmt.Sprintf("%v", v))
				}
			}
		}
	}
}

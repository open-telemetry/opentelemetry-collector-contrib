// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2018 Uber Technologies, Inc.
// SPDX-License-Identifier: Apache-2.0

package dbmodel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"strings"

	"github.com/jaegertracing/jaeger/model"
)

// newFromDomain creates fromDomain used to convert model span to db span
func newFromDomain(allTagsAsObject bool, tagKeysAsFields []string, tagDotReplacement string) fromDomain {
	tags := map[string]bool{}
	for _, k := range tagKeysAsFields {
		tags[k] = true
	}
	return fromDomain{allTagsAsFields: allTagsAsObject, tagKeysAsFields: tags, tagDotReplacement: tagDotReplacement}
}

// fromDomain is used to convert model span to db span
type fromDomain struct {
	allTagsAsFields   bool
	tagKeysAsFields   map[string]bool
	tagDotReplacement string
}

// fromDomainEmbedProcess converts model.span into json.span format.
// This format includes a ParentSpanID and an embedded process.
func (fd fromDomain) fromDomainEmbedProcess(span *model.Span) *LogzioSpan {
	return fd.convertSpanEmbedProcess(span)
}

func (fd fromDomain) convertSpanInternal(span *model.Span) LogzioSpan {
	tags, tagsMap := fd.convertKeyValuesString(span.Tags)
	return LogzioSpan{
		TraceID:         TraceID(span.TraceID.String()),
		SpanID:          SpanID(span.SpanID.String()),
		Flags:           uint32(span.Flags),
		OperationName:   span.OperationName,
		StartTime:       model.TimeAsEpochMicroseconds(span.StartTime),
		StartTimeMillis: model.TimeAsEpochMicroseconds(span.StartTime) / 1000,
		Duration:        model.DurationAsMicroseconds(span.Duration),
		Tags:            tags,
		Tag:             tagsMap,
		Logs:            fd.convertLogs(span.Logs),
	}
}

func (fd fromDomain) convertSpanEmbedProcess(span *model.Span) *LogzioSpan {
	s := fd.convertSpanInternal(span)
	s.Process = fd.convertProcess(span.Process)
	s.References = fd.convertReferences(span)
	return &s
}

func (fd fromDomain) convertReferences(span *model.Span) []Reference {
	out := make([]Reference, 0, len(span.References))
	for _, ref := range span.References {
		out = append(out, Reference{
			RefType: fd.convertRefType(ref.RefType),
			TraceID: TraceID(ref.TraceID.String()),
			SpanID:  SpanID(ref.SpanID.String()),
		})
	}
	return out
}

func (fromDomain) convertRefType(refType model.SpanRefType) ReferenceType {
	if refType == model.FollowsFrom {
		return FollowsFrom
	}
	return ChildOf
}

func (fd fromDomain) convertKeyValuesString(keyValues model.KeyValues) ([]KeyValue, map[string]any) {
	var tagsMap map[string]any
	var kvs []KeyValue
	for _, kv := range keyValues {
		if kv.GetVType() != model.BinaryType && (fd.allTagsAsFields || fd.tagKeysAsFields[kv.Key]) {
			if tagsMap == nil {
				tagsMap = map[string]any{}
			}
			tagsMap[strings.ReplaceAll(kv.Key, ".", fd.tagDotReplacement)] = kv.Value()
		} else {
			kvs = append(kvs, convertKeyValue(kv))
		}
	}
	if kvs == nil {
		kvs = make([]KeyValue, 0)
	}
	return kvs, tagsMap
}

func (fromDomain) convertLogs(logs []model.Log) []Log {
	out := make([]Log, len(logs))
	for i, log := range logs {
		var kvs []KeyValue
		for _, kv := range log.Fields {
			kvs = append(kvs, convertKeyValue(kv))
		}
		out[i] = Log{
			Timestamp: model.TimeAsEpochMicroseconds(log.Timestamp),
			Fields:    kvs,
		}
	}
	return out
}

func (fd fromDomain) convertProcess(process *model.Process) Process {
	tags, tagsMap := fd.convertKeyValuesString(process.Tags)
	return Process{
		ServiceName: process.ServiceName,
		Tags:        tags,
		Tag:         tagsMap,
	}
}

func convertKeyValue(kv model.KeyValue) KeyValue {
	return KeyValue{
		Key:   kv.Key,
		Type:  ValueType(strings.ToLower(kv.VType.String())),
		Value: kv.AsString(),
	}
}

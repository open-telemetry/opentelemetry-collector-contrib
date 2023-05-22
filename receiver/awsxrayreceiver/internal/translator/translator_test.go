// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

type perSpanProperties struct {
	traceID      string
	spanID       string
	parentSpanID *string
	name         string
	startTimeSec float64
	endTimeSec   *float64
	spanKind     ptrace.SpanKind
	spanStatus   spanSt
	eventsProps  []eventProps
	attrs        pcommon.Map
}

type spanSt struct {
	message string
	code    ptrace.StatusCode
}

type eventProps struct {
	name  string
	attrs pcommon.Map
}

func TestTranslation(t *testing.T) {
	var defaultServerSpanAttrs = func(seg *awsxray.Segment) pcommon.Map {
		m := pcommon.NewMap()
		assert.NoError(t, m.FromRaw(map[string]interface{}{
			conventions.AttributeHTTPMethod:       *seg.HTTP.Request.Method,
			conventions.AttributeHTTPClientIP:     *seg.HTTP.Request.ClientIP,
			conventions.AttributeHTTPUserAgent:    *seg.HTTP.Request.UserAgent,
			awsxray.AWSXRayXForwardedForAttribute: *seg.HTTP.Request.XForwardedFor,
			conventions.AttributeHTTPStatusCode:   *seg.HTTP.Response.Status,
			conventions.AttributeHTTPURL:          *seg.HTTP.Request.URL,
		}))
		return m
	}

	tests := []struct {
		testCase                  string
		expectedUnmarshallFailure bool
		samplePath                string
		expectedResourceAttrs     func(seg *awsxray.Segment) map[string]interface{}
		expectedRecord            xray.TelemetryRecord
		propsPerSpan              func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties
		verification              func(testCase string,
			actualSeg *awsxray.Segment,
			expectedRs ptrace.ResourceSpans,
			actualTraces ptrace.Traces,
			err error)
	}{
		{
			testCase:   "TranslateInstrumentedServerSegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "serverSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:          *seg.Name,
					conventions.AttributeCloudProvider:        conventions.AttributeCloudProviderAWS,
					conventions.AttributeTelemetrySDKVersion:  *seg.AWS.XRay.SDKVersion,
					conventions.AttributeTelemetrySDKName:     *seg.AWS.XRay.SDK,
					conventions.AttributeTelemetrySDKLanguage: "Go",
					conventions.AttributeK8SClusterName:       *seg.AWS.EKS.ClusterName,
					conventions.AttributeK8SPodName:           *seg.AWS.EKS.Pod,
					conventions.AttributeContainerID:          *seg.AWS.EKS.ContainerID,
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindServer,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateInstrumentedClientSegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "ddbSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:          *seg.Name,
					conventions.AttributeCloudProvider:        conventions.AttributeCloudProviderAWS,
					conventions.AttributeTelemetrySDKVersion:  *seg.AWS.XRay.SDKVersion,
					conventions.AttributeTelemetrySDKName:     *seg.AWS.XRay.SDK,
					conventions.AttributeTelemetrySDKLanguage: "java",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(18),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties {
				rootSpanAttrs := pcommon.NewMap()
				rootSpanAttrs.PutStr(conventions.AttributeEnduserID, *seg.User)
				rootSpanEvts := initExceptionEvents(seg)
				assert.Len(t, rootSpanEvts, 1, testCase+": rootSpanEvts has incorrect size")
				rootSpan := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindServer,
					spanStatus: spanSt{
						code: ptrace.StatusCodeError,
					},
					eventsProps: rootSpanEvts,
					attrs:       rootSpanAttrs,
				}

				// this is the subsegment with ID that starts with 7df6
				subseg7df6 := seg.Subsegments[0]
				childSpan7df6Attrs := pcommon.NewMap()
				childKeys := childSpan7df6Attrs.PutEmptySlice(awsxray.AWSXraySegmentAnnotationsAttribute)
				for k, v := range subseg7df6.Annotations {
					childSpan7df6Attrs.PutStr(k, v.(string))
					childKeys.AppendEmpty().SetStr(k)
				}
				for k, v := range subseg7df6.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan7df6Attrs.PutStr(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				assert.Equal(t, 3, childSpan7df6Attrs.Len(), testCase+": childSpan7df6Attrs has incorrect size")
				childSpan7df6Evts := initExceptionEvents(&subseg7df6)
				assert.Len(t, childSpan7df6Evts, 1, testCase+": childSpan7df6Evts has incorrect size")
				childSpan7df6 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7df6.ID,
					parentSpanID: &rootSpan.spanID,
					name:         *subseg7df6.Name,
					startTimeSec: *subseg7df6.StartTime,
					endTimeSec:   subseg7df6.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeError,
					},
					eventsProps: childSpan7df6Evts,
					attrs:       childSpan7df6Attrs,
				}

				subseg7318 := seg.Subsegments[0].Subsegments[0]
				childSpan7318Attrs := pcommon.NewMap()
				assert.NoError(t, childSpan7318Attrs.FromRaw(map[string]interface{}{
					awsxray.AWSServiceAttribute:                    *subseg7318.Name,
					conventions.AttributeHTTPResponseContentLength: int64(subseg7318.HTTP.Response.ContentLength.(float64)),
					conventions.AttributeHTTPStatusCode:            *subseg7318.HTTP.Response.Status,
					awsxray.AWSOperationAttribute:                  *subseg7318.AWS.Operation,
					awsxray.AWSRegionAttribute:                     *subseg7318.AWS.RemoteRegion,
					awsxray.AWSRequestIDAttribute:                  *subseg7318.AWS.RequestID,
					awsxray.AWSTableNameAttribute:                  *subseg7318.AWS.TableName,
					awsxray.AWSXrayRetriesAttribute:                *subseg7318.AWS.Retries,
				}))

				childSpan7318 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7318.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7318.Name,
					startTimeSec: *subseg7318.StartTime,
					endTimeSec:   subseg7318.EndTime,
					spanKind:     ptrace.SpanKindClient,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpan7318Attrs,
				}

				subseg0239 := seg.Subsegments[0].Subsegments[0].Subsegments[0]
				childSpan0239 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg0239.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg0239.Name,
					startTimeSec: *subseg0239.StartTime,
					endTimeSec:   subseg0239.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subseg23cf := seg.Subsegments[0].Subsegments[0].Subsegments[1]
				childSpan23cf := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg23cf.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg23cf.Name,
					startTimeSec: *subseg23cf.StartTime,
					endTimeSec:   subseg23cf.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subseg417b := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0]
				childSpan417bAttrs := pcommon.NewMap()
				for k, v := range subseg417b.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan417bAttrs.PutStr(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpan417b := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg417b.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subseg417b.Name,
					startTimeSec: *subseg417b.StartTime,
					endTimeSec:   subseg417b.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpan417bAttrs,
				}

				subseg0cab := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[0]
				childSpan0cabAttrs := pcommon.NewMap()
				for k, v := range subseg0cab.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan0cabAttrs.PutStr(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpan0cab := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg0cab.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subseg0cab.Name,
					startTimeSec: *subseg0cab.StartTime,
					endTimeSec:   subseg0cab.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpan0cabAttrs,
				}

				subsegF8db := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[1]
				childSpanF8dbAttrs := pcommon.NewMap()
				for k, v := range subsegF8db.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanF8dbAttrs.PutStr(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpanF8db := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegF8db.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegF8db.Name,
					startTimeSec: *subsegF8db.StartTime,
					endTimeSec:   subsegF8db.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpanF8dbAttrs,
				}

				subsegE2de := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[2]
				childSpanE2deAttrs := pcommon.NewMap()
				for k, v := range subsegE2de.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanE2deAttrs.PutStr(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpanE2de := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegE2de.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegE2de.Name,
					startTimeSec: *subsegE2de.StartTime,
					endTimeSec:   subsegE2de.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpanE2deAttrs,
				}

				subsegA70b := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[1]
				childSpanA70b := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegA70b.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subsegA70b.Name,
					startTimeSec: *subsegA70b.StartTime,
					endTimeSec:   subsegA70b.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subsegC053 := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[2]
				childSpanC053 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegC053.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subsegC053.Name,
					startTimeSec: *subsegC053.StartTime,
					endTimeSec:   subsegC053.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subseg5fca := seg.Subsegments[0].Subsegments[0].Subsegments[2]
				childSpan5fca := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg5fca.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg5fca.Name,
					startTimeSec: *subseg5fca.StartTime,
					endTimeSec:   subseg5fca.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subseg7163 := seg.Subsegments[0].Subsegments[1]
				childSpan7163Attrs := pcommon.NewMap()
				assert.NoError(t, childSpan7163Attrs.FromRaw(map[string]interface{}{
					awsxray.AWSServiceAttribute:                    *subseg7163.Name,
					conventions.AttributeHTTPStatusCode:            *subseg7163.HTTP.Response.Status,
					conventions.AttributeHTTPResponseContentLength: int64(subseg7163.HTTP.Response.ContentLength.(float64)),
					awsxray.AWSOperationAttribute:                  *subseg7163.AWS.Operation,
					awsxray.AWSRegionAttribute:                     *subseg7163.AWS.RemoteRegion,
					awsxray.AWSRequestIDAttribute:                  *subseg7163.AWS.RequestID,
					awsxray.AWSTableNameAttribute:                  *subseg7163.AWS.TableName,
					awsxray.AWSXrayRetriesAttribute:                *subseg7163.AWS.Retries,
				}))

				childSpan7163Evts := initExceptionEvents(&subseg7163)
				assert.Len(t, childSpan7163Evts, 1, testCase+": childSpan7163Evts has incorrect size")
				childSpan7163 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7163.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7163.Name,
					startTimeSec: *subseg7163.StartTime,
					endTimeSec:   subseg7163.EndTime,
					spanKind:     ptrace.SpanKindClient,
					spanStatus: spanSt{
						code: ptrace.StatusCodeError,
					},
					eventsProps: childSpan7163Evts,
					attrs:       childSpan7163Attrs,
				}

				subseg9da0 := seg.Subsegments[0].Subsegments[1].Subsegments[0]
				childSpan9da0 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg9da0.ID,
					parentSpanID: &childSpan7163.spanID,
					name:         *subseg9da0.Name,
					startTimeSec: *subseg9da0.StartTime,
					endTimeSec:   subseg9da0.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subseg56b1 := seg.Subsegments[0].Subsegments[1].Subsegments[1]
				childSpan56b1Evts := initExceptionEvents(&subseg56b1)
				assert.Len(t, childSpan56b1Evts, 1, testCase+": childSpan56b1Evts has incorrect size")
				childSpan56b1 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg56b1.ID,
					parentSpanID: &childSpan7163.spanID,
					name:         *subseg56b1.Name,
					startTimeSec: *subseg56b1.StartTime,
					endTimeSec:   subseg56b1.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeError,
					},
					eventsProps: childSpan56b1Evts,
					attrs:       pcommon.NewMap(),
				}

				subseg6f90 := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[0]
				childSpan6f90 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg6f90.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subseg6f90.Name,
					startTimeSec: *subseg6f90.StartTime,
					endTimeSec:   subseg6f90.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subsegAcfa := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[1]
				childSpanAcfa := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegAcfa.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subsegAcfa.Name,
					startTimeSec: *subsegAcfa.StartTime,
					endTimeSec:   subsegAcfa.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pcommon.NewMap(),
				}

				subsegBa8d := seg.Subsegments[0].Subsegments[1].Subsegments[2]
				childSpanBa8dEvts := initExceptionEvents(&subsegBa8d)
				assert.Len(t, childSpanBa8dEvts, 1, testCase+": childSpanBa8dEvts has incorrect size")
				childSpanBa8d := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegBa8d.ID,
					parentSpanID: &childSpan7163.spanID,
					name:         *subsegBa8d.Name,
					startTimeSec: *subsegBa8d.StartTime,
					endTimeSec:   subsegBa8d.EndTime,
					spanKind:     ptrace.SpanKindInternal,
					spanStatus: spanSt{
						code: ptrace.StatusCodeError,
					},
					eventsProps: childSpanBa8dEvts,
					attrs:       pcommon.NewMap(),
				}

				return []perSpanProperties{rootSpan,
					childSpan7df6,
					childSpan7318,
					childSpan0239,
					childSpan23cf,
					childSpan417b,
					childSpan0cab,
					childSpanF8db,
					childSpanE2de,
					childSpanA70b,
					childSpanC053,
					childSpan5fca,
					childSpan7163,
					childSpan9da0,
					childSpan56b1,
					childSpan6f90,
					childSpanAcfa,
					childSpanBa8d,
				}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					"one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "[aws] TranslateMissingAWSFieldSegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "awsMissingAwsField.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:   *seg.Name,
					conventions.AttributeCloudProvider: "unknown",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindServer,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "[aws] TranslateEC2AWSFieldsSegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "awsValidAwsFields.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:           *seg.Name,
					conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderAWS,
					conventions.AttributeCloudAccountID:        *seg.AWS.AccountID,
					conventions.AttributeCloudAvailabilityZone: *seg.AWS.EC2.AvailabilityZone,
					conventions.AttributeHostID:                *seg.AWS.EC2.InstanceID,
					conventions.AttributeHostType:              *seg.AWS.EC2.InstanceSize,
					conventions.AttributeHostImageID:           *seg.AWS.EC2.AmiID,
					conventions.AttributeContainerName:         *seg.AWS.ECS.ContainerName,
					conventions.AttributeContainerID:           *seg.AWS.ECS.ContainerID,
					conventions.AttributeServiceNamespace:      *seg.AWS.Beanstalk.Environment,
					conventions.AttributeServiceInstanceID:     "32",
					conventions.AttributeServiceVersion:        *seg.AWS.Beanstalk.VersionLabel,
					conventions.AttributeTelemetrySDKVersion:   *seg.AWS.XRay.SDKVersion,
					conventions.AttributeTelemetrySDKName:      *seg.AWS.XRay.SDK,
					conventions.AttributeTelemetrySDKLanguage:  "Go",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				attrs.PutStr(awsxray.AWSAccountAttribute, *seg.AWS.AccountID)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindServer,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateCauseIsExceptionId",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "minCauseIsExceptionId.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:   *seg.Name,
					conventions.AttributeCloudProvider: "unknown",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindServer,
					spanStatus: spanSt{
						message: *seg.Cause.ExceptionID,
						code:    ptrace.StatusCodeError,
					},
					attrs: pcommon.NewMap(),
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateInvalidNamespace",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "invalidNamespace.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return nil
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(18),
				SegmentsRejectedCount: aws.Int64(18),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.EqualError(t, err,
					fmt.Sprintf("unexpected namespace: %s",
						*actualSeg.Subsegments[0].Subsegments[0].Namespace),
					testCase+": translation should've failed")
			},
		},
		{
			testCase:   "TranslateIndepSubsegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegment.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:   *seg.Name,
					conventions.AttributeCloudProvider: "unknown",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pcommon.NewMap()
				assert.NoError(t, attrs.FromRaw(map[string]interface{}{
					conventions.AttributeHTTPMethod:                *seg.HTTP.Request.Method,
					conventions.AttributeHTTPStatusCode:            *seg.HTTP.Response.Status,
					conventions.AttributeHTTPURL:                   *seg.HTTP.Request.URL,
					conventions.AttributeHTTPResponseContentLength: int64(seg.HTTP.Response.ContentLength.(float64)),
					awsxray.AWSXRayTracedAttribute:                 true,
				}))
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindClient,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateIndepSubsegmentForContentLengthString",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithContentLengthString.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:   *seg.Name,
					conventions.AttributeCloudProvider: "unknown",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pcommon.NewMap()
				assert.NoError(t, attrs.FromRaw(map[string]interface{}{
					conventions.AttributeHTTPMethod:                *seg.HTTP.Request.Method,
					conventions.AttributeHTTPStatusCode:            *seg.HTTP.Response.Status,
					conventions.AttributeHTTPURL:                   *seg.HTTP.Request.URL,
					conventions.AttributeHTTPResponseContentLength: seg.HTTP.Response.ContentLength.(string),
					awsxray.AWSXRayTracedAttribute:                 true,
				}))

				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindClient,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateSql",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithSql.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return map[string]interface{}{
					conventions.AttributeServiceName:   *seg.Name,
					conventions.AttributeCloudProvider: "unknown",
				}
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pcommon.NewMap()
				assert.NoError(t, attrs.FromRaw(map[string]interface{}{
					conventions.AttributeDBConnectionString: "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2." +
						"rds.amazonaws.com:5432",
					conventions.AttributeDBName:      "ebdb",
					conventions.AttributeDBSystem:    *seg.SQL.DatabaseType,
					conventions.AttributeDBStatement: *seg.SQL.SanitizedQuery,
					conventions.AttributeDBUser:      *seg.SQL.User,
				}))
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     ptrace.SpanKindClient,
					spanStatus: spanSt{
						code: ptrace.StatusCodeUnset,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				assert.NoError(t, ptracetest.CompareResourceSpans(expectedRs, actualRs))
			},
		},
		{
			testCase:   "TranslateInvalidSqlUrl",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithInvalidSqlUrl.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return nil
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(1),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.EqualError(t, err,
					fmt.Sprintf(
						"failed to parse out the database name in the \"sql.url\" field, rawUrl: %s",
						*actualSeg.SQL.URL,
					),
					testCase+": translation should've failed")
			},
		},
		{
			testCase:                  "TranslateJsonUnmarshallFailed",
			expectedUnmarshallFailure: true,
			samplePath:                filepath.Join("../../../../internal/aws/xray", "testdata", "minCauseIsInvalid.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return nil
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(0),
				SegmentsRejectedCount: aws.Int64(0),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.EqualError(t, err,
					fmt.Sprintf(
						"the value assigned to the `cause` field does not appear to be a string: %v",
						[]byte{'2', '0', '0'},
					),
					testCase+": translation should've failed")
			},
		},
		{
			testCase:   "TranslateRootSegValidationFailed",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "segmentValidationFailed.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]interface{} {
				return nil
			},
			expectedRecord: xray.TelemetryRecord{
				SegmentsReceivedCount: aws.Int64(1),
				SegmentsRejectedCount: aws.Int64(1),
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs ptrace.ResourceSpans, actualTraces ptrace.Traces, err error) {
				assert.EqualError(t, err, `segment "start_time" can not be nil`,
					testCase+": translation should've failed")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testCase, func(t *testing.T) {
			content, err := os.ReadFile(tc.samplePath)
			assert.NoError(t, err, "can not read raw segment")
			assert.True(t, len(content) > 0, "content length is 0")

			var (
				actualSeg  awsxray.Segment
				expectedRs ptrace.ResourceSpans
			)
			if !tc.expectedUnmarshallFailure {
				err = json.Unmarshal(content, &actualSeg)
				// the correctness of the actual segment
				// has been verified in the tracesegment_test.go
				assert.NoError(t, err, "failed to unmarhal raw segment")
				expectedRs = initResourceSpans(t,
					&actualSeg,
					tc.expectedResourceAttrs(&actualSeg),
					tc.propsPerSpan(tc.testCase, t, &actualSeg),
				)
			}

			recorder := telemetry.NewRecorder()
			traces, totalSpanCount, err := ToTraces(content, recorder)
			if err == nil || (!tc.expectedUnmarshallFailure && expectedRs.ScopeSpans().Len() > 0 && expectedRs.ScopeSpans().At(0).Spans().Len() > 0) {
				assert.Equal(t, totalSpanCount,
					expectedRs.ScopeSpans().At(0).Spans().Len(),
					"generated span count is different from the expected",
				)
			}
			tc.verification(tc.testCase, &actualSeg, expectedRs, traces, err)
			record := recorder.Rotate()
			assert.Equal(t, *tc.expectedRecord.SegmentsReceivedCount, *record.SegmentsReceivedCount)
			assert.Equal(t, *tc.expectedRecord.SegmentsRejectedCount, *record.SegmentsRejectedCount)
		})
	}
}

func initExceptionEvents(expectedSeg *awsxray.Segment) []eventProps {
	res := make([]eventProps, 0, len(expectedSeg.Cause.Exceptions))
	for _, excp := range expectedSeg.Cause.Exceptions {
		attrs := pcommon.NewMap()
		attrs.PutStr(awsxray.AWSXrayExceptionIDAttribute, *excp.ID)
		if excp.Message != nil {
			attrs.PutStr(conventions.AttributeExceptionMessage, *excp.Message)
		}

		if excp.Type != nil {
			attrs.PutStr(conventions.AttributeExceptionType, *excp.Type)
		}

		if excp.Remote != nil {
			attrs.PutBool(awsxray.AWSXrayExceptionRemoteAttribute, *excp.Remote)
		}

		if excp.Truncated != nil {
			attrs.PutInt(awsxray.AWSXrayExceptionTruncatedAttribute, *excp.Truncated)
		}

		if excp.Skipped != nil {
			attrs.PutInt(awsxray.AWSXrayExceptionSkippedAttribute, *excp.Skipped)
		}

		if excp.Cause != nil {
			attrs.PutStr(awsxray.AWSXrayExceptionCauseAttribute, *excp.Cause)
		}

		if len(excp.Stack) > 0 {
			attrs.PutStr(conventions.AttributeExceptionStacktrace, convertStackFramesToStackTraceStr(excp))
		}
		res = append(res, eventProps{
			name:  ExceptionEventName,
			attrs: attrs,
		})
	}
	return res
}

func initResourceSpans(t *testing.T, expectedSeg *awsxray.Segment,
	resourceAttrs map[string]interface{},
	propsPerSpan []perSpanProperties,
) ptrace.ResourceSpans {
	if expectedSeg == nil {
		return ptrace.ResourceSpans{}
	}

	rs := ptrace.NewResourceSpans()

	if len(resourceAttrs) > 0 {
		assert.NoError(t, rs.Resource().Attributes().FromRaw(resourceAttrs))
	} else {
		rs.Resource().Attributes().Clear()
		rs.Resource().Attributes().EnsureCapacity(initAttrCapacity)
	}

	if len(propsPerSpan) == 0 {
		return rs
	}

	ls := rs.ScopeSpans().AppendEmpty()
	ls.Spans().EnsureCapacity(len(propsPerSpan))

	for _, props := range propsPerSpan {
		sp := ls.Spans().AppendEmpty()
		spanIDBytes, _ := decodeXRaySpanID(&props.spanID)
		sp.SetSpanID(spanIDBytes)
		if props.parentSpanID != nil {
			parentIDBytes, _ := decodeXRaySpanID(props.parentSpanID)
			sp.SetParentSpanID(parentIDBytes)
		}
		sp.SetName(props.name)
		sp.SetStartTimestamp(pcommon.Timestamp(props.startTimeSec * float64(time.Second)))
		if props.endTimeSec != nil {
			sp.SetEndTimestamp(pcommon.Timestamp(*props.endTimeSec * float64(time.Second)))
		}
		sp.SetKind(props.spanKind)
		traceIDBytes, _ := decodeXRayTraceID(&props.traceID)
		sp.SetTraceID(traceIDBytes)
		sp.Status().SetMessage(props.spanStatus.message)
		sp.Status().SetCode(props.spanStatus.code)

		if len(props.eventsProps) > 0 {
			sp.Events().EnsureCapacity(len(props.eventsProps))
			for _, evtProps := range props.eventsProps {
				spEvt := sp.Events().AppendEmpty()
				spEvt.SetName(evtProps.name)
				evtProps.attrs.CopyTo(spEvt.Attributes())
			}
		}

		if props.attrs.Len() > 0 {
			props.attrs.CopyTo(sp.Attributes())
		} else {
			sp.Attributes().Clear()
			sp.Attributes().EnsureCapacity(initAttrCapacity)
		}
	}
	return rs
}

func TestDecodeXRayTraceID(t *testing.T) {
	// normal
	traceID := "1-5f84c7a1-e7d1852db8c4fd35d88bf49a"
	traceIDBytes, err := decodeXRayTraceID(&traceID)
	expectedTraceIDBytes := [16]byte{0x5f, 0x84, 0xc7, 0xa1, 0xe7, 0xd1, 0x85, 0x2d, 0xb8, 0xc4, 0xfd, 0x35, 0xd8, 0x8b, 0xf4, 0x9a}
	if assert.NoError(t, err) {
		assert.Equal(t, traceIDBytes, expectedTraceIDBytes)
	}

	// invalid format
	traceID = "1-5f84c7a1-e7d1852db"
	_, err = decodeXRayTraceID(&traceID)
	assert.Error(t, err)

	// null point
	_, err = decodeXRayTraceID(nil)
	assert.Error(t, err)
}

func TestDecodeXRaySpanID(t *testing.T) {
	// normal
	spanID := "defdfd9912dc5a56"
	spanIDBytes, err := decodeXRaySpanID(&spanID)
	expectedSpanIDBytes := [8]byte{0xde, 0xfd, 0xfd, 0x99, 0x12, 0xdc, 0x5a, 0x56}
	if assert.NoError(t, err) {
		assert.Equal(t, spanIDBytes, expectedSpanIDBytes)
	}

	// invalid format
	spanID = "12345566"
	_, err = decodeXRaySpanID(&spanID)
	assert.Error(t, err)

	// null point
	_, err = decodeXRaySpanID(nil)
	assert.Error(t, err)

}

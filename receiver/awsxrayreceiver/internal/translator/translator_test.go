// Copyright The OpenTelemetry Authors
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

type perSpanProperties struct {
	traceID      string
	spanID       string
	parentSpanID *string
	name         string
	startTimeSec float64
	endTimeSec   *float64
	spanKind     pdata.SpanKind
	spanStatus   spanSt
	eventsProps  []eventProps
	attrs        map[string]pdata.AttributeValue
}

type spanSt struct {
	message string
	code    pdata.StatusCode
}

type eventProps struct {
	name  string
	attrs map[string]pdata.AttributeValue
}

func TestTranslation(t *testing.T) {
	var defaultServerSpanAttrs = func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
		attrs := make(map[string]pdata.AttributeValue)
		attrs[conventions.AttributeHTTPMethod] = pdata.NewAttributeValueString(
			*seg.HTTP.Request.Method)
		attrs[conventions.AttributeHTTPClientIP] = pdata.NewAttributeValueString(
			*seg.HTTP.Request.ClientIP)
		attrs[conventions.AttributeHTTPUserAgent] = pdata.NewAttributeValueString(
			*seg.HTTP.Request.UserAgent)
		attrs[awsxray.AWSXRayXForwardedForAttribute] = pdata.NewAttributeValueBool(
			*seg.HTTP.Request.XForwardedFor)
		attrs[conventions.AttributeHTTPStatusCode] = pdata.NewAttributeValueInt(
			*seg.HTTP.Response.Status)
		attrs[conventions.AttributeHTTPURL] = pdata.NewAttributeValueString(
			*seg.HTTP.Request.URL)

		return attrs
	}

	tests := []struct {
		testCase                  string
		expectedUnmarshallFailure bool
		samplePath                string
		expectedResourceAttrs     func(seg *awsxray.Segment) map[string]pdata.AttributeValue
		propsPerSpan              func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties
		verification              func(testCase string,
			actualSeg *awsxray.Segment,
			expectedRs *pdata.ResourceSpans,
			actualTraces *pdata.Traces,
			err error)
	}{
		{
			testCase:   "TranslateInstrumentedServerSegment",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "serverSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("aws")
				attrs[conventions.AttributeTelemetrySDKVersion] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDKVersion)
				attrs[conventions.AttributeTelemetrySDKName] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDK)
				attrs[conventions.AttributeTelemetrySDKLanguage] = pdata.NewAttributeValueString("Go")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindSERVER,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "TranslateInstrumentedClientSegment",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "ddbSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("aws")
				attrs[conventions.AttributeTelemetrySDKVersion] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDKVersion)
				attrs[conventions.AttributeTelemetrySDKName] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDK)
				attrs[conventions.AttributeTelemetrySDKLanguage] = pdata.NewAttributeValueString("java")

				return attrs
			},
			propsPerSpan: func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties {
				rootSpanAttrs := make(map[string]pdata.AttributeValue)
				rootSpanAttrs[conventions.AttributeEnduserID] = pdata.NewAttributeValueString(*seg.User)
				rootSpanEvts := initExceptionEvents(seg)
				assert.Len(t, rootSpanEvts, 1, testCase+": rootSpanEvts has incorrect size")
				rootSpan := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeInvalidArgument,
					},
					eventsProps: rootSpanEvts,
					attrs:       rootSpanAttrs,
				}

				// this is the subsegment with ID that starts with 7df6
				subseg7df6 := seg.Subsegments[0]
				childSpan7df6Attrs := make(map[string]pdata.AttributeValue)
				for k, v := range subseg7df6.Annotations {
					childSpan7df6Attrs[k] = pdata.NewAttributeValueString(
						v.(string))
				}
				for k, v := range subseg7df6.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan7df6Attrs[awsxray.AWSXraySegmentMetadataAttributePrefix+k] = pdata.NewAttributeValueString(
						string(m))
				}
				assert.Len(t, childSpan7df6Attrs, 2, testCase+": childSpan7df6Attrs has incorrect size")
				childSpan7df6Evts := initExceptionEvents(&subseg7df6)
				assert.Len(t, childSpan7df6Evts, 1, testCase+": childSpan7df6Evts has incorrect size")
				childSpan7df6 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7df6.ID,
					parentSpanID: &rootSpan.spanID,
					name:         *subseg7df6.Name,
					startTimeSec: *subseg7df6.StartTime,
					endTimeSec:   subseg7df6.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeInvalidArgument,
					},
					eventsProps: childSpan7df6Evts,
					attrs:       childSpan7df6Attrs,
				}

				subseg7318 := seg.Subsegments[0].Subsegments[0]
				childSpan7318Attrs := make(map[string]pdata.AttributeValue)
				childSpan7318Attrs[awsxray.AWSServiceAttribute] = pdata.NewAttributeValueString(
					*subseg7318.Name)
				childSpan7318Attrs[conventions.AttributeHTTPStatusCode] = pdata.NewAttributeValueInt(
					*subseg7318.HTTP.Response.Status)
				childSpan7318Attrs[conventions.AttributeHTTPResponseContentLength] = pdata.NewAttributeValueInt(
					*subseg7318.HTTP.Response.ContentLength)
				childSpan7318Attrs[awsxray.AWSOperationAttribute] = pdata.NewAttributeValueString(
					*subseg7318.AWS.Operation)
				childSpan7318Attrs[awsxray.AWSRegionAttribute] = pdata.NewAttributeValueString(
					*subseg7318.AWS.RemoteRegion)
				childSpan7318Attrs[awsxray.AWSRequestIDAttribute] = pdata.NewAttributeValueString(
					*subseg7318.AWS.RequestID)
				childSpan7318Attrs[awsxray.AWSTableNameAttribute] = pdata.NewAttributeValueString(
					*subseg7318.AWS.TableName)
				childSpan7318Attrs[awsxray.AWSXrayRetriesAttribute] = pdata.NewAttributeValueInt(
					*subseg7318.AWS.Retries)

				childSpan7318 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7318.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7318.Name,
					startTimeSec: *subseg7318.StartTime,
					endTimeSec:   subseg7318.EndTime,
					spanKind:     pdata.SpanKindCLIENT,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
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
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subseg23cf := seg.Subsegments[0].Subsegments[0].Subsegments[1]
				childSpan23cf := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg23cf.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg23cf.Name,
					startTimeSec: *subseg23cf.StartTime,
					endTimeSec:   subseg23cf.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subseg417b := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0]
				childSpan417bAttrs := make(map[string]pdata.AttributeValue)
				for k, v := range subseg417b.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan417bAttrs[awsxray.AWSXraySegmentMetadataAttributePrefix+k] = pdata.NewAttributeValueString(
						string(m))
				}
				childSpan417b := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg417b.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subseg417b.Name,
					startTimeSec: *subseg417b.StartTime,
					endTimeSec:   subseg417b.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       childSpan417bAttrs,
				}

				subseg0cab := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[0]
				childSpan0cabAttrs := make(map[string]pdata.AttributeValue)
				for k, v := range subseg0cab.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan0cabAttrs[awsxray.AWSXraySegmentMetadataAttributePrefix+k] = pdata.NewAttributeValueString(
						string(m))
				}
				childSpan0cab := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg0cab.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subseg0cab.Name,
					startTimeSec: *subseg0cab.StartTime,
					endTimeSec:   subseg0cab.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       childSpan0cabAttrs,
				}

				subsegF8db := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[1]
				childSpanF8dbAttrs := make(map[string]pdata.AttributeValue)
				for k, v := range subsegF8db.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanF8dbAttrs[awsxray.AWSXraySegmentMetadataAttributePrefix+k] = pdata.NewAttributeValueString(
						string(m))
				}
				childSpanF8db := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegF8db.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegF8db.Name,
					startTimeSec: *subsegF8db.StartTime,
					endTimeSec:   subsegF8db.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       childSpanF8dbAttrs,
				}

				subsegE2de := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[2]
				childSpanE2deAttrs := make(map[string]pdata.AttributeValue)
				for k, v := range subsegE2de.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanE2deAttrs[awsxray.AWSXraySegmentMetadataAttributePrefix+k] = pdata.NewAttributeValueString(
						string(m))
				}
				childSpanE2de := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegE2de.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegE2de.Name,
					startTimeSec: *subsegE2de.StartTime,
					endTimeSec:   subsegE2de.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
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
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subsegC053 := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[2]
				childSpanC053 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegC053.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subsegC053.Name,
					startTimeSec: *subsegC053.StartTime,
					endTimeSec:   subsegC053.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subseg5fca := seg.Subsegments[0].Subsegments[0].Subsegments[2]
				childSpan5fca := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg5fca.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg5fca.Name,
					startTimeSec: *subseg5fca.StartTime,
					endTimeSec:   subseg5fca.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subseg7163 := seg.Subsegments[0].Subsegments[1]
				childSpan7163Attrs := make(map[string]pdata.AttributeValue)
				childSpan7163Attrs[awsxray.AWSServiceAttribute] = pdata.NewAttributeValueString(
					*subseg7163.Name)
				childSpan7163Attrs[conventions.AttributeHTTPStatusCode] = pdata.NewAttributeValueInt(
					*subseg7163.HTTP.Response.Status)
				childSpan7163Attrs[conventions.AttributeHTTPResponseContentLength] = pdata.NewAttributeValueInt(
					*subseg7163.HTTP.Response.ContentLength)
				childSpan7163Attrs[awsxray.AWSOperationAttribute] = pdata.NewAttributeValueString(
					*subseg7163.AWS.Operation)
				childSpan7163Attrs[awsxray.AWSRegionAttribute] = pdata.NewAttributeValueString(
					*subseg7163.AWS.RemoteRegion)
				childSpan7163Attrs[awsxray.AWSRequestIDAttribute] = pdata.NewAttributeValueString(
					*subseg7163.AWS.RequestID)
				childSpan7163Attrs[awsxray.AWSTableNameAttribute] = pdata.NewAttributeValueString(
					*subseg7163.AWS.TableName)
				childSpan7163Attrs[awsxray.AWSXrayRetriesAttribute] = pdata.NewAttributeValueInt(
					*subseg7163.AWS.Retries)

				childSpan7163Evts := initExceptionEvents(&subseg7163)
				assert.Len(t, childSpan7163Evts, 1, testCase+": childSpan7163Evts has incorrect size")
				childSpan7163 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7163.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7163.Name,
					startTimeSec: *subseg7163.StartTime,
					endTimeSec:   subseg7163.EndTime,
					spanKind:     pdata.SpanKindCLIENT,
					spanStatus: spanSt{
						code: pdata.StatusCodeInvalidArgument,
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
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
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
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnknownError,
					},
					eventsProps: childSpan56b1Evts,
					attrs:       nil,
				}

				subseg6f90 := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[0]
				childSpan6f90 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg6f90.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subseg6f90.Name,
					startTimeSec: *subseg6f90.StartTime,
					endTimeSec:   subseg6f90.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
				}

				subsegAcfa := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[1]
				childSpanAcfa := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegAcfa.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subsegAcfa.Name,
					startTimeSec: *subsegAcfa.StartTime,
					endTimeSec:   subsegAcfa.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					eventsProps: nil,
					attrs:       nil,
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
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnknownError,
					},
					eventsProps: childSpanBa8dEvts,
					attrs:       nil,
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
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					"one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "[aws] TranslateMissingAWSFieldSegment",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "awsMissingAwsField.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindSERVER,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "[aws] TranslateEC2AWSFieldsSegment",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "awsValidAwsFields.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("aws")
				attrs[conventions.AttributeCloudAccount] = pdata.NewAttributeValueString(
					*seg.AWS.AccountID)
				attrs[conventions.AttributeCloudZone] = pdata.NewAttributeValueString(
					*seg.AWS.EC2.AvailabilityZone)
				attrs[conventions.AttributeHostID] = pdata.NewAttributeValueString(
					*seg.AWS.EC2.InstanceID)
				attrs[conventions.AttributeHostType] = pdata.NewAttributeValueString(
					*seg.AWS.EC2.InstanceSize)
				attrs[conventions.AttributeHostImageID] = pdata.NewAttributeValueString(
					*seg.AWS.EC2.AmiID)
				attrs[conventions.AttributeContainerName] = pdata.NewAttributeValueString(
					*seg.AWS.ECS.ContainerName)
				attrs[conventions.AttributeServiceNamespace] = pdata.NewAttributeValueString(
					*seg.AWS.Beanstalk.Environment)
				attrs[conventions.AttributeServiceInstance] = pdata.NewAttributeValueString(
					"32")
				attrs[conventions.AttributeServiceVersion] = pdata.NewAttributeValueString(
					*seg.AWS.Beanstalk.VersionLabel)
				attrs[conventions.AttributeTelemetrySDKVersion] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDKVersion)
				attrs[conventions.AttributeTelemetrySDKName] = pdata.NewAttributeValueString(
					*seg.AWS.XRay.SDK)
				attrs[conventions.AttributeTelemetrySDKLanguage] = pdata.NewAttributeValueString("Go")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				attrs[awsxray.AWSAccountAttribute] = pdata.NewAttributeValueString(
					*seg.AWS.AccountID)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindSERVER,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "TranslateCauseIsExceptionId",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "minCauseIsExceptionId.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindINTERNAL,
					spanStatus: spanSt{
						message: *seg.Cause.ExceptionID,
						code:    pdata.StatusCodeUnknownError,
					},
					attrs: nil,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "TranslateInvalidNamespace",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "invalidNamespace.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				return nil
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.EqualError(t, err,
					fmt.Sprintf("unexpected namespace: %s",
						*actualSeg.Subsegments[0].Subsegments[0].Namespace),
					testCase+": translation should've failed")
			},
		},
		{
			testCase:   "TranslateIndepSubsegment",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "indepSubsegment.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeHTTPMethod] = pdata.NewAttributeValueString(
					*seg.HTTP.Request.Method)
				attrs[conventions.AttributeHTTPStatusCode] = pdata.NewAttributeValueInt(
					*seg.HTTP.Response.Status)
				attrs[conventions.AttributeHTTPURL] = pdata.NewAttributeValueString(
					*seg.HTTP.Request.URL)
				attrs[conventions.AttributeHTTPResponseContentLength] = pdata.NewAttributeValueInt(
					*seg.HTTP.Response.ContentLength)
				attrs[awsxray.AWSXRayTracedAttribute] = pdata.NewAttributeValueBool(true)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindCLIENT,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "TranslateSql",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "indepSubsegmentWithSql.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeCloudProvider] = pdata.NewAttributeValueString("unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := make(map[string]pdata.AttributeValue)
				attrs[conventions.AttributeDBConnectionString] = pdata.NewAttributeValueString(
					"jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432")
				attrs[conventions.AttributeDBName] = pdata.NewAttributeValueString("ebdb")
				attrs[conventions.AttributeDBSystem] = pdata.NewAttributeValueString(
					*seg.SQL.DatabaseType)
				attrs[conventions.AttributeDBStatement] = pdata.NewAttributeValueString(
					*seg.SQL.SanitizedQuery)
				attrs[conventions.AttributeDBUser] = pdata.NewAttributeValueString(
					*seg.SQL.User)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindCLIENT,
					spanStatus: spanSt{
						code: pdata.StatusCodeOk,
					},
					attrs: attrs,
				}
				return []perSpanProperties{res}
			},
			verification: func(testCase string,
				_ *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.NoError(t, err, testCase+": translation should've succeeded")
				assert.Equal(t, 1, actualTraces.ResourceSpans().Len(),
					testCase+": one segment should translate to 1 ResourceSpans")

				actualRs := actualTraces.ResourceSpans().At(0)
				compare2ResourceSpans(t, testCase, expectedRs, &actualRs)
			},
		},
		{
			testCase:   "TranslateInvalidSqlUrl",
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "indepSubsegmentWithInvalidSqlUrl.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				return nil
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
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
			samplePath:                path.Join("../../../../internal/awsxray", "testdata", "minCauseIsInvalid.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				return nil
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
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
			samplePath: path.Join("../../../../internal/awsxray", "testdata", "segmentValidationFailed.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) map[string]pdata.AttributeValue {
				return nil
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				return nil
			},
			verification: func(testCase string,
				actualSeg *awsxray.Segment,
				expectedRs *pdata.ResourceSpans, actualTraces *pdata.Traces, err error) {
				assert.EqualError(t, err, `segment "start_time" can not be nil`,
					testCase+": translation should've failed")
			},
		},
	}

	for _, tc := range tests {
		content, err := ioutil.ReadFile(tc.samplePath)
		assert.NoError(t, err, tc.testCase+": can not read raw segment")
		assert.True(t, len(content) > 0, tc.testCase+": content length is 0")

		var (
			actualSeg  awsxray.Segment
			expectedRs *pdata.ResourceSpans
		)
		if !tc.expectedUnmarshallFailure {
			err = json.Unmarshal(content, &actualSeg)
			// the correctness of the actual segment
			// has been verified in the tracesegment_test.go
			assert.NoError(t, err, tc.testCase+": failed to unmarhal raw segment")
			expectedRs = initResourceSpans(
				&actualSeg,
				tc.expectedResourceAttrs(&actualSeg),
				tc.propsPerSpan(tc.testCase, t, &actualSeg),
			)
		}

		traces, totalSpansCount, err := ToTraces(content)
		if err == nil || (expectedRs != nil && expectedRs.InstrumentationLibrarySpans().Len() > 0 &&
			expectedRs.InstrumentationLibrarySpans().At(0).Spans().Len() > 0) {
			assert.Equal(t, totalSpansCount,
				expectedRs.InstrumentationLibrarySpans().At(0).Spans().Len(),
				"generated span count is different from the expected",
			)
		}
		tc.verification(tc.testCase, &actualSeg, expectedRs, traces, err)
	}
}

func initExceptionEvents(expectedSeg *awsxray.Segment) []eventProps {
	res := make([]eventProps, 0, len(expectedSeg.Cause.Exceptions))
	for _, excp := range expectedSeg.Cause.Exceptions {
		attrs := make(map[string]pdata.AttributeValue)
		attrs[awsxray.AWSXrayExceptionIDAttribute] = pdata.NewAttributeValueString(
			*excp.ID)
		if excp.Message != nil {
			attrs[conventions.AttributeExceptionMessage] = pdata.NewAttributeValueString(
				*excp.Message)
		}

		if excp.Type != nil {
			attrs[conventions.AttributeExceptionType] = pdata.NewAttributeValueString(
				*excp.Type)
		}

		if excp.Remote != nil {
			attrs[awsxray.AWSXrayExceptionRemoteAttribute] = pdata.NewAttributeValueBool(
				*excp.Remote)
		}

		if excp.Truncated != nil {
			attrs[awsxray.AWSXrayExceptionTruncatedAttribute] = pdata.NewAttributeValueInt(
				*excp.Truncated)
		}

		if excp.Skipped != nil {
			attrs[awsxray.AWSXrayExceptionSkippedAttribute] = pdata.NewAttributeValueInt(
				*excp.Skipped)
		}

		if excp.Cause != nil {
			attrs[awsxray.AWSXrayExceptionCauseAttribute] = pdata.NewAttributeValueString(
				*excp.Cause)
		}

		if len(excp.Stack) > 0 {
			attrs[conventions.AttributeExceptionStacktrace] = pdata.NewAttributeValueString(
				convertStackFramesToStackTraceStr(excp))
		}
		res = append(res, eventProps{
			name:  conventions.AttributeExceptionEventName,
			attrs: attrs,
		})
	}
	return res
}

func initResourceSpans(expectedSeg *awsxray.Segment,
	resourceAttrs map[string]pdata.AttributeValue,
	propsPerSpan []perSpanProperties,
) *pdata.ResourceSpans {
	if expectedSeg == nil {
		return nil
	}

	rs := pdata.NewResourceSpans()
	rs.InitEmpty()
	rs.Resource().InitEmpty()

	if len(resourceAttrs) > 0 {
		resourceAttrMap := pdata.NewAttributeMap()
		resourceAttrMap.InitFromMap(resourceAttrs)
		rs.Resource().Attributes().InitFromAttributeMap(resourceAttrMap)
	} else {
		rs.Resource().Attributes().InitEmptyWithCapacity(initAttrCapacity)
	}

	if len(propsPerSpan) == 0 {
		return &rs
	}

	rs.InstrumentationLibrarySpans().Resize(1)
	ls := rs.InstrumentationLibrarySpans().At(0)
	ls.InitEmpty()
	ls.Spans().Resize(len(propsPerSpan))

	for i, props := range propsPerSpan {
		sp := ls.Spans().At(i)
		sp.SetSpanID(pdata.NewSpanID([]byte(props.spanID)))
		if props.parentSpanID != nil {
			sp.SetParentSpanID(pdata.NewSpanID([]byte(*props.parentSpanID)))
		}
		sp.SetName(props.name)
		sp.SetStartTime(pdata.TimestampUnixNano(props.startTimeSec * float64(time.Second)))
		if props.endTimeSec != nil {
			sp.SetEndTime(pdata.TimestampUnixNano(*props.endTimeSec * float64(time.Second)))
		}
		sp.SetKind(props.spanKind)
		sp.SetTraceID(pdata.NewTraceID([]byte(props.traceID)))
		sp.Status().InitEmpty()
		sp.Status().SetMessage(props.spanStatus.message)
		sp.Status().SetCode(props.spanStatus.code)

		if len(props.eventsProps) > 0 {
			sp.Events().Resize(len(props.eventsProps))
			for i, evtProps := range props.eventsProps {
				spEvt := sp.Events().At(i)
				spEvt.SetName(evtProps.name)
				evtAttrMap := pdata.NewAttributeMap()
				evtAttrMap.InitFromMap(evtProps.attrs)
				spEvt.Attributes().InitFromAttributeMap(evtAttrMap)
			}
		}

		if len(props.attrs) > 0 {
			spanAttrMap := pdata.NewAttributeMap()
			spanAttrMap.InitFromMap(props.attrs)
			sp.Attributes().InitFromAttributeMap(spanAttrMap)
		} else {
			sp.Attributes().InitEmptyWithCapacity(initAttrCapacity)
		}
	}
	return &rs
}

// note that this function causes side effects on the expected (
// abbrev. as exp) and actual ResourceSpans (abbrev. as act):
// 1. clears the resource attributes on both exp and act, after verifying
// .  both sets are the same.
// 2. clears the span attributes of all the
//    spans on both exp and act, after going through all the spans
// .  on both exp and act and verify that all the attributes match.
// 3. similarly, for all the events and their attributes within a span,
//    this function performs the same equality verification, then clears
//    up all the attribute.
// The reason for doing so is just to be able to use deep equal via assert.Equal()
func compare2ResourceSpans(t *testing.T, testCase string, exp, act *pdata.ResourceSpans) {
	assert.Equal(t, exp.InstrumentationLibrarySpans().Len(),
		act.InstrumentationLibrarySpans().Len(),
		testCase+": InstrumentationLibrarySpans.Len() differ")

	assert.Equal(t,
		exp.Resource().Attributes().Sort(),
		act.Resource().Attributes().Sort(),
		testCase+": Resource.Attributes() differ")
	exp.Resource().InitEmpty()
	act.Resource().InitEmpty()

	actSpans := act.InstrumentationLibrarySpans().At(0).Spans()
	expSpans := exp.InstrumentationLibrarySpans().At(0).Spans()
	assert.Equal(t,
		expSpans.Len(),
		actSpans.Len(),
		testCase+": span.Len() differ",
	)

	for i := 0; i < expSpans.Len(); i++ {
		expS := expSpans.At(i)
		actS := actSpans.At(i)

		assert.Equal(t,
			expS.Attributes().Sort(),
			actS.Attributes().Sort(),
			fmt.Sprintf("%s: span[%s].Attributes() differ", testCase, expS.SpanID().HexString()),
		)
		expS.Attributes().InitEmptyWithCapacity(0)
		actS.Attributes().InitEmptyWithCapacity(0)

		expEvts := expS.Events()
		actEvts := actS.Events()
		assert.Equal(t,
			expEvts.Len(),
			actEvts.Len(),
			fmt.Sprintf("%s: span[%s].Events().Len() differ",
				testCase, expS.SpanID().HexString()),
		)

		for j := 0; j < expEvts.Len(); j++ {
			expEvt := expEvts.At(j)
			actEvt := actEvts.At(j)

			assert.Equal(t,
				expEvt.Attributes().Sort(),
				actEvt.Attributes().Sort(),
				fmt.Sprintf("%s: span[%s], event[%d].Attributes() differ",
					testCase, expS.SpanID().HexString(), j),
			)
			expEvt.Attributes().InitEmptyWithCapacity(0)
			actEvt.Attributes().InitEmptyWithCapacity(0)
		}
	}

	assert.Equal(t, exp, act,
		testCase+": actual ResourceSpans differ from the expected")
}

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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
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
	attrs        pdata.Map
}

type spanSt struct {
	message string
	code    pdata.StatusCode
}

type eventProps struct {
	name  string
	attrs pdata.Map
}

func TestTranslation(t *testing.T) {
	var defaultServerSpanAttrs = func(seg *awsxray.Segment) pdata.Map {
		return pdata.NewMapFromRaw(map[string]interface{}{
			conventions.AttributeHTTPMethod:       *seg.HTTP.Request.Method,
			conventions.AttributeHTTPClientIP:     *seg.HTTP.Request.ClientIP,
			conventions.AttributeHTTPUserAgent:    *seg.HTTP.Request.UserAgent,
			awsxray.AWSXRayXForwardedForAttribute: *seg.HTTP.Request.XForwardedFor,
			conventions.AttributeHTTPStatusCode:   *seg.HTTP.Response.Status,
			conventions.AttributeHTTPURL:          *seg.HTTP.Request.URL,
		})
	}

	tests := []struct {
		testCase                  string
		expectedUnmarshallFailure bool
		samplePath                string
		expectedResourceAttrs     func(seg *awsxray.Segment) pdata.Map
		propsPerSpan              func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties
		verification              func(testCase string,
			actualSeg *awsxray.Segment,
			expectedRs *pdata.ResourceSpans,
			actualTraces *pdata.Traces,
			err error)
	}{
		{
			testCase:   "TranslateInstrumentedServerSegment",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "serverSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMapFromRaw(map[string]interface{}{
					conventions.AttributeCloudProvider:        conventions.AttributeCloudProviderAWS,
					conventions.AttributeTelemetrySDKVersion:  *seg.AWS.XRay.SDKVersion,
					conventions.AttributeTelemetrySDKName:     *seg.AWS.XRay.SDK,
					conventions.AttributeTelemetrySDKLanguage: "Go",
					conventions.AttributeK8SClusterName:       *seg.AWS.EKS.ClusterName,
					conventions.AttributeK8SPodName:           *seg.AWS.EKS.Pod,
					conventions.AttributeContainerID:          *seg.AWS.EKS.ContainerID,
				})
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindServer,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "ddbSample.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMapFromRaw(map[string]interface{}{
					conventions.AttributeCloudProvider:        conventions.AttributeCloudProviderAWS,
					conventions.AttributeTelemetrySDKVersion:  *seg.AWS.XRay.SDKVersion,
					conventions.AttributeTelemetrySDKName:     *seg.AWS.XRay.SDK,
					conventions.AttributeTelemetrySDKLanguage: "java",
				})
			},
			propsPerSpan: func(testCase string, t *testing.T, seg *awsxray.Segment) []perSpanProperties {
				rootSpanAttrs := pdata.NewMap()
				rootSpanAttrs.UpsertString(conventions.AttributeEnduserID, *seg.User)
				rootSpanEvts := initExceptionEvents(seg)
				assert.Len(t, rootSpanEvts, 1, testCase+": rootSpanEvts has incorrect size")
				rootSpan := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindServer,
					spanStatus: spanSt{
						code: pdata.StatusCodeError,
					},
					eventsProps: rootSpanEvts,
					attrs:       rootSpanAttrs,
				}

				// this is the subsegment with ID that starts with 7df6
				subseg7df6 := seg.Subsegments[0]
				childSpan7df6Attrs := pdata.NewMap()
				for k, v := range subseg7df6.Annotations {
					childSpan7df6Attrs.UpsertString(k, v.(string))
				}
				for k, v := range subseg7df6.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan7df6Attrs.UpsertString(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				assert.Equal(t, 2, childSpan7df6Attrs.Len(), testCase+": childSpan7df6Attrs has incorrect size")
				childSpan7df6Evts := initExceptionEvents(&subseg7df6)
				assert.Len(t, childSpan7df6Evts, 1, testCase+": childSpan7df6Evts has incorrect size")
				childSpan7df6 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7df6.ID,
					parentSpanID: &rootSpan.spanID,
					name:         *subseg7df6.Name,
					startTimeSec: *subseg7df6.StartTime,
					endTimeSec:   subseg7df6.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeError,
					},
					eventsProps: childSpan7df6Evts,
					attrs:       childSpan7df6Attrs,
				}

				subseg7318 := seg.Subsegments[0].Subsegments[0]
				childSpan7318Attrs := pdata.NewMapFromRaw(map[string]interface{}{
					awsxray.AWSServiceAttribute:                    *subseg7318.Name,
					conventions.AttributeHTTPResponseContentLength: int64(subseg7318.HTTP.Response.ContentLength.(float64)),
					conventions.AttributeHTTPStatusCode:            *subseg7318.HTTP.Response.Status,
					awsxray.AWSOperationAttribute:                  *subseg7318.AWS.Operation,
					awsxray.AWSRegionAttribute:                     *subseg7318.AWS.RemoteRegion,
					awsxray.AWSRequestIDAttribute:                  *subseg7318.AWS.RequestID,
					awsxray.AWSTableNameAttribute:                  *subseg7318.AWS.TableName,
					awsxray.AWSXrayRetriesAttribute:                *subseg7318.AWS.Retries,
				})

				childSpan7318 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7318.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7318.Name,
					startTimeSec: *subseg7318.StartTime,
					endTimeSec:   subseg7318.EndTime,
					spanKind:     pdata.SpanKindClient,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subseg23cf := seg.Subsegments[0].Subsegments[0].Subsegments[1]
				childSpan23cf := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg23cf.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg23cf.Name,
					startTimeSec: *subseg23cf.StartTime,
					endTimeSec:   subseg23cf.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subseg417b := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0]
				childSpan417bAttrs := pdata.NewMap()
				for k, v := range subseg417b.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan417bAttrs.UpsertString(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpan417b := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg417b.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subseg417b.Name,
					startTimeSec: *subseg417b.StartTime,
					endTimeSec:   subseg417b.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpan417bAttrs,
				}

				subseg0cab := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[0]
				childSpan0cabAttrs := pdata.NewMap()
				for k, v := range subseg0cab.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpan0cabAttrs.UpsertString(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpan0cab := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg0cab.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subseg0cab.Name,
					startTimeSec: *subseg0cab.StartTime,
					endTimeSec:   subseg0cab.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpan0cabAttrs,
				}

				subsegF8db := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[1]
				childSpanF8dbAttrs := pdata.NewMap()
				for k, v := range subsegF8db.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanF8dbAttrs.UpsertString(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpanF8db := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegF8db.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegF8db.Name,
					startTimeSec: *subsegF8db.StartTime,
					endTimeSec:   subsegF8db.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       childSpanF8dbAttrs,
				}

				subsegE2de := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[0].Subsegments[2]
				childSpanE2deAttrs := pdata.NewMap()
				for k, v := range subsegE2de.Metadata {
					m, err := json.Marshal(v)
					assert.NoError(t, err, "metadata marshaling failed")
					childSpanE2deAttrs.UpsertString(awsxray.AWSXraySegmentMetadataAttributePrefix+k, string(m))
				}
				childSpanE2de := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegE2de.ID,
					parentSpanID: &childSpan417b.spanID,
					name:         *subsegE2de.Name,
					startTimeSec: *subsegE2de.StartTime,
					endTimeSec:   subsegE2de.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subsegC053 := seg.Subsegments[0].Subsegments[0].Subsegments[1].Subsegments[2]
				childSpanC053 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegC053.ID,
					parentSpanID: &childSpan23cf.spanID,
					name:         *subsegC053.Name,
					startTimeSec: *subsegC053.StartTime,
					endTimeSec:   subsegC053.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subseg5fca := seg.Subsegments[0].Subsegments[0].Subsegments[2]
				childSpan5fca := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg5fca.ID,
					parentSpanID: &childSpan7318.spanID,
					name:         *subseg5fca.Name,
					startTimeSec: *subseg5fca.StartTime,
					endTimeSec:   subseg5fca.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subseg7163 := seg.Subsegments[0].Subsegments[1]
				childSpan7163Attrs := pdata.NewMapFromRaw(map[string]interface{}{
					awsxray.AWSServiceAttribute:                    *subseg7163.Name,
					conventions.AttributeHTTPStatusCode:            *subseg7163.HTTP.Response.Status,
					conventions.AttributeHTTPResponseContentLength: int64(subseg7163.HTTP.Response.ContentLength.(float64)),
					awsxray.AWSOperationAttribute:                  *subseg7163.AWS.Operation,
					awsxray.AWSRegionAttribute:                     *subseg7163.AWS.RemoteRegion,
					awsxray.AWSRequestIDAttribute:                  *subseg7163.AWS.RequestID,
					awsxray.AWSTableNameAttribute:                  *subseg7163.AWS.TableName,
					awsxray.AWSXrayRetriesAttribute:                *subseg7163.AWS.Retries,
				})

				childSpan7163Evts := initExceptionEvents(&subseg7163)
				assert.Len(t, childSpan7163Evts, 1, testCase+": childSpan7163Evts has incorrect size")
				childSpan7163 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg7163.ID,
					parentSpanID: &childSpan7df6.spanID,
					name:         *subseg7163.Name,
					startTimeSec: *subseg7163.StartTime,
					endTimeSec:   subseg7163.EndTime,
					spanKind:     pdata.SpanKindClient,
					spanStatus: spanSt{
						code: pdata.StatusCodeError,
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
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
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
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeError,
					},
					eventsProps: childSpan56b1Evts,
					attrs:       pdata.NewMap(),
				}

				subseg6f90 := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[0]
				childSpan6f90 := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subseg6f90.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subseg6f90.Name,
					startTimeSec: *subseg6f90.StartTime,
					endTimeSec:   subseg6f90.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
				}

				subsegAcfa := seg.Subsegments[0].Subsegments[1].Subsegments[1].Subsegments[1]
				childSpanAcfa := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *subsegAcfa.ID,
					parentSpanID: &childSpan56b1.spanID,
					name:         *subsegAcfa.Name,
					startTimeSec: *subsegAcfa.StartTime,
					endTimeSec:   subsegAcfa.EndTime,
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
					},
					eventsProps: nil,
					attrs:       pdata.NewMap(),
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
					spanKind:     pdata.SpanKindInternal,
					spanStatus: spanSt{
						code: pdata.StatusCodeError,
					},
					eventsProps: childSpanBa8dEvts,
					attrs:       pdata.NewMap(),
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "awsMissingAwsField.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				attrs := pdata.NewMap()
				attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
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
					spanKind:     pdata.SpanKindServer,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "awsValidAwsFields.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMapFromRaw(map[string]interface{}{
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
				})
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := defaultServerSpanAttrs(seg)
				attrs.UpsertString(awsxray.AWSAccountAttribute, *seg.AWS.AccountID)
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindServer,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "minCauseIsExceptionId.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				attrs := pdata.NewMap()
				attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindServer,
					spanStatus: spanSt{
						message: *seg.Cause.ExceptionID,
						code:    pdata.StatusCodeError,
					},
					attrs: pdata.NewMap(),
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "invalidNamespace.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMap()
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegment.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				attrs := pdata.NewMap()
				attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pdata.NewMapFromRaw(map[string]interface{}{
					conventions.AttributeHTTPMethod:                *seg.HTTP.Request.Method,
					conventions.AttributeHTTPStatusCode:            *seg.HTTP.Response.Status,
					conventions.AttributeHTTPURL:                   *seg.HTTP.Request.URL,
					conventions.AttributeHTTPResponseContentLength: int64(seg.HTTP.Response.ContentLength.(float64)),
					awsxray.AWSXRayTracedAttribute:                 true,
				})
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindClient,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			testCase:   "TranslateIndepSubsegmentForContentLengthString",
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithContentLengthString.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				attrs := pdata.NewMap()
				attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pdata.NewMapFromRaw(map[string]interface{}{
					conventions.AttributeHTTPMethod:                *seg.HTTP.Request.Method,
					conventions.AttributeHTTPStatusCode:            *seg.HTTP.Response.Status,
					conventions.AttributeHTTPURL:                   *seg.HTTP.Request.URL,
					conventions.AttributeHTTPResponseContentLength: seg.HTTP.Response.ContentLength.(string),
					awsxray.AWSXRayTracedAttribute:                 true,
				})

				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindClient,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithSql.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				attrs := pdata.NewMap()
				attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
				return attrs
			},
			propsPerSpan: func(_ string, _ *testing.T, seg *awsxray.Segment) []perSpanProperties {
				attrs := pdata.NewMapFromRaw(map[string]interface{}{
					conventions.AttributeDBConnectionString: "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2." +
						"rds.amazonaws.com:5432",
					conventions.AttributeDBName:      "ebdb",
					conventions.AttributeDBSystem:    *seg.SQL.DatabaseType,
					conventions.AttributeDBStatement: *seg.SQL.SanitizedQuery,
					conventions.AttributeDBUser:      *seg.SQL.User,
				})
				res := perSpanProperties{
					traceID:      *seg.TraceID,
					spanID:       *seg.ID,
					parentSpanID: seg.ParentID,
					name:         *seg.Name,
					startTimeSec: *seg.StartTime,
					endTimeSec:   seg.EndTime,
					spanKind:     pdata.SpanKindClient,
					spanStatus: spanSt{
						code: pdata.StatusCodeUnset,
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "indepSubsegmentWithInvalidSqlUrl.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMap()
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
			samplePath:                filepath.Join("../../../../internal/aws/xray", "testdata", "minCauseIsInvalid.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMap()
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
			samplePath: filepath.Join("../../../../internal/aws/xray", "testdata", "segmentValidationFailed.txt"),
			expectedResourceAttrs: func(seg *awsxray.Segment) pdata.Map {
				return pdata.NewMap()
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

		traces, totalSpanCount, err := ToTraces(content)
		if err == nil || (expectedRs != nil && expectedRs.InstrumentationLibrarySpans().Len() > 0 &&
			expectedRs.InstrumentationLibrarySpans().At(0).Spans().Len() > 0) {
			assert.Equal(t, totalSpanCount,
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
		attrs := pdata.NewMap()
		attrs.UpsertString(awsxray.AWSXrayExceptionIDAttribute, *excp.ID)
		if excp.Message != nil {
			attrs.UpsertString(conventions.AttributeExceptionMessage, *excp.Message)
		}

		if excp.Type != nil {
			attrs.UpsertString(conventions.AttributeExceptionType, *excp.Type)
		}

		if excp.Remote != nil {
			attrs.UpsertBool(awsxray.AWSXrayExceptionRemoteAttribute, *excp.Remote)
		}

		if excp.Truncated != nil {
			attrs.UpsertInt(awsxray.AWSXrayExceptionTruncatedAttribute, *excp.Truncated)
		}

		if excp.Skipped != nil {
			attrs.UpsertInt(awsxray.AWSXrayExceptionSkippedAttribute, *excp.Skipped)
		}

		if excp.Cause != nil {
			attrs.UpsertString(awsxray.AWSXrayExceptionCauseAttribute, *excp.Cause)
		}

		if len(excp.Stack) > 0 {
			attrs.UpsertString(conventions.AttributeExceptionStacktrace, convertStackFramesToStackTraceStr(excp))
		}
		res = append(res, eventProps{
			name:  ExceptionEventName,
			attrs: attrs,
		})
	}
	return res
}

func initResourceSpans(expectedSeg *awsxray.Segment,
	resourceAttrs pdata.Map,
	propsPerSpan []perSpanProperties,
) *pdata.ResourceSpans {
	if expectedSeg == nil {
		return nil
	}

	rs := pdata.NewResourceSpans()

	if resourceAttrs.Len() > 0 {
		resourceAttrs.CopyTo(rs.Resource().Attributes())
	} else {
		rs.Resource().Attributes().Clear()
		rs.Resource().Attributes().EnsureCapacity(initAttrCapacity)
	}

	if len(propsPerSpan) == 0 {
		return &rs
	}

	ls := rs.InstrumentationLibrarySpans().AppendEmpty()
	ls.Spans().EnsureCapacity(len(propsPerSpan))

	for _, props := range propsPerSpan {
		sp := ls.Spans().AppendEmpty()
		spanIDBytes, _ := decodeXRaySpanID(&props.spanID)
		sp.SetSpanID(pdata.NewSpanID(spanIDBytes))
		if props.parentSpanID != nil {
			parentIDBytes, _ := decodeXRaySpanID(props.parentSpanID)
			sp.SetParentSpanID(pdata.NewSpanID(parentIDBytes))
		}
		sp.SetName(props.name)
		sp.SetStartTimestamp(pdata.Timestamp(props.startTimeSec * float64(time.Second)))
		if props.endTimeSec != nil {
			sp.SetEndTimestamp(pdata.Timestamp(*props.endTimeSec * float64(time.Second)))
		}
		sp.SetKind(props.spanKind)
		traceIDBytes, _ := decodeXRayTraceID(&props.traceID)
		sp.SetTraceID(pdata.NewTraceID(traceIDBytes))
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
		expS.Attributes().Clear()
		actS.Attributes().Clear()

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
			expEvt.Attributes().Clear()
			actEvt.Attributes().Clear()
		}
	}

	assert.Equal(t, exp, act,
		testCase+": actual ResourceSpans differ from the expected")
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

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

package awsxray

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/awsxray/util"
)

var rawExpectedSegmentForInstrumentedApp = Segment{
	Name:      String("DDB"),
	ID:        String("88ad1df59cd7a7be"),
	StartTime: Float64(1596566305.535414),
	TraceID:   String("1-5f29ab21-d4ebf299219a65bd5c31d6da"),
	EndTime:   Float64(1596566305.5928545),
	Fault:     Bool(true),
	User:      String("xraysegmentdump"),
	Cause: &CauseData{
		Type: CauseTypeObject,
		CauseObject: CauseObject{
			WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
			Exceptions: []Exception{
				{
					ID:      String("3e9e11e3ab3fba60"),
					Message: String("ResourceNotFoundException: Requested resource not found"),
					Type:    String("dynamodb.ResourceNotFoundException"),
					Remote:  Bool(true),
					Stack: []StackFrame{
						{
							Path:  String("runtime/proc.go"),
							Line:  Int(203),
							Label: String("main"),
						},
						{
							Path:  String("runtime/asm_amd64.s"),
							Line:  Int(1373),
							Label: String("goexit"),
						},
					},
				},
			},
		},
	},
	AWS: &AWSData{
		XRay: &XRayMetaData{
			SDKVersion: String("1.1.0"),
			SDK:        String("X-Ray for Go"),
		},
	},
	Service: &ServiceData{
		CompilerVersion: String("go1.14.6"),
		Compiler:        String("gc"),
	},
	Subsegments: []Segment{
		{
			Name:      String("DDB.DescribeExistingTableAndPutToMissingTable"),
			ID:        String("7df694142c905d8d"),
			StartTime: Float64(1596566305.5354965),
			EndTime:   Float64(1596566305.5928457),
			Fault:     Bool(true),
			Cause: &CauseData{
				Type: CauseTypeObject,
				CauseObject: CauseObject{
					WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
					Exceptions: []Exception{
						{
							ID:      String("e2ba8a2109451f5b"),
							Message: String("ResourceNotFoundException: Requested resource not found"),
							Type:    String("dynamodb.ResourceNotFoundException"),
							Remote:  Bool(true),
							Stack: []StackFrame{
								{
									Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
									Line:  Int(48),
									Label: String("Capture"),
								},
								{
									Path:  String("sampleapp/sample.go"),
									Line:  Int(41),
									Label: String("ddbExpectedFailure"),
								},
								{
									Path:  String("sampleapp/sample.go"),
									Line:  Int(36),
									Label: String("main"),
								},
								{
									Path:  String("runtime/proc.go"),
									Line:  Int(203),
									Label: String("main"),
								},
								{
									Path:  String("runtime/asm_amd64.s"),
									Line:  Int(1373),
									Label: String("goexit"),
								},
							},
						},
					},
				},
			},
			Annotations: map[string]interface{}{
				"DDB.DescribeExistingTableAndPutToMissingTable.Annotation": "anno",
			},
			Metadata: map[string]map[string]interface{}{
				"default": {
					"DDB.DescribeExistingTableAndPutToMissingTable.AddMetadata": "meta",
				},
			},
			Subsegments: []Segment{
				{
					Name:      String("dynamodb"),
					ID:        String("7318c46a385557f5"),
					StartTime: Float64(1596566305.5355225),
					EndTime:   Float64(1596566305.5873947),
					Namespace: String("aws"),
					HTTP: &HTTPData{
						Response: &ResponseData{
							Status:        Int64(200),
							ContentLength: Int64(713),
						},
					},
					AWS: &AWSData{
						Operation:    String("DescribeTable"),
						RemoteRegion: String("us-west-2"),
						RequestID:    String("29P5V7QSAKHS4LNL56ECAJFF3BVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						Retries:      Int(0),
						TableName:    String("xray_sample_table"),
					},
					Subsegments: []Segment{
						{
							ID:        String("0239834271dbee25"),
							Name:      String("marshal"),
							StartTime: Float64(1596566305.5355248),
							EndTime:   Float64(1596566305.5355635),
						},
						{
							ID:        String("23cf5bb60e4f66b1"),
							Name:      String("attempt"),
							StartTime: Float64(1596566305.5355663),
							EndTime:   Float64(1596566305.5873196),
							Subsegments: []Segment{
								{
									ID:        String("417b81b977b9563b"),
									Name:      String("connect"),
									StartTime: Float64(1596566305.5357504),
									EndTime:   Float64(1596566305.575329),
									Metadata: map[string]map[string]interface{}{
										"http": {
											"connection": map[string]interface{}{
												"reused":   false,
												"was_idle": false,
											},
										},
									},
									Subsegments: []Segment{
										{
											ID:        String("0cab02b318413eb1"),
											Name:      String("dns"),
											StartTime: Float64(1596566305.5357957),
											EndTime:   Float64(1596566305.5373216),
											Metadata: map[string]map[string]interface{}{
												"http": {
													"dns": map[string]interface{}{
														"addresses": []interface{}{
															map[string]interface{}{
																"IP":   "52.94.10.94",
																"Zone": "",
															},
														},
														"coalesced": false,
													},
												},
											},
										},
										{
											ID:        String("f8dbc5c6b291017e"),
											Name:      String("dial"),
											StartTime: Float64(1596566305.5373297),
											EndTime:   Float64(1596566305.537964),
											Metadata: map[string]map[string]interface{}{
												"http": {
													"connect": map[string]interface{}{
														"network": "tcp",
													},
												},
											},
										},
										{
											ID:        String("e2deb66ecaa769a5"),
											Name:      String("tls"),
											StartTime: Float64(1596566305.5380135),
											EndTime:   Float64(1596566305.5753162),
											Metadata: map[string]map[string]interface{}{
												"http": {
													"tls": map[string]interface{}{
														"cipher_suite":                  49199.0,
														"did_resume":                    false,
														"negotiated_protocol":           "http/1.1",
														"negotiated_protocol_is_mutual": true,
													},
												},
											},
										},
									},
								},
								{
									ID:        String("a70bfab91597c7a2"),
									Name:      String("request"),
									StartTime: Float64(1596566305.5753367),
									EndTime:   Float64(1596566305.5754144),
								},
								{
									ID:        String("c05331c26d3e8a7f"),
									Name:      String("response"),
									StartTime: Float64(1596566305.5754204),
									EndTime:   Float64(1596566305.5872962),
								},
							},
						},
						{
							ID:        String("5fca2dfc9de81f4c"),
							Name:      String("unmarshal"),
							StartTime: Float64(1596566305.5873249),
							EndTime:   Float64(1596566305.587389),
						},
					},
				},
				{
					Name:      String("dynamodb"),
					ID:        String("71631df3f58bdfc5"),
					StartTime: Float64(1596566305.5874245),
					EndTime:   Float64(1596566305.5928326),
					Fault:     Bool(true),
					Cause: &CauseData{
						Type: CauseTypeObject,
						CauseObject: CauseObject{
							WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
							Exceptions: []Exception{
								{
									ID:      String("7121b882a0ef44da"),
									Message: String("ResourceNotFoundException: Requested resource not found"),
									Type:    String("dynamodb.ResourceNotFoundException"),
									Remote:  Bool(true),
									Stack: []StackFrame{
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
											Line:  Int(267),
											Label: String("(*HandlerList).Run"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  Int(515),
											Label: String("(*Request).Send.func1"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  Int(538),
											Label: String("(*Request).Send"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
											Line:  Int(3414),
											Label: String("(*DynamoDB).PutItemWithContext"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  Int(62),
											Label: String("ddbExpectedFailure.func1"),
										},
										{
											Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
											Line:  Int(45),
											Label: String("Capture"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  Int(41),
											Label: String("ddbExpectedFailure"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  Int(36),
											Label: String("main"),
										},
										{
											Path:  String("runtime/proc.go"),
											Line:  Int(203),
											Label: String("main"),
										},
										{
											Path:  String("runtime/asm_amd64.s"),
											Line:  Int(1373),
											Label: String("goexit"),
										},
									},
								},
							},
						},
					},
					Namespace: String("aws"),
					HTTP: &HTTPData{
						Response: &ResponseData{
							Status:        Int64(400),
							ContentLength: Int64(112),
						},
					},
					AWS: &AWSData{
						Operation:    String("PutItem"),
						RemoteRegion: String("us-west-2"),
						RequestID:    String("TJUJNR0JV84CFHJL93D3GIA0LBVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						TableName:    String("does_not_exist"),
						Retries:      Int(0),
					},
					Subsegments: []Segment{
						{
							Name:      String("marshal"),
							ID:        String("9da02fcbb9711b47"),
							StartTime: Float64(1596566305.5874267),
							EndTime:   Float64(1596566305.58745),
						},
						{
							Name:      String("attempt"),
							ID:        String("56b1cb185cbdb378"),
							StartTime: Float64(1596566305.587453),
							EndTime:   Float64(1596566305.592767),
							Fault:     Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      String("59de8ae27660d21d"),
											Message: String("ResourceNotFoundException: Requested resource not found"),
											Type:    String("dynamodb.ResourceNotFoundException"),
											Remote:  Bool(true),
											Stack: []StackFrame{
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/go"),
													Line:  Int(139),
													Label: String("glob..func7"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  Int(267),
													Label: String("(*HandlerList).Run"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  Int(534),
													Label: String("(*Request).Send"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  Int(3414),
													Label: String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(62),
													Label: String("ddbExpectedFailure.func1"),
												},
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  Int(45),
													Label: String("Capture"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(41),
													Label: String("ddbExpectedFailure"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(36),
													Label: String("main"),
												},
												{
													Path:  String("runtime/proc.go"),
													Line:  Int(203),
													Label: String("main"),
												},
												{
													Path:  String("runtime/asm_amd64.s"),
													Line:  Int(1373),
													Label: String("goexit"),
												},
											},
										},
									},
								},
							},
							Subsegments: []Segment{
								{
									Name:      String("request"),
									ID:        String("6f908a1d3ec70abe"),
									StartTime: Float64(1596566305.5875077),
									EndTime:   Float64(1596566305.587543),
								},
								{
									Name:      String("response"),
									ID:        String("acfaa7e3fe3aab03"),
									StartTime: Float64(1596566305.5875454),
									EndTime:   Float64(1596566305.592695),
								},
							},
						},
						{
							Name:      String("wait"),
							ID:        String("ba8d350c0e8cdc4b"),
							StartTime: Float64(1596566305.592807),
							EndTime:   Float64(1596566305.5928102),
							Fault:     Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      String("5a07f08a8c260405"),
											Message: String("ResourceNotFoundException: Requested resource not found"),
											Type:    String("dynamodb.ResourceNotFoundException"),
											Remote:  Bool(true),
											Stack: []StackFrame{
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/go"),
													Line:  Int(149),
													Label: String("glob..func8"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  Int(267),
													Label: String("(*HandlerList).Run"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  Int(535),
													Label: String("(*Request).Send"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  Int(3414),
													Label: String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(62),
													Label: String("ddbExpectedFailure.func1"),
												},
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  Int(45),
													Label: String("Capture"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(41),
													Label: String("ddbExpectedFailure"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  Int(36),
													Label: String("main"),
												},
												{
													Path:  String("runtime/proc.go"),
													Line:  Int(203),
													Label: String("main"),
												},
												{
													Path:  String("runtime/asm_amd64.s"),
													Line:  Int(1373),
													Label: String("goexit"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var rawExpectedSegmentForInstrumentedServer = Segment{
	Name:      String("SampleServer"),
	ID:        String("bda182a644eee9b3"),
	StartTime: Float64(1596648396.6399446),
	TraceID:   String("1-5f2aebcc-b475d14618c51eaa28753d37"),
	EndTime:   Float64(1596648396.6401389),
	HTTP: &HTTPData{
		Request: &RequestData{
			Method:        String("GET"),
			URL:           String("http://localhost:8000/"),
			ClientIP:      String("127.0.0.1"),
			UserAgent:     String("Go-http-client/1.1"),
			XForwardedFor: Bool(true),
		},
		Response: &ResponseData{
			Status: Int64(200),
		},
	},
	AWS: &AWSData{
		XRay: &XRayMetaData{
			SDKVersion: String("1.1.0"),
			SDK:        String("X-Ray for Go"),
		},
	},
	Service: &ServiceData{
		CompilerVersion: String("go1.14.6"),
		Compiler:        String("gc"),
	},
}

func TestTraceBodyUnMarshalling(t *testing.T) {
	tests := []struct {
		testCase     string
		samplePath   string
		verification func(testCase string, actualSeg Segment, err error)
	}{
		{
			testCase:   "TestTraceBodyCorrectlyUnmarshalledForInstrumentedApp",
			samplePath: path.Join("testdata", "ddbSample.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, rawExpectedSegmentForInstrumentedApp,
					actualSeg, testCase+": unmarshalled app segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyInProgressUnmarshalled",
			samplePath: path.Join("testdata", "minInProgress.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:       String("LongOperation"),
					ID:         String("5cc4a447f5d4d696"),
					StartTime:  Float64(1595437651.680097),
					TraceID:    String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					InProgress: Bool(true),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyOtherTopLevelFieldsUnmarshalled",
			samplePath: path.Join("testdata", "minOtherFields.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:        String("OtherTopLevelFields"),
					ID:          String("5cc4a447f5d4d696"),
					StartTime:   Float64(1595437651.680097),
					EndTime:     Float64(1595437652.197392),
					TraceID:     String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Error:       Bool(false),
					Throttle:    Bool(true),
					ResourceARN: String("chicken"),
					Origin:      String("AWS::EC2::Instance"),
					ParentID:    String("defdfd9912dc5a56"),
					Type:        String("subsegment"),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyCauseIsExceptionIdUnmarshalled",
			samplePath: path.Join("testdata", "minCauseIsExceptionId.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:      String("CauseIsExceptionID"),
					ID:        String("5cc4a447f5d4d696"),
					StartTime: Float64(1595437651.680097),
					EndTime:   Float64(1595437652.197392),
					TraceID:   String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Fault:     Bool(true),
					Cause: &CauseData{
						Type:        CauseTypeExceptionID,
						ExceptionID: String("abcdefghijklmnop"),
					},
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")

			},
		},
		{
			testCase:   "TestTraceBodyInvalidCauseUnmarshalled",
			samplePath: path.Join("testdata", "minCauseIsInvalid.txt"),
			verification: func(testCase string, _ Segment, err error) {
				assert.EqualError(t, err,
					fmt.Sprintf(
						"the value assigned to the `cause` field does not appear to be a string: %v",
						[]byte{'2', '0', '0'},
					),
					testCase+": invalid `cause` implies invalid segment, so unmarshalling should've failed")
			},
		},
		{
			testCase:   "TestTraceBodyCorrectlyUnmarshalledForInstrumentedServer",
			samplePath: path.Join("testdata", "serverSample.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, rawExpectedSegmentForInstrumentedServer,
					actualSeg, testCase+": unmarshalled server segment is different from the expected")
			},
		},
	}

	for _, tc := range tests {
		content, err := ioutil.ReadFile(tc.samplePath)
		assert.NoError(t, err, fmt.Sprintf("[%s] can not read raw segment", tc.testCase))

		assert.True(t, len(content) > 0, fmt.Sprintf("[%s] content length is 0", tc.testCase))

		var actualSeg Segment
		err = json.Unmarshal(content, &actualSeg)

		tc.verification(tc.testCase, actualSeg, err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		testCase         string
		input            *Segment
		expectedErrorStr string
	}{
		{
			testCase:         "missing segment name",
			input:            &Segment{},
			expectedErrorStr: `segment "name" can not be nil`,
		},
		{
			testCase: "missing segment id",
			input: &Segment{
				Name: String("a name"),
			},
			expectedErrorStr: `segment "id" can not be nil`,
		},
		{
			testCase: "missing segment start_time",
			input: &Segment{
				Name: String("a name"),
				ID:   String("an ID"),
			},
			expectedErrorStr: `segment "start_time" can not be nil`,
		},
		{
			testCase: "missing segment trace_id",
			input: &Segment{
				Name:      String("a name"),
				ID:        String("an ID"),
				StartTime: Float64(10),
			},
			expectedErrorStr: `segment "trace_id" can not be nil`,
		},
		{
			testCase: "happy case",
			input: &Segment{
				Name:      String("a name"),
				ID:        String("an ID"),
				StartTime: Float64(10),
				TraceID:   String("a traceID"),
			},
		},
	}

	for _, tc := range tests {
		err := tc.input.Validate()

		if len(tc.expectedErrorStr) > 0 {
			assert.EqualError(t, err, tc.expectedErrorStr)
		} else {
			assert.NoError(t, err, "Validate should not fail")
		}
	}
}

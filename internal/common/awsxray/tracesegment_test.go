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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

var rawExpectedSegmentForInstrumentedApp = Segment{
	Name:      aws.String("DDB"),
	ID:        aws.String("88ad1df59cd7a7be"),
	StartTime: aws.Float64(1596566305.535414),
	TraceID:   aws.String("1-5f29ab21-d4ebf299219a65bd5c31d6da"),
	EndTime:   aws.Float64(1596566305.5928545),
	Fault:     aws.Bool(true),
	User:      aws.String("xraysegmentdump"),
	Cause: &CauseData{
		Type: CauseTypeObject,
		CauseObject: CauseObject{
			WorkingDirectory: aws.String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
			Exceptions: []Exception{
				{
					ID:      aws.String("3e9e11e3ab3fba60"),
					Message: aws.String("ResourceNotFoundException: Requested resource not found"),
					Type:    aws.String("dynamodb.ResourceNotFoundException"),
					Remote:  aws.Bool(true),
					Stack: []StackFrame{
						{
							Path:  aws.String("runtime/proc.go"),
							Line:  aws.Int(203),
							Label: aws.String("main"),
						},
						{
							Path:  aws.String("runtime/asm_amd64.s"),
							Line:  aws.Int(1373),
							Label: aws.String("goexit"),
						},
					},
				},
			},
		},
	},
	AWS: &AWSData{
		XRay: &XRayMetaData{
			SDKVersion: aws.String("1.1.0"),
			SDK:        aws.String("X-Ray for Go"),
		},
	},
	Service: &ServiceData{
		CompilerVersion: aws.String("go1.14.6"),
		Compiler:        aws.String("gc"),
	},
	Subsegments: []Segment{
		{
			Name:      aws.String("DDB.DescribeExistingTableAndPutToMissingTable"),
			ID:        aws.String("7df694142c905d8d"),
			StartTime: aws.Float64(1596566305.5354965),
			EndTime:   aws.Float64(1596566305.5928457),
			Fault:     aws.Bool(true),
			Cause: &CauseData{
				Type: CauseTypeObject,
				CauseObject: CauseObject{
					WorkingDirectory: aws.String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
					Exceptions: []Exception{
						{
							ID:      aws.String("e2ba8a2109451f5b"),
							Message: aws.String("ResourceNotFoundException: Requested resource not found"),
							Type:    aws.String("dynamodb.ResourceNotFoundException"),
							Remote:  aws.Bool(true),
							Stack: []StackFrame{
								{
									Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
									Line:  aws.Int(48),
									Label: aws.String("Capture"),
								},
								{
									Path:  aws.String("sampleapp/sample.go"),
									Line:  aws.Int(41),
									Label: aws.String("ddbExpectedFailure"),
								},
								{
									Path:  aws.String("sampleapp/sample.go"),
									Line:  aws.Int(36),
									Label: aws.String("main"),
								},
								{
									Path:  aws.String("runtime/proc.go"),
									Line:  aws.Int(203),
									Label: aws.String("main"),
								},
								{
									Path:  aws.String("runtime/asm_amd64.s"),
									Line:  aws.Int(1373),
									Label: aws.String("goexit"),
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
					Name:      aws.String("dynamodb"),
					ID:        aws.String("7318c46a385557f5"),
					StartTime: aws.Float64(1596566305.5355225),
					EndTime:   aws.Float64(1596566305.5873947),
					Namespace: aws.String("aws"),
					HTTP: &HTTPData{
						Response: &ResponseData{
							Status:        aws.Int64(200),
							ContentLength: aws.Int64(713),
						},
					},
					AWS: &AWSData{
						Operation:    aws.String("DescribeTable"),
						RemoteRegion: aws.String("us-west-2"),
						RequestID:    aws.String("29P5V7QSAKHS4LNL56ECAJFF3BVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						Retries:      aws.Int64(0),
						TableName:    aws.String("xray_sample_table"),
					},
					Subsegments: []Segment{
						{
							ID:        aws.String("0239834271dbee25"),
							Name:      aws.String("marshal"),
							StartTime: aws.Float64(1596566305.5355248),
							EndTime:   aws.Float64(1596566305.5355635),
						},
						{
							ID:        aws.String("23cf5bb60e4f66b1"),
							Name:      aws.String("attempt"),
							StartTime: aws.Float64(1596566305.5355663),
							EndTime:   aws.Float64(1596566305.5873196),
							Subsegments: []Segment{
								{
									ID:        aws.String("417b81b977b9563b"),
									Name:      aws.String("connect"),
									StartTime: aws.Float64(1596566305.5357504),
									EndTime:   aws.Float64(1596566305.575329),
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
											ID:        aws.String("0cab02b318413eb1"),
											Name:      aws.String("dns"),
											StartTime: aws.Float64(1596566305.5357957),
											EndTime:   aws.Float64(1596566305.5373216),
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
											ID:        aws.String("f8dbc5c6b291017e"),
											Name:      aws.String("dial"),
											StartTime: aws.Float64(1596566305.5373297),
											EndTime:   aws.Float64(1596566305.537964),
											Metadata: map[string]map[string]interface{}{
												"http": {
													"connect": map[string]interface{}{
														"network": "tcp",
													},
												},
											},
										},
										{
											ID:        aws.String("e2deb66ecaa769a5"),
											Name:      aws.String("tls"),
											StartTime: aws.Float64(1596566305.5380135),
											EndTime:   aws.Float64(1596566305.5753162),
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
									ID:        aws.String("a70bfab91597c7a2"),
									Name:      aws.String("request"),
									StartTime: aws.Float64(1596566305.5753367),
									EndTime:   aws.Float64(1596566305.5754144),
								},
								{
									ID:        aws.String("c05331c26d3e8a7f"),
									Name:      aws.String("response"),
									StartTime: aws.Float64(1596566305.5754204),
									EndTime:   aws.Float64(1596566305.5872962),
								},
							},
						},
						{
							ID:        aws.String("5fca2dfc9de81f4c"),
							Name:      aws.String("unmarshal"),
							StartTime: aws.Float64(1596566305.5873249),
							EndTime:   aws.Float64(1596566305.587389),
						},
					},
				},
				{
					Name:      aws.String("dynamodb"),
					ID:        aws.String("71631df3f58bdfc5"),
					StartTime: aws.Float64(1596566305.5874245),
					EndTime:   aws.Float64(1596566305.5928326),
					Fault:     aws.Bool(true),
					Cause: &CauseData{
						Type: CauseTypeObject,
						CauseObject: CauseObject{
							WorkingDirectory: aws.String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
							Exceptions: []Exception{
								{
									ID:      aws.String("7121b882a0ef44da"),
									Message: aws.String("ResourceNotFoundException: Requested resource not found"),
									Type:    aws.String("dynamodb.ResourceNotFoundException"),
									Remote:  aws.Bool(true),
									Stack: []StackFrame{
										{
											Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
											Line:  aws.Int(267),
											Label: aws.String("(*HandlerList).Run"),
										},
										{
											Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  aws.Int(515),
											Label: aws.String("(*Request).Send.func1"),
										},
										{
											Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  aws.Int(538),
											Label: aws.String("(*Request).Send"),
										},
										{
											Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
											Line:  aws.Int(3414),
											Label: aws.String("(*DynamoDB).PutItemWithContext"),
										},
										{
											Path:  aws.String("sampleapp/sample.go"),
											Line:  aws.Int(62),
											Label: aws.String("ddbExpectedFailure.func1"),
										},
										{
											Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
											Line:  aws.Int(45),
											Label: aws.String("Capture"),
										},
										{
											Path:  aws.String("sampleapp/sample.go"),
											Line:  aws.Int(41),
											Label: aws.String("ddbExpectedFailure"),
										},
										{
											Path:  aws.String("sampleapp/sample.go"),
											Line:  aws.Int(36),
											Label: aws.String("main"),
										},
										{
											Path:  aws.String("runtime/proc.go"),
											Line:  aws.Int(203),
											Label: aws.String("main"),
										},
										{
											Path:  aws.String("runtime/asm_amd64.s"),
											Line:  aws.Int(1373),
											Label: aws.String("goexit"),
										},
									},
								},
							},
						},
					},
					Namespace: aws.String("aws"),
					HTTP: &HTTPData{
						Response: &ResponseData{
							Status:        aws.Int64(400),
							ContentLength: aws.Int64(112),
						},
					},
					AWS: &AWSData{
						Operation:    aws.String("PutItem"),
						RemoteRegion: aws.String("us-west-2"),
						RequestID:    aws.String("TJUJNR0JV84CFHJL93D3GIA0LBVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						TableName:    aws.String("does_not_exist"),
						Retries:      aws.Int64(0),
					},
					Subsegments: []Segment{
						{
							Name:      aws.String("marshal"),
							ID:        aws.String("9da02fcbb9711b47"),
							StartTime: aws.Float64(1596566305.5874267),
							EndTime:   aws.Float64(1596566305.58745),
						},
						{
							Name:      aws.String("attempt"),
							ID:        aws.String("56b1cb185cbdb378"),
							StartTime: aws.Float64(1596566305.587453),
							EndTime:   aws.Float64(1596566305.592767),
							Fault:     aws.Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: aws.String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      aws.String("59de8ae27660d21d"),
											Message: aws.String("ResourceNotFoundException: Requested resource not found"),
											Type:    aws.String("dynamodb.ResourceNotFoundException"),
											Remote:  aws.Bool(true),
											Stack: []StackFrame{
												{
													Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/aws.go"),
													Line:  aws.Int(139),
													Label: aws.String("glob..func7"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  aws.Int(267),
													Label: aws.String("(*HandlerList).Run"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  aws.Int(534),
													Label: aws.String("(*Request).Send"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  aws.Int(3414),
													Label: aws.String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(62),
													Label: aws.String("ddbExpectedFailure.func1"),
												},
												{
													Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  aws.Int(45),
													Label: aws.String("Capture"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(41),
													Label: aws.String("ddbExpectedFailure"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(36),
													Label: aws.String("main"),
												},
												{
													Path:  aws.String("runtime/proc.go"),
													Line:  aws.Int(203),
													Label: aws.String("main"),
												},
												{
													Path:  aws.String("runtime/asm_amd64.s"),
													Line:  aws.Int(1373),
													Label: aws.String("goexit"),
												},
											},
										},
									},
								},
							},
							Subsegments: []Segment{
								{
									Name:      aws.String("request"),
									ID:        aws.String("6f908a1d3ec70abe"),
									StartTime: aws.Float64(1596566305.5875077),
									EndTime:   aws.Float64(1596566305.587543),
								},
								{
									Name:      aws.String("response"),
									ID:        aws.String("acfaa7e3fe3aab03"),
									StartTime: aws.Float64(1596566305.5875454),
									EndTime:   aws.Float64(1596566305.592695),
								},
							},
						},
						{
							Name:      aws.String("wait"),
							ID:        aws.String("ba8d350c0e8cdc4b"),
							StartTime: aws.Float64(1596566305.592807),
							EndTime:   aws.Float64(1596566305.5928102),
							Fault:     aws.Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: aws.String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      aws.String("5a07f08a8c260405"),
											Message: aws.String("ResourceNotFoundException: Requested resource not found"),
											Type:    aws.String("dynamodb.ResourceNotFoundException"),
											Remote:  aws.Bool(true),
											Stack: []StackFrame{
												{
													Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/aws.go"),
													Line:  aws.Int(149),
													Label: aws.String("glob..func8"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  aws.Int(267),
													Label: aws.String("(*HandlerList).Run"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  aws.Int(535),
													Label: aws.String("(*Request).Send"),
												},
												{
													Path:  aws.String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  aws.Int(3414),
													Label: aws.String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(62),
													Label: aws.String("ddbExpectedFailure.func1"),
												},
												{
													Path:  aws.String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  aws.Int(45),
													Label: aws.String("Capture"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(41),
													Label: aws.String("ddbExpectedFailure"),
												},
												{
													Path:  aws.String("sampleapp/sample.go"),
													Line:  aws.Int(36),
													Label: aws.String("main"),
												},
												{
													Path:  aws.String("runtime/proc.go"),
													Line:  aws.Int(203),
													Label: aws.String("main"),
												},
												{
													Path:  aws.String("runtime/asm_amd64.s"),
													Line:  aws.Int(1373),
													Label: aws.String("goexit"),
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
	Name:      aws.String("SampleServer"),
	ID:        aws.String("bda182a644eee9b3"),
	StartTime: aws.Float64(1596648396.6399446),
	TraceID:   aws.String("1-5f2aebcc-b475d14618c51eaa28753d37"),
	EndTime:   aws.Float64(1596648396.6401389),
	HTTP: &HTTPData{
		Request: &RequestData{
			Method:        aws.String("GET"),
			URL:           aws.String("http://localhost:8000/"),
			ClientIP:      aws.String("127.0.0.1"),
			UserAgent:     aws.String("Go-http-client/1.1"),
			XForwardedFor: aws.Bool(true),
		},
		Response: &ResponseData{
			Status: aws.Int64(200),
		},
	},
	AWS: &AWSData{
		XRay: &XRayMetaData{
			SDKVersion: aws.String("1.1.0"),
			SDK:        aws.String("X-Ray for Go"),
		},
	},
	Service: &ServiceData{
		CompilerVersion: aws.String("go1.14.6"),
		Compiler:        aws.String("gc"),
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
					Name:       aws.String("LongOperation"),
					ID:         aws.String("5cc4a447f5d4d696"),
					StartTime:  aws.Float64(1595437651.680097),
					TraceID:    aws.String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					InProgress: aws.Bool(true),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyOtherTopLevelFieldsUnmarshalled",
			samplePath: path.Join("testdata", "minOtherFields.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:        aws.String("OtherTopLevelFields"),
					ID:          aws.String("5cc4a447f5d4d696"),
					StartTime:   aws.Float64(1595437651.680097),
					EndTime:     aws.Float64(1595437652.197392),
					TraceID:     aws.String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Error:       aws.Bool(false),
					Throttle:    aws.Bool(true),
					ResourceARN: aws.String("chicken"),
					Origin:      aws.String("AWS::EC2::Instance"),
					ParentID:    aws.String("defdfd9912dc5a56"),
					Type:        aws.String("subsegment"),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyCauseIsExceptionIdUnmarshalled",
			samplePath: path.Join("testdata", "minCauseIsExceptionId.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:      aws.String("CauseIsExceptionID"),
					ID:        aws.String("5cc4a447f5d4d696"),
					StartTime: aws.Float64(1595437651.680097),
					EndTime:   aws.Float64(1595437652.197392),
					TraceID:   aws.String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Fault:     aws.Bool(true),
					Cause: &CauseData{
						Type:        CauseTypeExceptionID,
						ExceptionID: aws.String("abcdefghijklmnop"),
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
				Name: aws.String("a name"),
			},
			expectedErrorStr: `segment "id" can not be nil`,
		},
		{
			testCase: "missing segment start_time",
			input: &Segment{
				Name: aws.String("a name"),
				ID:   aws.String("an ID"),
			},
			expectedErrorStr: `segment "start_time" can not be nil`,
		},
		{
			testCase: "missing segment trace_id",
			input: &Segment{
				Name:      aws.String("a name"),
				ID:        aws.String("an ID"),
				StartTime: aws.Float64(10),
			},
			expectedErrorStr: `segment "trace_id" can not be nil`,
		},
		{
			testCase: "happy case",
			input: &Segment{
				Name:      aws.String("a name"),
				ID:        aws.String("an ID"),
				StartTime: aws.Float64(10),
				TraceID:   aws.String("a traceID"),
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

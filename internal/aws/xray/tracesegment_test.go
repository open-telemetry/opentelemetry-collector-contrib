// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
)

var rawExpectedSegmentForInstrumentedApp = Segment{
	Name:      String("DDB"),
	ID:        String("88ad1df59cd7a7be"),
	StartTime: aws.Float64(1596566305.535414),
	TraceID:   String("1-5f29ab21-d4ebf299219a65bd5c31d6da"),
	EndTime:   aws.Float64(1596566305.5928545),
	Fault:     aws.Bool(true),
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
					Remote:  aws.Bool(true),
					Stack: []StackFrame{
						{
							Path:  String("runtime/proc.go"),
							Line:  aws.Int(203),
							Label: String("main"),
						},
						{
							Path:  String("runtime/asm_amd64.s"),
							Line:  aws.Int(1373),
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
			StartTime: aws.Float64(1596566305.5354965),
			EndTime:   aws.Float64(1596566305.5928457),
			Fault:     aws.Bool(true),
			Cause: &CauseData{
				Type: CauseTypeObject,
				CauseObject: CauseObject{
					WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
					Exceptions: []Exception{
						{
							ID:      String("e2ba8a2109451f5b"),
							Message: String("ResourceNotFoundException: Requested resource not found"),
							Type:    String("dynamodb.ResourceNotFoundException"),
							Remote:  aws.Bool(true),
							Stack: []StackFrame{
								{
									Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
									Line:  aws.Int(48),
									Label: String("Capture"),
								},
								{
									Path:  String("sampleapp/sample.go"),
									Line:  aws.Int(41),
									Label: String("ddbExpectedFailure"),
								},
								{
									Path:  String("sampleapp/sample.go"),
									Line:  aws.Int(36),
									Label: String("main"),
								},
								{
									Path:  String("runtime/proc.go"),
									Line:  aws.Int(203),
									Label: String("main"),
								},
								{
									Path:  String("runtime/asm_amd64.s"),
									Line:  aws.Int(1373),
									Label: String("goexit"),
								},
							},
						},
					},
				},
			},
			Annotations: map[string]any{
				"DDB.DescribeExistingTableAndPutToMissingTable.Annotation": "anno",
			},
			Metadata: map[string]map[string]any{
				"default": {
					"DDB.DescribeExistingTableAndPutToMissingTable.AddMetadata": "meta",
				},
			},
			Subsegments: []Segment{
				{
					Name:      String("dynamodb"),
					ID:        String("7318c46a385557f5"),
					StartTime: aws.Float64(1596566305.5355225),
					EndTime:   aws.Float64(1596566305.5873947),
					Namespace: String("aws"),
					HTTP: &HTTPData{
						Response: &ResponseData{
							Status:        aws.Int64(200),
							ContentLength: *aws.Float64(713),
						},
					},
					AWS: &AWSData{
						Operation:    String("DescribeTable"),
						RemoteRegion: String("us-west-2"),
						RequestID:    String("29P5V7QSAKHS4LNL56ECAJFF3BVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						Retries:      aws.Int64(0),
						TableName:    String("xray_sample_table"),
					},
					Subsegments: []Segment{
						{
							ID:        String("0239834271dbee25"),
							Name:      String("marshal"),
							StartTime: aws.Float64(1596566305.5355248),
							EndTime:   aws.Float64(1596566305.5355635),
						},
						{
							ID:        String("23cf5bb60e4f66b1"),
							Name:      String("attempt"),
							StartTime: aws.Float64(1596566305.5355663),
							EndTime:   aws.Float64(1596566305.5873196),
							Subsegments: []Segment{
								{
									ID:        String("417b81b977b9563b"),
									Name:      String("connect"),
									StartTime: aws.Float64(1596566305.5357504),
									EndTime:   aws.Float64(1596566305.575329),
									Metadata: map[string]map[string]any{
										"http": {
											"connection": map[string]any{
												"reused":   false,
												"was_idle": false,
											},
										},
									},
									Subsegments: []Segment{
										{
											ID:        String("0cab02b318413eb1"),
											Name:      String("dns"),
											StartTime: aws.Float64(1596566305.5357957),
											EndTime:   aws.Float64(1596566305.5373216),
											Metadata: map[string]map[string]any{
												"http": {
													"dns": map[string]any{
														"addresses": []any{
															map[string]any{
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
											StartTime: aws.Float64(1596566305.5373297),
											EndTime:   aws.Float64(1596566305.537964),
											Metadata: map[string]map[string]any{
												"http": {
													"connect": map[string]any{
														"network": "tcp",
													},
												},
											},
										},
										{
											ID:        String("e2deb66ecaa769a5"),
											Name:      String("tls"),
											StartTime: aws.Float64(1596566305.5380135),
											EndTime:   aws.Float64(1596566305.5753162),
											Metadata: map[string]map[string]any{
												"http": {
													"tls": map[string]any{
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
									StartTime: aws.Float64(1596566305.5753367),
									EndTime:   aws.Float64(1596566305.5754144),
								},
								{
									ID:        String("c05331c26d3e8a7f"),
									Name:      String("response"),
									StartTime: aws.Float64(1596566305.5754204),
									EndTime:   aws.Float64(1596566305.5872962),
								},
							},
						},
						{
							ID:        String("5fca2dfc9de81f4c"),
							Name:      String("unmarshal"),
							StartTime: aws.Float64(1596566305.5873249),
							EndTime:   aws.Float64(1596566305.587389),
						},
					},
				},
				{
					Name:      String("dynamodb"),
					ID:        String("71631df3f58bdfc5"),
					StartTime: aws.Float64(1596566305.5874245),
					EndTime:   aws.Float64(1596566305.5928326),
					Fault:     aws.Bool(true),
					Cause: &CauseData{
						Type: CauseTypeObject,
						CauseObject: CauseObject{
							WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
							Exceptions: []Exception{
								{
									ID:      String("7121b882a0ef44da"),
									Message: String("ResourceNotFoundException: Requested resource not found"),
									Type:    String("dynamodb.ResourceNotFoundException"),
									Remote:  aws.Bool(true),
									Stack: []StackFrame{
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
											Line:  aws.Int(267),
											Label: String("(*HandlerList).Run"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  aws.Int(515),
											Label: String("(*Request).Send.func1"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
											Line:  aws.Int(538),
											Label: String("(*Request).Send"),
										},
										{
											Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
											Line:  aws.Int(3414),
											Label: String("(*DynamoDB).PutItemWithContext"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  aws.Int(62),
											Label: String("ddbExpectedFailure.func1"),
										},
										{
											Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
											Line:  aws.Int(45),
											Label: String("Capture"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  aws.Int(41),
											Label: String("ddbExpectedFailure"),
										},
										{
											Path:  String("sampleapp/sample.go"),
											Line:  aws.Int(36),
											Label: String("main"),
										},
										{
											Path:  String("runtime/proc.go"),
											Line:  aws.Int(203),
											Label: String("main"),
										},
										{
											Path:  String("runtime/asm_amd64.s"),
											Line:  aws.Int(1373),
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
							Status:        aws.Int64(400),
							ContentLength: *aws.Float64(112),
						},
					},
					AWS: &AWSData{
						Operation:    String("PutItem"),
						RemoteRegion: String("us-west-2"),
						RequestID:    String("TJUJNR0JV84CFHJL93D3GIA0LBVV4KQNSO5AEMVJF66Q9ASUAAJG"),
						TableName:    String("does_not_exist"),
						Retries:      aws.Int64(0),
					},
					Subsegments: []Segment{
						{
							Name:      String("marshal"),
							ID:        String("9da02fcbb9711b47"),
							StartTime: aws.Float64(1596566305.5874267),
							EndTime:   aws.Float64(1596566305.58745),
						},
						{
							Name:      String("attempt"),
							ID:        String("56b1cb185cbdb378"),
							StartTime: aws.Float64(1596566305.587453),
							EndTime:   aws.Float64(1596566305.592767),
							Fault:     aws.Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      String("59de8ae27660d21d"),
											Message: String("ResourceNotFoundException: Requested resource not found"),
											Type:    String("dynamodb.ResourceNotFoundException"),
											Remote:  aws.Bool(true),
											Stack: []StackFrame{
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/aws.go"),
													Line:  aws.Int(139),
													Label: String("glob..func7"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  aws.Int(267),
													Label: String("(*HandlerList).Run"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  aws.Int(534),
													Label: String("(*Request).Send"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  aws.Int(3414),
													Label: String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(62),
													Label: String("ddbExpectedFailure.func1"),
												},
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  aws.Int(45),
													Label: String("Capture"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(41),
													Label: String("ddbExpectedFailure"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(36),
													Label: String("main"),
												},
												{
													Path:  String("runtime/proc.go"),
													Line:  aws.Int(203),
													Label: String("main"),
												},
												{
													Path:  String("runtime/asm_amd64.s"),
													Line:  aws.Int(1373),
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
									StartTime: aws.Float64(1596566305.5875077),
									EndTime:   aws.Float64(1596566305.587543),
								},
								{
									Name:      String("response"),
									ID:        String("acfaa7e3fe3aab03"),
									StartTime: aws.Float64(1596566305.5875454),
									EndTime:   aws.Float64(1596566305.592695),
								},
							},
						},
						{
							Name:      String("wait"),
							ID:        String("ba8d350c0e8cdc4b"),
							StartTime: aws.Float64(1596566305.592807),
							EndTime:   aws.Float64(1596566305.5928102),
							Fault:     aws.Bool(true),
							Cause: &CauseData{
								Type: CauseTypeObject,
								CauseObject: CauseObject{
									WorkingDirectory: String("/home/ubuntu/opentelemetry-collector-contrib/receiver/awsxrayreceiver/testdata/rawsegment/sampleapp"),
									Exceptions: []Exception{
										{
											ID:      String("5a07f08a8c260405"),
											Message: String("ResourceNotFoundException: Requested resource not found"),
											Type:    String("dynamodb.ResourceNotFoundException"),
											Remote:  aws.Bool(true),
											Stack: []StackFrame{
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/aws.go"),
													Line:  aws.Int(149),
													Label: String("glob..func8"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/handlers.go"),
													Line:  aws.Int(267),
													Label: String("(*HandlerList).Run"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/aws/request/request.go"),
													Line:  aws.Int(535),
													Label: String("(*Request).Send"),
												},
												{
													Path:  String("github.com/aws/aws-sdk-go@v1.33.9/service/dynamodb/api.go"),
													Line:  aws.Int(3414),
													Label: String("(*DynamoDB).PutItemWithContext"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(62),
													Label: String("ddbExpectedFailure.func1"),
												},
												{
													Path:  String("github.com/aws/aws-xray-sdk-go@v1.1.0/xray/capture.go"),
													Line:  aws.Int(45),
													Label: String("Capture"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(41),
													Label: String("ddbExpectedFailure"),
												},
												{
													Path:  String("sampleapp/sample.go"),
													Line:  aws.Int(36),
													Label: String("main"),
												},
												{
													Path:  String("runtime/proc.go"),
													Line:  aws.Int(203),
													Label: String("main"),
												},
												{
													Path:  String("runtime/asm_amd64.s"),
													Line:  aws.Int(1373),
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
	StartTime: aws.Float64(1596648396.6399446),
	TraceID:   String("1-5f2aebcc-b475d14618c51eaa28753d37"),
	EndTime:   aws.Float64(1596648396.6401389),
	HTTP: &HTTPData{
		Request: &RequestData{
			Method:        String("GET"),
			URL:           String("http://localhost:8000/"),
			ClientIP:      String("127.0.0.1"),
			UserAgent:     String("Go-http-client/1.1"),
			XForwardedFor: aws.Bool(true),
		},
		Response: &ResponseData{
			Status: aws.Int64(200),
		},
	},
	AWS: &AWSData{
		XRay: &XRayMetaData{
			SDKVersion: String("1.1.0"),
			SDK:        String("X-Ray for Go"),
		},
		EKS: &EKSMetadata{
			String("containerName"),
			String("podname"),
			String("d8453812a556"),
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
			samplePath: filepath.Join("testdata", "ddbSample.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, rawExpectedSegmentForInstrumentedApp,
					actualSeg, testCase+": unmarshalled app segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyInProgressUnmarshalled",
			samplePath: filepath.Join("testdata", "minInProgress.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:       String("LongOperation"),
					ID:         String("5cc4a447f5d4d696"),
					StartTime:  aws.Float64(1595437651.680097),
					TraceID:    String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					InProgress: aws.Bool(true),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyOtherTopLevelFieldsUnmarshalled",
			samplePath: filepath.Join("testdata", "minOtherFields.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:        String("OtherTopLevelFields"),
					ID:          String("5cc4a447f5d4d696"),
					StartTime:   aws.Float64(1595437651.680097),
					EndTime:     aws.Float64(1595437652.197392),
					TraceID:     String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Error:       aws.Bool(false),
					Throttle:    aws.Bool(true),
					ResourceARN: String("chicken"),
					Origin:      String("AWS::EC2::Instance"),
					ParentID:    String("defdfd9912dc5a56"),
					Type:        String("subsegment"),
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")
			},
		},
		{
			testCase:   "TestTraceBodyCauseIsExceptionIdUnmarshalled",
			samplePath: filepath.Join("testdata", "minCauseIsExceptionId.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, Segment{
					Name:      String("CauseIsExceptionID"),
					ID:        String("5cc4a447f5d4d696"),
					StartTime: aws.Float64(1595437651.680097),
					EndTime:   aws.Float64(1595437652.197392),
					TraceID:   String("1-5f187253-6a106696d56b1f4ef9eba2ed"),
					Fault:     aws.Bool(true),
					Cause: &CauseData{
						Type:        CauseTypeExceptionID,
						ExceptionID: String("abcdefghijklmnop"),
					},
				}, actualSeg, testCase+": unmarshalled segment is different from the expected")

			},
		},
		{
			testCase:   "TestTraceBodyInvalidCauseUnmarshalled",
			samplePath: filepath.Join("testdata", "minCauseIsInvalid.txt"),
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
			samplePath: filepath.Join("testdata", "serverSample.txt"),
			verification: func(testCase string, actualSeg Segment, err error) {
				assert.NoError(t, err, testCase+": JSON Unmarshalling should've succeeded")

				assert.Equal(t, rawExpectedSegmentForInstrumentedServer,
					actualSeg, testCase+": unmarshalled server segment is different from the expected")
			},
		},
	}

	for _, tc := range tests {
		content, err := os.ReadFile(tc.samplePath)
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
				StartTime: aws.Float64(10),
			},
			expectedErrorStr: `segment "trace_id" can not be nil`,
		},
		{
			testCase: "happy case",
			input: &Segment{
				Name:      String("a name"),
				ID:        String("an ID"),
				StartTime: aws.Float64(10),
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

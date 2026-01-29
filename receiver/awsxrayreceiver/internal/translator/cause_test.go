// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestConvertStackFramesToStackTraceStr(t *testing.T) {
	excp := awsxray.Exception{
		Type:    awsxray.String("exceptionType"),
		Message: awsxray.String("exceptionMessage"),
		Stack: []awsxray.StackFrame{
			{
				Path:  awsxray.String("path0"),
				Line:  aws.Int(10),
				Label: awsxray.String("label0"),
			},
			{
				Path:  awsxray.String("path1"),
				Line:  aws.Int(11),
				Label: awsxray.String("label1"),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, "exceptionType: exceptionMessage\n\tat label0(path0: 10)\n\tat label1(path1: 11)\n", actual)
}

func TestConvertStackFramesToStackTraceStrNoPath(t *testing.T) {
	excp := awsxray.Exception{
		Type:    awsxray.String("exceptionType"),
		Message: awsxray.String("exceptionMessage"),
		Stack: []awsxray.StackFrame{
			{
				Path:  awsxray.String("path0"),
				Line:  aws.Int(10),
				Label: awsxray.String("label0"),
			},
			{
				Line:  aws.Int(11),
				Label: awsxray.String("label1"),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, "exceptionType: exceptionMessage\n\tat label0(path0: 10)\n\tat label1(: 11)\n", actual)
}

func TestConvertStackFramesToStackTraceStrNoLine(t *testing.T) {
	excp := awsxray.Exception{
		Type:    awsxray.String("exceptionType"),
		Message: awsxray.String("exceptionMessage"),
		Stack: []awsxray.StackFrame{
			{
				Path:  awsxray.String("path0"),
				Line:  aws.Int(10),
				Label: awsxray.String("label0"),
			},
			{
				Path:  awsxray.String("path1"),
				Label: awsxray.String("label1"),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, "exceptionType: exceptionMessage\n\tat label0(path0: 10)\n\tat label1(path1: <unknown>)\n", actual)
}

func TestConvertStackFramesToStackTraceStrNoLabel(t *testing.T) {
	excp := awsxray.Exception{
		Type:    awsxray.String("exceptionType"),
		Message: awsxray.String("exceptionMessage"),
		Stack: []awsxray.StackFrame{
			{
				Path:  awsxray.String("path0"),
				Line:  aws.Int(10),
				Label: awsxray.String("label0"),
			},
			{
				Path: awsxray.String("path1"),
				Line: aws.Int(11),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, "exceptionType: exceptionMessage\n\tat label0(path0: 10)\n\tat (path1: 11)\n", actual)
}

func TestConvertStackFramesToStackTraceStrNoErrorMessage(t *testing.T) {
	excp := awsxray.Exception{
		Stack: []awsxray.StackFrame{
			{
				Path:  awsxray.String("path0"),
				Line:  aws.Int(10),
				Label: awsxray.String("label0"),
			},
			{
				Path: awsxray.String("path1"),
				Line: aws.Int(11),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, ": \n\tat label0(path0: 10)\n\tat (path1: 11)\n", actual)
}

func TestAddCause(t *testing.T) {
	exceptionType := "TestException"
	exceptionMessage := "test exception message"

	seg := &awsxray.Segment{
		Cause: &awsxray.CauseData{
			Type: awsxray.CauseTypeObject,
			CauseObject: awsxray.CauseObject{
				Exceptions: []awsxray.Exception{
					{
						// ID is intentionally nil
						Type:    &exceptionType,
						Message: &exceptionMessage,
					},
				},
			},
		},
	}

	span := ptrace.NewSpan()

	addCause(seg, span)

	assert.Equal(t, 1, span.Events().Len())

	evt := span.Events().At(0)
	assert.Equal(t, ExceptionEventName, evt.Name())

	attrs := evt.Attributes()
	typeVal, typeExists := attrs.Get("exception.type")
	assert.True(t, typeExists)
	assert.Equal(t, exceptionType, typeVal.Str())

	msgVal, msgExists := attrs.Get("exception.message")
	assert.True(t, msgExists)
	assert.Equal(t, exceptionMessage, msgVal.Str())

	_, idExists := attrs.Get(awsxray.AWSXrayExceptionIDAttribute)
	assert.False(t, idExists, "exception ID attribute should not be set when ID is nil")
}

func TestAddCauseWithExceptionWithID(t *testing.T) {
	exceptionID := "exception123"
	exceptionType := "TestException"
	exceptionMessage := "test exception message"

	seg := &awsxray.Segment{
		Cause: &awsxray.CauseData{
			Type: awsxray.CauseTypeObject,
			CauseObject: awsxray.CauseObject{
				Exceptions: []awsxray.Exception{
					{
						ID:      &exceptionID,
						Type:    &exceptionType,
						Message: &exceptionMessage,
					},
				},
			},
		},
	}

	span := ptrace.NewSpan()

	addCause(seg, span)

	assert.Equal(t, 1, span.Events().Len())

	evt := span.Events().At(0)
	attrs := evt.Attributes()

	idVal, idExists := attrs.Get(awsxray.AWSXrayExceptionIDAttribute)
	assert.True(t, idExists, "exception ID attribute should be set when ID is provided")
	assert.Equal(t, exceptionID, idVal.Str())
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	seg := &awsxray.Segment{
		Cause: &awsxray.CauseData{
			Type: awsxray.CauseTypeObject,
			CauseObject: awsxray.CauseObject{
				Exceptions: []awsxray.Exception{
					{
						// ID is nil
						Message: awsxray.String("test error message"),
						Type:    awsxray.String("TestException"),
					},
				},
			},
		},
	}

	span := ptrace.NewSpan()

	require.NotPanics(t, func() {
		addCause(seg, span)
	})

	require.Equal(t, 1, span.Events().Len())

	evt := span.Events().At(0)
	assert.Equal(t, ExceptionEventName, evt.Name())

	_, exists := evt.Attributes().Get(awsxray.AWSXrayExceptionIDAttribute)
	assert.False(t, exists, "exception ID attribute should not be set when ID is nil")

	msgVal, exists := evt.Attributes().Get("exception.message")
	require.True(t, exists)
	assert.Equal(t, "test error message", msgVal.Str())

	typeVal, exists := evt.Attributes().Get("exception.type")
	require.True(t, exists)
	assert.Equal(t, "TestException", typeVal.Str())
}

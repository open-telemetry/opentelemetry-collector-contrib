// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

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

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"bufio"
	"encoding/hex"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv112 "go.opentelemetry.io/otel/semconv/v1.12.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

// ExceptionEventName the name of the exception event.
// TODO: Remove this when collector defines this semantic convention.
const (
	ExceptionEventName              = "exception"
	AwsIndividualHTTPEventName      = "HTTP request failure"
	AwsIndividualHTTPErrorEventType = "aws.http.error.event"
	AwsIndividualHTTPErrorMsgAttr   = "aws.http.error_message"
)

func makeCause(span ptrace.Span, attributes map[string]pcommon.Value, resource pcommon.Resource) (isError, isFault, isThrottle bool,
	filtered map[string]pcommon.Value, cause *awsxray.CauseData,
) {
	status := span.Status()

	filtered = attributes

	var (
		message   string
		errorKind string
	)

	isAwsSdkSpan := isAwsSdkSpan(span)
	hasExceptionEvents := false
	hasAwsIndividualHTTPError := false
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		if event.Name() == ExceptionEventName {
			hasExceptionEvents = true
			break
		}
		if isAwsSdkSpan && event.Name() == AwsIndividualHTTPEventName {
			hasAwsIndividualHTTPError = true
			break
		}
	}
	hasExceptions := hasExceptionEvents || hasAwsIndividualHTTPError

	switch {
	case hasExceptions:
		language := ""
		if val, ok := resource.Attributes().Get(string(conventionsv112.TelemetrySDKLanguageKey)); ok {
			language = val.Str()
		}
		isRemote := false
		if span.Kind() == ptrace.SpanKindClient || span.Kind() == ptrace.SpanKindProducer {
			isRemote = true
		}

		var exceptions []awsxray.Exception
		for i := 0; i < span.Events().Len(); i++ {
			event := span.Events().At(i)
			if event.Name() == ExceptionEventName {
				exceptionType := ""
				message = ""
				stacktrace := ""

				if val, ok := event.Attributes().Get(string(conventionsv112.ExceptionTypeKey)); ok {
					exceptionType = val.Str()
				}

				if val, ok := event.Attributes().Get(string(conventionsv112.ExceptionMessageKey)); ok {
					message = val.Str()
				}

				if val, ok := event.Attributes().Get(string(conventionsv112.ExceptionStacktraceKey)); ok {
					stacktrace = val.Str()
				}

				parsed := parseException(exceptionType, message, stacktrace, isRemote, language)
				exceptions = append(exceptions, parsed...)
			} else if isAwsSdkSpan && event.Name() == AwsIndividualHTTPEventName {
				errorCode, ok1 := event.Attributes().Get(string(conventions.HTTPResponseStatusCodeKey))
				errorMessage, ok2 := event.Attributes().Get(AwsIndividualHTTPErrorMsgAttr)
				if ok1 && ok2 {
					eventEpochTime := event.Timestamp().AsTime().UnixMicro()
					strs := []string{
						errorCode.AsString(),
						strconv.FormatFloat(float64(eventEpochTime)/1_000_000, 'f', 6, 64),
						errorMessage.Str(),
					}
					message = strings.Join(strs, "@")
					segmentID := newSegmentID()
					exception := awsxray.Exception{
						ID:      aws.String(hex.EncodeToString(segmentID[:])),
						Type:    aws.String(AwsIndividualHTTPErrorEventType),
						Remote:  aws.Bool(true),
						Message: aws.String(message),
					}
					exceptions = append(exceptions, exception)
				}
			}
		}
		cause = &awsxray.CauseData{
			Type: awsxray.CauseTypeObject,
			CauseObject: awsxray.CauseObject{
				Exceptions: exceptions,
			},
		}

	case status.Code() != ptrace.StatusCodeError:
		cause = nil

	default:
		// Use OpenCensus behavior if we didn't find any exception events to ease migration.
		message = status.Message()
		filtered = make(map[string]pcommon.Value)
		for key, value := range attributes {
			switch key {
			case "http.status_text":
				if message == "" {
					message = value.Str()
				}
			default:
				filtered[key] = value
			}
		}

		if message != "" {
			segmentID := newSegmentID()
			cause = &awsxray.CauseData{
				Type: awsxray.CauseTypeObject,
				CauseObject: awsxray.CauseObject{
					Exceptions: []awsxray.Exception{
						{
							ID:      aws.String(hex.EncodeToString(segmentID[:])),
							Type:    aws.String(errorKind),
							Message: aws.String(message),
						},
					},
				},
			}
		}
	}

	val, ok := span.Attributes().Get(string(conventionsv112.HTTPStatusCodeKey))
	if !ok {
		val, ok = span.Attributes().Get(string(conventions.HTTPResponseStatusCodeKey))
	}

	// The segment status for http spans will be based on their http.statuscode as we found some http
	// spans does not fill with status.Code() but always filled with http.statuscode
	var code int64
	if ok {
		code = val.Int()
	}

	// Default values
	isThrottle = false
	isError = false
	isFault = false

	switch {
	case !ok || code < 400 || code > 599:
		if status.Code() == ptrace.StatusCodeError {
			isFault = true
		}
	case code >= 400 && code <= 499:
		isError = true
		if code == 429 {
			isThrottle = true
		}
	case code >= 500 && code <= 599:
		isFault = true
	}

	return isError, isFault, isThrottle, filtered, cause
}

func parseException(exceptionType string, message string, stacktrace string, isRemote bool, language string) []awsxray.Exception {
	exceptions := make([]awsxray.Exception, 0, 1)
	segmentID := newSegmentID()
	exceptions = append(exceptions, awsxray.Exception{
		ID:      aws.String(hex.EncodeToString(segmentID[:])),
		Type:    aws.String(exceptionType),
		Remote:  aws.Bool(isRemote),
		Message: aws.String(message),
	})

	if stacktrace == "" {
		return exceptions
	}

	switch language {
	case "java":
		exceptions = fillJavaStacktrace(stacktrace, exceptions)
	case "python":
		exceptions = fillPythonStacktrace(stacktrace, exceptions)
	case "javascript":
		exceptions = fillJavaScriptStacktrace(stacktrace, exceptions)
	case "dotnet":
		exceptions = fillDotnetStacktrace(stacktrace, exceptions)
	case "php":
		// The PHP SDK formats stack traces exactly like Java would
		exceptions = fillJavaStacktrace(stacktrace, exceptions)
	case "go":
		exceptions = fillGoStacktrace(stacktrace, exceptions)
	}

	return exceptions
}

func fillJavaStacktrace(stacktrace string, exceptions []awsxray.Exception) []awsxray.Exception {
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(stacktrace)))

	// Skip first line containing top level message
	exception := &exceptions[0]
	isRemote := exception.Remote
	_, err := r.ReadLine()
	if err != nil {
		return exceptions
	}
	var line string
	line, err = r.ReadLine()
	if err != nil {
		return exceptions
	}

	exception.Stack = nil
	for {
		if strings.HasPrefix(line, "\tat ") {
			parenIdx := strings.IndexByte(line, '(')
			if parenIdx >= 0 && line[len(line)-1] == ')' {
				label := line[len("\tat "):parenIdx]
				slashIdx := strings.IndexByte(label, '/')
				if slashIdx >= 0 {
					// Class loader or Java module prefix, remove it
					label = label[slashIdx+1:]
				}

				path := line[parenIdx+1 : len(line)-1]
				line := 0

				colonIdx := strings.IndexByte(path, ':')
				if colonIdx >= 0 {
					lineStr := path[colonIdx+1:]
					path = path[0:colonIdx]
					line, _ = strconv.Atoi(lineStr)
				}

				stack := awsxray.StackFrame{
					Path:  aws.String(path),
					Label: aws.String(label),
					Line:  aws.Int(line),
				}

				exception.Stack = append(exception.Stack, stack)
			}
		} else if strings.HasPrefix(line, "Caused by: ") {
			causeType := line[len("Caused by: "):]
			colonIdx := strings.IndexByte(causeType, ':')
			causeMessage := ""
			if colonIdx >= 0 {
				// Skip space after colon too.
				causeMessage = causeType[colonIdx+2:]
				causeType = causeType[0:colonIdx]
			}
			for {
				// Need to peek lines since the message may have newlines.
				line, err = r.ReadLine()
				if err != nil {
					break
				}
				if strings.HasPrefix(line, "\tat ") && strings.IndexByte(line, '(') >= 0 && line[len(line)-1] == ')' {
					// Stack frame (hopefully, user can masquerade since we only have a string), process above.
					break
				}
				// String append overhead in this case, but multiline messages should be far less common than single
				// line ones.
				causeMessage += line
			}
			segmentID := newSegmentID()
			exceptions = append(exceptions, awsxray.Exception{
				ID:      aws.String(hex.EncodeToString(segmentID[:])),
				Type:    aws.String(causeType),
				Remote:  isRemote,
				Message: aws.String(causeMessage),
				Stack:   nil,
			})
			// when append causes `exceptions` to outgrow its existing
			// capacity, re-allocation will happen so the place
			// `exception` points to is no longer `exceptions[len(exceptions)-2]`,
			// consequently, we cannot write `exception.Cause = newException.ID`
			// below.
			newException := &exceptions[len(exceptions)-1]
			exceptions[len(exceptions)-2].Cause = newException.ID

			exception.Cause = newException.ID
			exception = newException
			// We peeked to a line starting with "\tat", a stack frame, so continue straight to processing.
			continue
		}
		// We skip "..." (common frames) and Suppressed By exceptions.
		line, err = r.ReadLine()
		if err != nil {
			break
		}
	}

	return exceptions
}

func fillPythonStacktrace(stacktrace string, exceptions []awsxray.Exception) []awsxray.Exception {
	// Need to read in reverse order so can't use a reader. Python formatted tracebacks always use '\n'
	// for newlines so we can just split on it without worrying about Windows newlines.

	lines := strings.Split(stacktrace, "\n")

	// Skip last line containing top level exception / message
	lineIdx := len(lines) - 2
	if lineIdx < 0 {
		return exceptions
	}
	line := lines[lineIdx]
	exception := &exceptions[0]
	isRemote := exception.Remote

	exception.Stack = nil
	for {
		if strings.HasPrefix(line, "  File ") {
			parts := strings.Split(line, ",")
			if len(parts) == 3 {
				filePart := parts[0]
				file := filePart[8 : len(filePart)-1]
				lineNumber := 0
				if strings.HasPrefix(parts[1], " line ") {
					lineNumber, _ = strconv.Atoi(parts[1][6:])
				}

				label := ""
				if strings.HasPrefix(parts[2], " in ") {
					label = parts[2][4:]
				}

				stack := awsxray.StackFrame{
					Path:  aws.String(file),
					Label: aws.String(label),
					Line:  aws.Int(lineNumber),
				}

				exception.Stack = append(exception.Stack, stack)
			}
		} else if strings.HasPrefix(line, "During handling of the above exception, another exception occurred:") {
			nextFileLineIdx := lineIdx - 1
			for {
				if nextFileLineIdx < 0 {
					// Couldn't find a "  File ..." line before end of input, malformed stack trace.
					return exceptions
				}
				if strings.HasPrefix(lines[nextFileLineIdx], "  File ") {
					break
				}
				nextFileLineIdx--
			}

			// Join message which potentially has newlines. Message starts two lines from the next "File " line and ends
			// two lines before the "During handling " line.
			message := strings.Join(lines[nextFileLineIdx+2:lineIdx-1], "\n")

			lineIdx = nextFileLineIdx

			colonIdx := strings.IndexByte(message, ':')
			if colonIdx < 0 {
				// Error not followed by a colon, malformed stack trace.
				return exceptions
			}

			causeType := message[0:colonIdx]
			causeMessage := message[colonIdx+2:]
			segmentID := newSegmentID()
			exceptions = append(exceptions, awsxray.Exception{
				ID:      aws.String(hex.EncodeToString(segmentID[:])),
				Type:    aws.String(causeType),
				Remote:  isRemote,
				Message: aws.String(causeMessage),
			})
			// when append causes `exceptions` to outgrow its existing
			// capacity, re-allocation will happen so the place
			// `exception` points to is no longer `exceptions[len(exceptions)-2]`,
			// consequently, we cannot write `exception.Cause = newException.ID`
			// below.
			newException := &exceptions[len(exceptions)-1]
			exceptions[len(exceptions)-2].Cause = newException.ID

			exception.Cause = newException.ID
			exception = newException
			// lineIdx is set to the next File line so ready to process it.
			line = lines[lineIdx]
			continue
		}
		lineIdx--
		if lineIdx < 0 {
			break
		}
		line = lines[lineIdx]
	}

	return exceptions
}

func fillJavaScriptStacktrace(stacktrace string, exceptions []awsxray.Exception) []awsxray.Exception {
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(stacktrace)))

	// Skip first line containing top level message
	exception := &exceptions[0]
	_, err := r.ReadLine()
	if err != nil {
		return exceptions
	}
	var line string
	line, err = r.ReadLine()
	if err != nil {
		return exceptions
	}

	exception.Stack = nil
	for {
		if strings.HasPrefix(line, "    at ") {
			parenIdx := strings.IndexByte(line, '(')
			label := ""
			path := ""
			lineIdx := 0
			if parenIdx >= 0 && line[len(line)-1] == ')' {
				label = line[7:parenIdx]
				path = line[parenIdx+1 : len(line)-1]
			} else if parenIdx < 0 {
				label = ""
				path = line[7:]
			}

			colonFirstIdx := strings.IndexByte(path, ':')
			colonSecondIdx := indexOf(path, ':', colonFirstIdx)

			if colonFirstIdx >= 0 && colonSecondIdx >= 0 && colonFirstIdx != colonSecondIdx {
				lineStr := path[colonFirstIdx+1 : colonSecondIdx]
				path = path[0:colonFirstIdx]
				lineIdx, _ = strconv.Atoi(lineStr)
			} else if colonFirstIdx < 0 && strings.Contains(path, "native") {
				path = "native"
			}

			// only append the exception if at least one of the values is not default
			if path != "" || label != "" || lineIdx != 0 {
				stack := awsxray.StackFrame{
					Path:  aws.String(path),
					Label: aws.String(label),
					Line:  aws.Int(lineIdx),
				}
				exception.Stack = append(exception.Stack, stack)
			}
		}
		line, err = r.ReadLine()
		if err != nil {
			break
		}
	}
	return exceptions
}

func fillDotnetStacktrace(stacktrace string, exceptions []awsxray.Exception) []awsxray.Exception {
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(stacktrace)))

	// Skip first line containing top level message
	exception := &exceptions[0]
	_, err := r.ReadLine()
	if err != nil {
		return exceptions
	}
	var line string
	line, err = r.ReadLine()
	if err != nil {
		return exceptions
	}

	exception.Stack = nil
	for {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "at ") {
			index := strings.Index(line, " in ")
			if index >= 0 {
				parts := strings.Split(line, " in ")

				label := parts[0][len("at "):]
				path := parts[1]
				lineNumber := 0

				colonIdx := strings.LastIndexByte(parts[1], ':')
				if colonIdx >= 0 {
					lineStr := path[colonIdx+1:]

					if strings.HasPrefix(lineStr, "line") {
						lineStr = lineStr[5:]
					}
					path = path[0:colonIdx]
					lineNumber, _ = strconv.Atoi(lineStr)
				}

				stack := awsxray.StackFrame{
					Path:  aws.String(path),
					Label: aws.String(label),
					Line:  aws.Int(lineNumber),
				}

				exception.Stack = append(exception.Stack, stack)
			} else {
				idx := strings.LastIndexByte(line, ')')
				if idx >= 0 {
					label := line[len("at ") : idx+1]
					path := ""
					lineNumber := 0

					stack := awsxray.StackFrame{
						Path:  aws.String(path),
						Label: aws.String(label),
						Line:  aws.Int(lineNumber),
					}

					exception.Stack = append(exception.Stack, stack)
				}
			}
		}

		line, err = r.ReadLine()
		if err != nil {
			break
		}
	}
	return exceptions
}

func fillGoStacktrace(stacktrace string, exceptions []awsxray.Exception) []awsxray.Exception {
	var line string
	var label string
	var path string
	var lineNumber int

	plnre := regexp.MustCompile(`([^:\s]+)\:(\d+)`)
	re := regexp.MustCompile(`^goroutine.*\brunning\b.*:$`)

	r := textproto.NewReader(bufio.NewReader(strings.NewReader(stacktrace)))

	// Skip first line containing top level message
	exception := &exceptions[0]
	_, err := r.ReadLine()
	if err != nil {
		return exceptions
	}
	line, err = r.ReadLine()
	if err != nil {
		return exceptions
	}

	exception.Stack = nil
	for {
		match := re.Match([]byte(line))
		if match {
			line, _ = r.ReadLine()
		}

		label = line
		line, _ = r.ReadLine()

		matches := plnre.FindStringSubmatch(line)
		if len(matches) == 3 {
			path = matches[1]
			lineNumber, _ = strconv.Atoi(matches[2])
		}

		stack := awsxray.StackFrame{
			Path:  aws.String(path),
			Label: aws.String(label),
			Line:  aws.Int(lineNumber),
		}

		exception.Stack = append(exception.Stack, stack)

		line, err = r.ReadLine()
		if err != nil {
			break
		}
	}

	return exceptions
}

// indexOf returns position of the first occurrence of a Byte in str starting at pos index.
func indexOf(str string, c byte, pos int) int {
	if pos < 0 {
		return -1
	}
	index := strings.IndexByte(str[pos+1:], c)
	if index > -1 {
		return index + pos + 1
	}
	return -1
}

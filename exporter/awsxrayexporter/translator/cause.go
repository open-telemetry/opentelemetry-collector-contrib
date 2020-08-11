// Copyright 2019, OpenTelemetry Authors
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
	"bufio"
	"encoding/hex"
	"net/textproto"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

// CauseData provides the shape for unmarshalling data that records exception.
type CauseData struct {
	WorkingDirectory string       `json:"working_directory,omitempty"`
	Paths            []string     `json:"paths,omitempty"`
	Exceptions       []*Exception `json:"exceptions,omitempty"`
}

// Exception provides the shape for unmarshalling an exception.
type Exception struct {
	ID      string  `json:"id,omitempty"`
	Type    string  `json:"type,omitempty"`
	Message string  `json:"message,omitempty"`
	Cause   string  `json:"cause,omitempty"`
	Stack   []Stack `json:"stack,omitempty"`
	Remote  bool    `json:"remote,omitempty"`
}

// Stack provides the shape for unmarshalling an stack.
type Stack struct {
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Label string `json:"label,omitempty"`
}

func makeCause(span pdata.Span, attributes map[string]string, resource pdata.Resource) (isError, isFault bool,
	filtered map[string]string, cause *CauseData) {
	status := span.Status()
	if status.IsNil() || status.Code() == 0 {
		return false, false, attributes, nil
	}
	filtered = attributes

	var (
		message   string
		errorKind string
	)

	hasExceptions := false
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		if event.Name() == semconventions.AttributeExceptionEventName {
			hasExceptions = true
			break
		}
	}

	if hasExceptions {
		language := ""
		if val, ok := resource.Attributes().Get(semconventions.AttributeTelemetrySDKLanguage); ok {
			language = val.StringVal()
		}

		exceptions := make([]*Exception, 0)
		for i := 0; i < span.Events().Len(); i++ {
			event := span.Events().At(i)
			if event.Name() == semconventions.AttributeExceptionEventName {
				exceptionType := ""
				message = ""
				stacktrace := ""

				if val, ok := event.Attributes().Get(semconventions.AttributeExceptionType); ok {
					exceptionType = val.StringVal()
				}

				if val, ok := event.Attributes().Get(semconventions.AttributeExceptionMessage); ok {
					message = val.StringVal()
				}

				if val, ok := event.Attributes().Get(semconventions.AttributeExceptionStacktrace); ok {
					stacktrace = val.StringVal()
				}

				parsed := parseException(exceptionType, message, stacktrace, language)
				exceptions = append(exceptions, parsed...)
			}
		}
		cause = &CauseData{Exceptions: exceptions}
	} else {
		// Use OpenCensus behavior if we didn't find any exception events to ease migration.
		message = status.Message()
		filtered = make(map[string]string)
		for key, value := range attributes {
			switch key {
			case semconventions.AttributeHTTPStatusText:
				if message == "" {
					message = value
				}
			default:
				filtered[key] = value
			}
		}

		if message != "" {
			id := newSegmentID()
			hexID := hex.EncodeToString(id)

			cause = &CauseData{
				Exceptions: []*Exception{
					{
						ID:      hexID,
						Type:    errorKind,
						Message: message,
					},
				},
			}
		}
	}

	if isClientError(status.Code()) {
		isError = true
		isFault = false
	} else {
		isError = false
		isFault = true
	}
	return isError, isFault, filtered, cause
}

func isClientError(code pdata.StatusCode) bool {
	httpStatus := tracetranslator.HTTPStatusCodeFromOCStatus(int32(code))
	return httpStatus >= 400 && httpStatus < 500
}

func parseException(exceptionType string, message string, stacktrace string, language string) []*Exception {
	r := textproto.NewReader(bufio.NewReader(strings.NewReader(stacktrace)))

	// Skip first line containing top level exception / message
	r.ReadLine()
	exception := &Exception{
		ID:      hex.EncodeToString(newSegmentID()),
		Type:    exceptionType,
		Message: message,
	}

	if language != "java" {
		// Only support Java stack traces right now.
		return []*Exception{exception}
	}

	if stacktrace == "" {
		return []*Exception{exception}
	}

	var line string
	line, err := r.ReadLine()
	if err != nil {
		return []*Exception{exception}
	}

	exceptions := make([]*Exception, 1)
	exceptions[0] = exception

	exception.Stack = make([]Stack, 0)
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

				stack := Stack{
					Path:  path,
					Label: label,
					Line:  line,
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
				} else {
					// String append overhead in this case, but multiline messages should be far less common than single
					// line ones.
					causeMessage += line
				}
			}
			newException := &Exception{
				ID:      hex.EncodeToString(newSegmentID()),
				Type:    causeType,
				Message: causeMessage,
				Stack:   make([]Stack, 0),
			}
			exception.Cause = newException.ID
			exception = newException
			exceptions = append(exceptions, newException)
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

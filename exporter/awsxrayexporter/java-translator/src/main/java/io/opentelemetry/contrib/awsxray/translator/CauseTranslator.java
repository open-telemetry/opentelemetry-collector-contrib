// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.contrib.awsxray.translator.model.CauseData;
import io.opentelemetry.sdk.trace.data.EventData;
import io.opentelemetry.sdk.trace.data.SpanData;
import java.io.BufferedReader;
import java.io.StringReader;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static io.opentelemetry.contrib.awsxray.translator.AttributeConstants.*;

final class CauseTranslator {

    static class CauseResult {
        final boolean isError;
        final boolean isFault;
        final boolean isThrottle;
        final Map<String, Object> filteredAttributes;
        final CauseData causeData;

        CauseResult(boolean isError, boolean isFault, boolean isThrottle,
                    Map<String, Object> filteredAttributes, CauseData causeData) {
            this.isError = isError;
            this.isFault = isFault;
            this.isThrottle = isThrottle;
            this.filteredAttributes = filteredAttributes;
            this.causeData = causeData;
        }
    }

    static CauseResult makeCause(SpanData span, Map<String, Object> attributes, Attributes resourceAttributes) {
        StatusCode statusCode = span.getStatus().getStatusCode();
        Map<String, Object> filtered = attributes;
        CauseData cause = null;
        String message = "";

        boolean isAwsSdkSpan = isAwsSdkSpan(span);
        boolean hasExceptionEvents = false;
        boolean hasAwsIndividualHttpError = false;

        for (EventData event : span.getEvents()) {
            if (EXCEPTION_EVENT_NAME.equals(event.getName())) {
                hasExceptionEvents = true;
                break;
            }
            if (isAwsSdkSpan && AWS_INDIVIDUAL_HTTP_EVENT_NAME.equals(event.getName())) {
                hasAwsIndividualHttpError = true;
                break;
            }
        }

        boolean hasExceptions = hasExceptionEvents || hasAwsIndividualHttpError;

        if (hasExceptions) {
            String language = "";
            Object langVal = resourceAttributes.get(AttributeKey.stringKey(TELEMETRY_SDK_LANGUAGE));
            if (langVal != null) {
                language = langVal.toString();
            }

            boolean isRemote = span.getKind() == SpanKind.CLIENT || span.getKind() == SpanKind.PRODUCER;

            List<CauseData.ExceptionData> exceptions = new ArrayList<>();
            for (EventData event : span.getEvents()) {
                if (EXCEPTION_EVENT_NAME.equals(event.getName())) {
                    String exceptionType = getStringAttribute(event.getAttributes(), EXCEPTION_TYPE);
                    message = getStringAttribute(event.getAttributes(), EXCEPTION_MESSAGE);
                    String stacktrace = getStringAttribute(event.getAttributes(), EXCEPTION_STACKTRACE);

                    List<CauseData.ExceptionData> parsed = parseException(exceptionType, message, stacktrace, isRemote, language);
                    exceptions.addAll(parsed);
                } else if (isAwsSdkSpan && AWS_INDIVIDUAL_HTTP_EVENT_NAME.equals(event.getName())) {
                    Object errorCode = event.getAttributes().get(AttributeKey.longKey(HTTP_RESPONSE_STATUS_CODE));
                    Object errorMessage = event.getAttributes().get(AttributeKey.stringKey(AWS_INDIVIDUAL_HTTP_ERROR_MSG_ATTR));
                    if (errorCode != null && errorMessage != null) {
                        long eventEpochTime = event.getEpochNanos() / 1000;
                        String msg = errorCode + "@" +
                                String.format("%.6f", eventEpochTime / 1_000_000.0) + "@" +
                                errorMessage;

                        CauseData.ExceptionData exception = new CauseData.ExceptionData();
                        exception.setId(generateSegmentId());
                        exception.setType(AWS_INDIVIDUAL_HTTP_ERROR_EVENT_TYPE);
                        exception.setRemote(true);
                        exception.setMessage(msg);
                        exceptions.add(exception);
                    }
                }
            }

            cause = new CauseData();
            cause.setExceptions(exceptions);

        } else if (statusCode != StatusCode.ERROR) {
            cause = null;
        } else {
            message = span.getStatus().getDescription() != null ? span.getStatus().getDescription() : "";
            filtered = new HashMap<>();
            for (Map.Entry<String, Object> entry : attributes.entrySet()) {
                if ("http.status_text".equals(entry.getKey())) {
                    if (message.isEmpty()) {
                        message = String.valueOf(entry.getValue());
                    }
                } else {
                    filtered.put(entry.getKey(), entry.getValue());
                }
            }

            if (!message.isEmpty()) {
                CauseData.ExceptionData exception = new CauseData.ExceptionData();
                exception.setId(generateSegmentId());
                exception.setType("");
                exception.setMessage(message);
                cause = new CauseData();
                List<CauseData.ExceptionData> exceptions = new ArrayList<>();
                exceptions.add(exception);
                cause.setExceptions(exceptions);
            }
        }

        // Determine error/fault/throttle from HTTP status code
        Long httpStatusCode = getHttpStatusCode(span);
        boolean isThrottle = false;
        boolean isError = false;
        boolean isFault = false;

        if (httpStatusCode == null || httpStatusCode < 400 || httpStatusCode > 599) {
            if (statusCode == StatusCode.ERROR) {
                isFault = true;
            }
        } else if (httpStatusCode >= 400 && httpStatusCode <= 499) {
            isError = true;
            if (httpStatusCode == 429) {
                isThrottle = true;
            }
        } else if (httpStatusCode >= 500 && httpStatusCode <= 599) {
            isFault = true;
        }

        return new CauseResult(isError, isFault, isThrottle, filtered, cause);
    }

    static List<CauseData.ExceptionData> parseException(String exceptionType, String message,
                                                         String stacktrace, boolean isRemote, String language) {
        List<CauseData.ExceptionData> exceptions = new ArrayList<>();
        CauseData.ExceptionData exception = new CauseData.ExceptionData();
        exception.setId(generateSegmentId());
        exception.setType(exceptionType);
        exception.setRemote(isRemote);
        exception.setMessage(message);
        exceptions.add(exception);

        if (stacktrace == null || stacktrace.isEmpty()) {
            return exceptions;
        }

        switch (language) {
            case "java":
            case "php":
                fillJavaStacktrace(stacktrace, exceptions);
                break;
            case "python":
                fillPythonStacktrace(stacktrace, exceptions);
                break;
            case "javascript":
                fillJavaScriptStacktrace(stacktrace, exceptions);
                break;
            case "dotnet":
                fillDotnetStacktrace(stacktrace, exceptions);
                break;
            case "go":
                fillGoStacktrace(stacktrace, exceptions);
                break;
            default:
                break;
        }

        return exceptions;
    }

    static void fillJavaStacktrace(String stacktrace, List<CauseData.ExceptionData> exceptions) {
        try (BufferedReader reader = new BufferedReader(new StringReader(stacktrace))) {
            CauseData.ExceptionData exception = exceptions.get(exceptions.size() - 1);
            boolean isRemote = exception.getRemote() != null && exception.getRemote();

            // Skip first line (top level message)
            String line = reader.readLine();
            if (line == null) return;
            line = reader.readLine();
            if (line == null) return;

            List<CauseData.StackFrame> stack = new ArrayList<>();
            while (line != null) {
                if (line.startsWith("\tat ")) {
                    int parenIdx = line.indexOf('(');
                    if (parenIdx >= 0 && line.charAt(line.length() - 1) == ')') {
                        String label = line.substring(4, parenIdx);
                        int slashIdx = label.indexOf('/');
                        if (slashIdx >= 0) {
                            label = label.substring(slashIdx + 1);
                        }

                        String path = line.substring(parenIdx + 1, line.length() - 1);
                        int lineNumber = 0;

                        int colonIdx = path.indexOf(':');
                        if (colonIdx >= 0) {
                            try {
                                lineNumber = Integer.parseInt(path.substring(colonIdx + 1));
                            } catch (NumberFormatException ignored) {}
                            path = path.substring(0, colonIdx);
                        }

                        CauseData.StackFrame frame = new CauseData.StackFrame();
                        frame.setPath(path);
                        frame.setLabel(label);
                        frame.setLine(lineNumber);
                        stack.add(frame);
                    }
                } else if (line.startsWith("Caused by: ")) {
                    exception.setStack(stack.isEmpty() ? null : stack);

                    String causeType = line.substring("Caused by: ".length());
                    String causeMessage = "";
                    int colonIdx = causeType.indexOf(':');
                    if (colonIdx >= 0) {
                        causeMessage = causeType.substring(colonIdx + 2);
                        causeType = causeType.substring(0, colonIdx);
                    }

                    // Peek for multiline messages
                    line = reader.readLine();
                    while (line != null && !line.startsWith("\tat ")) {
                        if (line.startsWith("Caused by: ") || line.startsWith("\t...")) {
                            break;
                        }
                        causeMessage += line;
                        line = reader.readLine();
                    }

                    CauseData.ExceptionData newException = new CauseData.ExceptionData();
                    newException.setId(generateSegmentId());
                    newException.setType(causeType);
                    newException.setRemote(isRemote);
                    newException.setMessage(causeMessage);
                    exceptions.add(newException);

                    exception.setCause(newException.getId());
                    exception = newException;
                    stack = new ArrayList<>();
                    continue;
                }
                line = reader.readLine();
            }
            exception.setStack(stack.isEmpty() ? null : stack);
        } catch (Exception ignored) {}
    }

    static void fillPythonStacktrace(String stacktrace, List<CauseData.ExceptionData> exceptions) {
        String[] lines = stacktrace.split("\n");
        CauseData.ExceptionData exception = exceptions.get(exceptions.size() - 1);
        boolean isRemote = exception.getRemote() != null && exception.getRemote();

        // Skip last line (top level exception/message)
        int lineIdx = lines.length - 2;
        if (lineIdx < 0) return;

        List<CauseData.StackFrame> stack = new ArrayList<>();
        while (lineIdx >= 0) {
            String line = lines[lineIdx];
            if (line.startsWith("  File ")) {
                String[] parts = line.split(",");
                if (parts.length == 3) {
                    String filePart = parts[0];
                    String file = filePart.substring(8, filePart.length() - 1);
                    int lineNumber = 0;
                    if (parts[1].startsWith(" line ")) {
                        try {
                            lineNumber = Integer.parseInt(parts[1].substring(6).trim());
                        } catch (NumberFormatException ignored) {}
                    }
                    String label = "";
                    if (parts[2].startsWith(" in ")) {
                        label = parts[2].substring(4);
                    }

                    CauseData.StackFrame frame = new CauseData.StackFrame();
                    frame.setPath(file);
                    frame.setLabel(label);
                    frame.setLine(lineNumber);
                    stack.add(frame);
                }
            } else if (line.startsWith("During handling of the above exception, another exception occurred:")) {
                exception.setStack(stack.isEmpty() ? null : stack);

                int nextFileLineIdx = lineIdx - 1;
                while (nextFileLineIdx >= 0 && !lines[nextFileLineIdx].startsWith("  File ")) {
                    nextFileLineIdx--;
                }
                if (nextFileLineIdx < 0) return;

                StringBuilder messageBuilder = new StringBuilder();
                for (int i = nextFileLineIdx + 2; i < lineIdx - 1; i++) {
                    if (messageBuilder.length() > 0) messageBuilder.append("\n");
                    messageBuilder.append(lines[i]);
                }
                String msg = messageBuilder.toString();
                lineIdx = nextFileLineIdx;

                int colonIdx = msg.indexOf(':');
                if (colonIdx < 0) return;

                String causeType = msg.substring(0, colonIdx);
                String causeMessage = msg.substring(colonIdx + 2);

                CauseData.ExceptionData newException = new CauseData.ExceptionData();
                newException.setId(generateSegmentId());
                newException.setType(causeType);
                newException.setRemote(isRemote);
                newException.setMessage(causeMessage);
                exceptions.add(newException);

                exception.setCause(newException.getId());
                exception = newException;
                stack = new ArrayList<>();
                continue;
            }
            lineIdx--;
        }
        exception.setStack(stack.isEmpty() ? null : stack);
    }

    static void fillJavaScriptStacktrace(String stacktrace, List<CauseData.ExceptionData> exceptions) {
        try (BufferedReader reader = new BufferedReader(new StringReader(stacktrace))) {
            CauseData.ExceptionData exception = exceptions.get(exceptions.size() - 1);

            // Skip first line (message)
            String line = reader.readLine();
            if (line == null) return;
            line = reader.readLine();
            if (line == null) return;

            List<CauseData.StackFrame> stack = new ArrayList<>();
            while (line != null) {
                if (line.startsWith("    at ")) {
                    int parenIdx = line.indexOf('(');
                    String label = "";
                    String path = "";
                    int lineNumber = 0;

                    if (parenIdx >= 0 && line.charAt(line.length() - 1) == ')') {
                        label = line.substring(7, parenIdx);
                        path = line.substring(parenIdx + 1, line.length() - 1);
                    } else if (parenIdx < 0) {
                        path = line.substring(7);
                    }

                    int colonFirstIdx = path.indexOf(':');
                    int colonSecondIdx = colonFirstIdx >= 0 ? path.indexOf(':', colonFirstIdx + 1) : -1;

                    if (colonFirstIdx >= 0 && colonSecondIdx >= 0 && colonFirstIdx != colonSecondIdx) {
                        try {
                            lineNumber = Integer.parseInt(path.substring(colonFirstIdx + 1, colonSecondIdx));
                        } catch (NumberFormatException ignored) {}
                        path = path.substring(0, colonFirstIdx);
                    } else if (colonFirstIdx < 0 && path.contains("native")) {
                        path = "native";
                    }

                    if (!path.isEmpty() || !label.isEmpty() || lineNumber != 0) {
                        CauseData.StackFrame frame = new CauseData.StackFrame();
                        frame.setPath(path);
                        frame.setLabel(label);
                        frame.setLine(lineNumber);
                        stack.add(frame);
                    }
                }
                line = reader.readLine();
            }
            exception.setStack(stack.isEmpty() ? null : stack);
        } catch (Exception ignored) {}
    }

    static void fillDotnetStacktrace(String stacktrace, List<CauseData.ExceptionData> exceptions) {
        try (BufferedReader reader = new BufferedReader(new StringReader(stacktrace))) {
            CauseData.ExceptionData exception = exceptions.get(exceptions.size() - 1);

            // Skip first line (message)
            String line = reader.readLine();
            if (line == null) return;
            line = reader.readLine();
            if (line == null) return;

            List<CauseData.StackFrame> stack = new ArrayList<>();
            while (line != null) {
                line = line.trim();
                if (line.startsWith("at ")) {
                    int inIdx = line.indexOf(" in ");
                    if (inIdx >= 0) {
                        String label = line.substring(3, inIdx);
                        String path = line.substring(inIdx + 4);
                        int lineNumber = 0;

                        int colonIdx = path.lastIndexOf(':');
                        if (colonIdx >= 0) {
                            String lineStr = path.substring(colonIdx + 1);
                            if (lineStr.startsWith("line")) {
                                lineStr = lineStr.substring(5);
                            }
                            path = path.substring(0, colonIdx);
                            try {
                                lineNumber = Integer.parseInt(lineStr.trim());
                            } catch (NumberFormatException ignored) {}
                        }

                        CauseData.StackFrame frame = new CauseData.StackFrame();
                        frame.setPath(path);
                        frame.setLabel(label);
                        frame.setLine(lineNumber);
                        stack.add(frame);
                    } else {
                        int parenIdx = line.lastIndexOf(')');
                        if (parenIdx >= 0) {
                            String label = line.substring(3, parenIdx + 1);
                            CauseData.StackFrame frame = new CauseData.StackFrame();
                            frame.setPath("");
                            frame.setLabel(label);
                            frame.setLine(0);
                            stack.add(frame);
                        }
                    }
                }
                line = reader.readLine();
            }
            exception.setStack(stack.isEmpty() ? null : stack);
        } catch (Exception ignored) {}
    }

    private static final Pattern GO_PATH_LINE_PATTERN = Pattern.compile("([^:\\s]+):(\\d+)");
    private static final Pattern GO_GOROUTINE_PATTERN = Pattern.compile("^goroutine.*\\brunning\\b.*:$");

    static void fillGoStacktrace(String stacktrace, List<CauseData.ExceptionData> exceptions) {
        try (BufferedReader reader = new BufferedReader(new StringReader(stacktrace))) {
            CauseData.ExceptionData exception = exceptions.get(exceptions.size() - 1);

            // Skip first line
            String line = reader.readLine();
            if (line == null) return;
            line = reader.readLine();
            if (line == null) return;

            List<CauseData.StackFrame> stack = new ArrayList<>();
            while (line != null) {
                if (GO_GOROUTINE_PATTERN.matcher(line).matches()) {
                    line = reader.readLine();
                    if (line == null) break;
                }

                String label = line;
                line = reader.readLine();
                if (line == null) break;

                String path = "";
                int lineNumber = 0;
                Matcher matcher = GO_PATH_LINE_PATTERN.matcher(line);
                if (matcher.find()) {
                    path = matcher.group(1);
                    try {
                        lineNumber = Integer.parseInt(matcher.group(2));
                    } catch (NumberFormatException ignored) {}
                }

                CauseData.StackFrame frame = new CauseData.StackFrame();
                frame.setPath(path);
                frame.setLabel(label);
                frame.setLine(lineNumber);
                stack.add(frame);

                line = reader.readLine();
            }
            exception.setStack(stack.isEmpty() ? null : stack);
        } catch (Exception ignored) {}
    }

    private static boolean isAwsSdkSpan(SpanData span) {
        Object rpcSystem = span.getAttributes().get(AttributeKey.stringKey(RPC_SYSTEM));
        return AWS_API_RPC_SYSTEM.equals(rpcSystem);
    }

    private static Long getHttpStatusCode(SpanData span) {
        Object val = span.getAttributes().get(AttributeKey.longKey(HTTP_STATUS_CODE));
        if (val == null) {
            val = span.getAttributes().get(AttributeKey.longKey(HTTP_RESPONSE_STATUS_CODE));
        }
        return val instanceof Long ? (Long) val : null;
    }

    private static String getStringAttribute(Attributes attributes, String key) {
        Object val = attributes.get(AttributeKey.stringKey(key));
        return val != null ? val.toString() : "";
    }

    static String generateSegmentId() {
        return UUID.randomUUID().toString().replace("-", "").substring(0, 16);
    }

    private CauseTranslator() {}
}

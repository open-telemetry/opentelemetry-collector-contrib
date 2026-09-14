// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class CauseData {

    @JsonProperty("working_directory")
    private String workingDirectory;

    @JsonProperty("paths")
    private List<String> paths;

    @JsonProperty("exceptions")
    private List<ExceptionData> exceptions;

    public String getWorkingDirectory() { return workingDirectory; }
    public void setWorkingDirectory(String workingDirectory) { this.workingDirectory = workingDirectory; }

    public List<String> getPaths() { return paths; }
    public void setPaths(List<String> paths) { this.paths = paths; }

    public List<ExceptionData> getExceptions() { return exceptions; }
    public void setExceptions(List<ExceptionData> exceptions) { this.exceptions = exceptions; }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class ExceptionData {
        @JsonProperty("id")
        private String id;

        @JsonProperty("message")
        private String message;

        @JsonProperty("type")
        private String type;

        @JsonProperty("remote")
        private Boolean remote;

        @JsonProperty("truncated")
        private Long truncated;

        @JsonProperty("skipped")
        private Long skipped;

        @JsonProperty("cause")
        private String cause;

        @JsonProperty("stack")
        private List<StackFrame> stack;

        public String getId() { return id; }
        public void setId(String id) { this.id = id; }

        public String getMessage() { return message; }
        public void setMessage(String message) { this.message = message; }

        public String getType() { return type; }
        public void setType(String type) { this.type = type; }

        public Boolean getRemote() { return remote; }
        public void setRemote(Boolean remote) { this.remote = remote; }

        public Long getTruncated() { return truncated; }
        public void setTruncated(Long truncated) { this.truncated = truncated; }

        public Long getSkipped() { return skipped; }
        public void setSkipped(Long skipped) { this.skipped = skipped; }

        public String getCause() { return cause; }
        public void setCause(String cause) { this.cause = cause; }

        public List<StackFrame> getStack() { return stack; }
        public void setStack(List<StackFrame> stack) { this.stack = stack; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class StackFrame {
        @JsonProperty("path")
        private String path;

        @JsonProperty("line")
        private Integer line;

        @JsonProperty("label")
        private String label;

        public String getPath() { return path; }
        public void setPath(String path) { this.path = path; }

        public Integer getLine() { return line; }
        public void setLine(Integer line) { this.line = line; }

        public String getLabel() { return label; }
        public void setLabel(String label) { this.label = label; }
    }
}

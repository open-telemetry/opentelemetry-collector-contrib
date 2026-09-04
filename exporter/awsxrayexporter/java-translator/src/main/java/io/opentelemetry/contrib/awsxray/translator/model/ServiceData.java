// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class ServiceData {

    @JsonProperty("version")
    private String version;

    @JsonProperty("compiler_version")
    private String compilerVersion;

    @JsonProperty("compiler")
    private String compiler;

    public String getVersion() { return version; }
    public void setVersion(String version) { this.version = version; }

    public String getCompilerVersion() { return compilerVersion; }
    public void setCompilerVersion(String compilerVersion) { this.compilerVersion = compilerVersion; }

    public String getCompiler() { return compiler; }
    public void setCompiler(String compiler) { this.compiler = compiler; }
}

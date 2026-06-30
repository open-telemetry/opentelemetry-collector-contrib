// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class SqlData {

    @JsonProperty("connection_string")
    private String connectionString;

    @JsonProperty("url")
    private String url;

    @JsonProperty("sanitized_query")
    private String sanitizedQuery;

    @JsonProperty("database_type")
    private String databaseType;

    @JsonProperty("database_version")
    private String databaseVersion;

    @JsonProperty("driver_version")
    private String driverVersion;

    @JsonProperty("user")
    private String user;

    @JsonProperty("preparation")
    private String preparation;

    public String getConnectionString() { return connectionString; }
    public void setConnectionString(String connectionString) { this.connectionString = connectionString; }

    public String getUrl() { return url; }
    public void setUrl(String url) { this.url = url; }

    public String getSanitizedQuery() { return sanitizedQuery; }
    public void setSanitizedQuery(String sanitizedQuery) { this.sanitizedQuery = sanitizedQuery; }

    public String getDatabaseType() { return databaseType; }
    public void setDatabaseType(String databaseType) { this.databaseType = databaseType; }

    public String getDatabaseVersion() { return databaseVersion; }
    public void setDatabaseVersion(String databaseVersion) { this.databaseVersion = databaseVersion; }

    public String getDriverVersion() { return driverVersion; }
    public void setDriverVersion(String driverVersion) { this.driverVersion = driverVersion; }

    public String getUser() { return user; }
    public void setUser(String user) { this.user = user; }

    public String getPreparation() { return preparation; }
    public void setPreparation(String preparation) { this.preparation = preparation; }
}

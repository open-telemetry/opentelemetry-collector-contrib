// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import io.opentelemetry.contrib.awsxray.translator.model.SqlData;
import io.opentelemetry.sdk.trace.data.SpanData;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

import static io.opentelemetry.contrib.awsxray.translator.AttributeConstants.*;

final class SqlTranslator {

    private static final Set<String> SQL_SYSTEMS = Set.of(
            "db2", "derby", "hive", "mariadb", "mssql", "mysql",
            "oracle", "postgresql", "sqlite", "teradata", "other_sql"
    );

    static class SqlResult {
        final Map<String, Object> filteredAttributes;
        final SqlData sqlData;

        SqlResult(Map<String, Object> filteredAttributes, SqlData sqlData) {
            this.filteredAttributes = filteredAttributes;
            this.sqlData = sqlData;
        }
    }

    static SqlResult makeSql(SpanData span, Map<String, Object> attributes) {
        Map<String, Object> filtered = new HashMap<>();
        String dbConnectionString = "";
        String dbSystem = "";
        String dbInstance = "";
        String dbStatement = "";
        String dbUser = "";

        for (Map.Entry<String, Object> entry : attributes.entrySet()) {
            switch (entry.getKey()) {
                case DB_CONNECTION_STRING:
                    dbConnectionString = String.valueOf(entry.getValue());
                    break;
                case DB_SYSTEM:
                    dbSystem = String.valueOf(entry.getValue());
                    break;
                case DB_NAME:
                    dbInstance = String.valueOf(entry.getValue());
                    break;
                case DB_STATEMENT:
                    dbStatement = String.valueOf(entry.getValue());
                    break;
                case DB_USER:
                    dbUser = String.valueOf(entry.getValue());
                    break;
                default:
                    filtered.put(entry.getKey(), entry.getValue());
                    break;
            }
        }

        if (!SQL_SYSTEMS.contains(dbSystem)) {
            return new SqlResult(attributes, null);
        }

        String dbUrl = span.getName();

        if (dbConnectionString.isEmpty()) {
            dbConnectionString = "localhost";
        }
        dbConnectionString = dbConnectionString + "/" + dbInstance;

        SqlData sqlData = new SqlData();
        sqlData.setUrl(dbUrl);
        sqlData.setConnectionString(dbConnectionString);
        sqlData.setDatabaseType(dbSystem);
        sqlData.setUser(dbUser.isEmpty() ? null : dbUser);
        sqlData.setSanitizedQuery(dbStatement.isEmpty() ? null : dbStatement);

        return new SqlResult(filtered, sqlData);
    }

    private SqlTranslator() {}
}

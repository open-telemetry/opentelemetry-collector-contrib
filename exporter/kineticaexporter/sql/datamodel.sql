
----- Logs

CREATE TABLE otel.log
(
        log_id                   VARCHAR (uuid),            -- generated
        trace_id                 VARCHAR(32),
        span_id                  VARCHAR(16),
        time_unix_nano           TIMESTAMP,
        observed_time_unix_nano  TIMESTAMP,
        severity_id              TINYINT,
        severity_text            VARCHAR(8),
        body                     VARCHAR,
        flags                    INT,
        PRIMARY KEY (log_id)
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.log_attribute
(
        log_id       VARCHAR (uuid),
        key          VARCHAR(256, dict),
        string_value VARCHAR(256),
        bool_value   BOOLEAN,
        int_value    INT,
        double_value DOUBLE,
        bytes_value  BYTES,
        PRIMARY KEY (log_id, key),
        SHARD KEY (log_id),
        FOREIGN KEY (log_id) REFERENCES otel.log(log_id) AS fk_log
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.log_resource_attribute
(
        log_id  VARCHAR (uuid),              -- generated
        key          VARCHAR(256, dict),
        string_value VARCHAR,
        bool_value   BOOLEAN,
        int_value    INT,
        double_value DOUBLE,
        bytes_value  BYTES,
        PRIMARY KEY (log_id, key),
        SHARD KEY (log_id),
        FOREIGN KEY (log_id) REFERENCES otel.log(log_id) AS fk_log_resource
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.log_scope_attribute
(
        log_id     VARCHAR (uuid),              -- generated
        scope_name   VARCHAR(64, dict),
        scope_ver    VARCHAR(16, dict),
        key          VARCHAR(256, dict),
        string_value VARCHAR,
        bool_value   BOOLEAN,
        int_value    INT,
        double_value DOUBLE,
        bytes_value  BYTES,
        PRIMARY KEY (log_id, key),
        SHARD KEY (log_id),
        FOREIGN KEY (log_id) REFERENCES otel.log(log_id) AS fk_log_scope
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

----- Traces

----- NEW

CREATE TABLE "otel"."trace_span"
(
    "id" UUID (primary_key) NOT NULL,
    "trace_id" VARCHAR (32) NOT NULL,
    "span_id" VARCHAR (16) NOT NULL,
    "parent_span_id" VARCHAR (16),
    "trace_state" VARCHAR (256),
    "name" VARCHAR (256, dict) NOT NULL,
    "span_kind" TINYINT (dict),
    "start_time_unix_nano" TIMESTAMP NOT NULL,
    "end_time_unix_nano" TIMESTAMP NOT NULL,
    "dropped_attributes_count" INTEGER,
    "dropped_events_count" INTEGER,
    "dropped_links_count" INTEGER,
    "message" VARCHAR(256),
    "status_code" TINYINT (dict)
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."trace_span_attribute"
(
    "span_id" UUID (primary_key, shard_key) NOT NULL,
    "key" VARCHAR (primary_key, 256, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (span_id) references otel.trace_span(id) as fk_span
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."trace_resource_attribute"
(
    span_id VARCHAR (UUID) NOT NULL,
    "key" VARCHAR (256, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    PRIMARY KEY (span_id, key),
    SHARD KEY (span_id),
    FOREIGN KEY (span_id) references otel.trace_span(id) as fk_span_resource
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."trace_scope_attribute"
(
    "span_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256, dict),
    "version" VARCHAR (256, dict),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (span_id),
    FOREIGN KEY (span_id) references otel.trace_span(id) as fk_span_scope

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."trace_event_attribute"
(
    "span_id" UUID (primary_key) NOT NULL,
    "event_name" VARCHAR (128, dict) NOT NULL,
    "time_unix_nano" TIMESTAMP,
    "key" VARCHAR (primary_key, 128) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (span_id),
    FOREIGN KEY (span_id) references otel.trace_span(id) as fk_span_event

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."trace_link_attribute"
(
    "link_span_id" UUID (primary_key) NOT NULL,
    "trace_id" VARCHAR (32),
    "span_id" VARCHAR (16),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" TINYINT,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (link_span_id),
    FOREIGN KEY (link_span_id) references otel.trace_span(id) as fk_span_link

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

------ METRICS

------ GAUGE

CREATE TABLE otel.metric_gauge
(
    gauge_id UUID (primary_key, shard_key) not null,
    metric_name varchar(256) not null,
    metric_description varchar (256),
    metric_unit varchar (256)

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_gauge_datapoint
(
    gauge_id UUID (primary_key, shard_key) not null,
    id UUID (primary_key) not null,
    start_time_unix TIMESTAMP NOT NULL,
    time_unix TIMESTAMP NOT NULL,
    gauge_value DOUBLE,
    flags INT,
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_gauge_datapoint_attribute"
(
    "gauge_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_gauge_datapoint_exemplar
(
    "gauge_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    time_unix TIMESTAMP NOT NULL,
    gauge_value DOUBLE,
    "trace_id" VARCHAR (32),
    "span_id" VARCHAR (16),
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_gauge_datapoint_exemplar_attribute
(
    "gauge_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_gauge_resource_attribute"
(
    "gauge_id" UUID (primary_key) NOT NULL,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (gauge_id),
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_resource_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_gauge_scope_attribute"
(
    "gauge_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256),
    "version" VARCHAR (256),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (gauge_id),
    FOREIGN KEY (gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_scope_attribute

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

------- SUM

CREATE TABLE otel.metric_sum
(
    sum_id UUID (primary_key, shard_key) not null,
    metric_name varchar (256) not null,
    metric_description varchar (256),
    metric_unit varchar (256),
    aggregation_temporality INTEGER,
    is_monotonic BOOLEAN

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_sum_datapoint
(
    sum_id UUID (primary_key, shard_key) not null,
    id UUID (primary_key) not null,
    start_time_unix TIMESTAMP NOT NULL,
    time_unix TIMESTAMP NOT NULL,
    sum_value DOUBLE,
    flags INT,
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_sum_datapoint_attribute"
(
    "sum_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_sum_datapoint_exemplar
(
    "sum_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    time_unix TIMESTAMP NOT NULL,
    sum_value DOUBLE,
    "trace_id" VARCHAR (32),
    "span_id" VARCHAR (16),
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_sum_datapoint_exemplar_attribute
(
    "sum_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_sum_resource_attribute"
(
    "sum_id" UUID (primary_key) NOT NULL,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (sum_id),
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_resource_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_sum_scope_attribute"
(
    "sum_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256),
    "version" VARCHAR (256),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (sum_id),
    FOREIGN KEY (sum_id) references otel.metric_sum(sum_id) as fk_sum_scope_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

------ HISTOGRAM

CREATE TABLE otel.metric_histogram
(
    histogram_id UUID (primary_key, shard_key) not null,
    metric_name varchar (256) not null,
    metric_description varchar (256),
    metric_unit varchar (256),
    aggregation_temporality int8

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_histogram_datapoint
(
    histogram_id UUID (primary_key, shard_key) not null,
    id UUID (primary_key) not null,
    start_time_unix TIMESTAMP,
    time_unix TIMESTAMP NOT NULL,
    count LONG,
    data_sum DOUBLE,
    data_min DOUBLE,
    data_max DOUBLE,
    flags INT,
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_histogram_datapoint_bucket_count
(
    histogram_id UUID (primary_key, shard_key) not null,
    datapoint_id UUID (primary_key) not null,
    count_id UUID (primary_key) not null,
    count LONG,
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_bucket_count

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_histogram_datapoint_explicit_bound
(
    histogram_id UUID (primary_key, shard_key) not null,
    datapoint_id UUID (primary_key) not null,
    bound_id UUID (primary_key) not null,
    explicit_bound DOUBLE,
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_explicit_bound

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_histogram_datapoint_attribute"
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_histogram_datapoint_exemplar
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    time_unix TIMESTAMP NOT NULL,
    histogram_value DOUBLE,
    "trace_id" VARCHAR (32),
    "span_id" VARCHAR (16),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_histogram_datapoint_exemplar_attribute
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_histogram_resource_attribute"
(
    "histogram_id" UUID (primary_key) NOT NULL,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (histogram_id),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_histogram_scope_attribute"
(
    "histogram_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256),
    "version" VARCHAR (256),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (histogram_id),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);


------- EXPONENTIAL HISTOGRAM

CREATE TABLE otel.metric_exp_histogram
(
    histogram_id UUID (primary_key, shard_key) not null,
    metric_name varchar (256) not null,
    metric_description varchar (256),
    metric_unit varchar (256),
    aggregation_temporality int8

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_exp_histogram_datapoint
(
    histogram_id UUID (primary_key, shard_key) not null,
    id UUID (primary_key) not null,
    start_time_unix TIMESTAMP,
    time_unix TIMESTAMP NOT NULL,
    count LONG,
    data_sum DOUBLE,
    scale INTEGER,
    zero_count LONG,
    buckets_positive_offset INTEGER,
    buckets_negative_offset INTEGER,
    data_min DOUBLE,
    data_max DOUBLE,
    flags INT,
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_exp_histogram_datapoint_bucket_positive_count
(
    histogram_id UUID (primary_key, shard_key) not null,
    datapoint_id UUID (primary_key) not null,
    count_id UUID (primary_key) not null,
    count LONG,
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_exp_histogram_datapoint_bucket_negative_count
(
    histogram_id UUID (primary_key, shard_key) not null,
    datapoint_id UUID (primary_key) not null,
    count_id UUID (primary_key) not null,
    count LONG,
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_exp_histogram_datapoint_attribute"
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_exp_histogram_datapoint_exemplar
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    time_unix TIMESTAMP NOT NULL,
    sum_value DOUBLE,
    "trace_id" VARCHAR (32),
    "span_id" VARCHAR (16),
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_exp_histogram_datapoint_exemplar_attribute
(
    "histogram_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    exemplar_id UUID (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_exp_histogram_resource_attribute"
(
    "histogram_id" UUID (primary_key) NOT NULL,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (histogram_id),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_exp_histogram_scope_attribute"
(
    "histogram_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256),
    "version" VARCHAR (256),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (histogram_id),
    FOREIGN KEY (histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

------- SUMMARY

CREATE TABLE otel.metric_summary
(
    summary_id UUID (primary_key, shard_key) not null,
    metric_name varchar (256) not null,
    metric_description varchar (256),
    metric_unit varchar (256)
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_summary_datapoint
(
    summary_id UUID (primary_key, shard_key) not null,
    id UUID (primary_key) not null,
    start_time_unix TIMESTAMP,
    time_unix TIMESTAMP NOT NULL,
    count LONG,
    data_sum DOUBLE,
    flags INT,
    FOREIGN KEY (summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint

) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_summary_datapoint_attribute"
(
    "summary_id" UUID (primary_key, shard_key) NOT NULL,
    datapoint_id uuid (primary_key) not null,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    FOREIGN KEY (summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE otel.metric_summary_datapoint_quantile_values
(
    summary_id UUID (primary_key, shard_key) not null,
    datapoint_id UUID (primary_key) not null,
    quantile_id UUID (primary_key) not null,
    quantile DOUBLE,
    value DOUBLE,
    FOREIGN KEY (summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_quantile
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_summary_resource_attribute"
(
    "summary_id" UUID (primary_key) NOT NULL,
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (summary_id),
    FOREIGN KEY (summary_id) references otel.metric_summary(summary_id) as fk_summary_resource_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

CREATE TABLE "otel"."metric_summary_scope_attribute"
(
    "summary_id" UUID (primary_key) NOT NULL,
    "name" VARCHAR (256),
    "version" VARCHAR (256),
    "key" VARCHAR (primary_key, 128, dict) NOT NULL,
    "string_value" VARCHAR (256),
    "bool_value" BOOLEAN,
    "int_value" INTEGER,
    "double_value" DOUBLE,
    "bytes_value" BLOB (store_only),
    SHARD KEY (summary_id),
    FOREIGN KEY (summary_id) references otel.metric_summary(summary_id) as fk_summary_scope_attribute
) USING TABLE PROPERTIES (NO_ERROR_IF_EXISTS = TRUE);

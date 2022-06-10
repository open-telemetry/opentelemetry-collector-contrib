# SQL Query Receiver (Alpha)

The SQL Query Receiver uses custom SQL queries to generate metrics from a database connection.

> :construction: This receiver is in **ALPHA**. Behavior, configuration fields, and metric data model are subject to change.

## Configuration

The configuration supports the following top-level fields:

- `driver`(required): The name of the database driver: one of _postgres_, _mysql_, _snowflake_, _sqlserver_, or _hdb_ (SAP HANA).
- `datasource`(required): The datasourcename value passed to [sql.Open](https://pkg.go.dev/database/sql#Open). This is 
a driver-specific string usually consisting of at least a database name and connection information.
e.g. _host=localhost port=5432 user=me password=s3cr3t sslmode=disable_
- `queries`(required): A list of queries, where a query is a sql statement and one or more metrics (details below).
- `collection_interval`(optional): The time interval between query executions. Defaults to _10s_.

### Queries

A _query_ consists of a sql statement and one or more _metrics_, where each metric consists of a
`metric_name`, a `value_column`, an optional list of `attribute_columns`, and an optional `is_monotonic` boolean.
Each _metric_ in the configuration will produce one OTel metric per row returned from its sql query.

* `metric_name`(required): the name assigned to the OTel metric.
* `value_column`(required): the column name in the returned dataset used to set the value of the metric's datapoint. The column's values must be of an integer type.
* `attribute_columns`(optional): a list of column names in the returned dataset used to set attibutes on the datapoint.
* `is_monotonic`(optional): a boolean value indicating whether the metric value is monotonically increasing. If it is, the receiver will emit a sum type for this metric.

### Example

```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "host=localhost port=5432 user=postgres password=s3cr3t sslmode=disable"
    queries:
      - sql: "select count(*) as count, 42 as val from movie"
        metrics:
          - metric_name: movie.count
            value_column: "count"
          - metric_name: movie.val
            value_column: "val"
      - sql: "select count(*) as count, genre from movie group by genre"
        metrics:
          - metric_name: movie.genres
            value_column: "count"
            attribute_columns: [ "genre" ]
```

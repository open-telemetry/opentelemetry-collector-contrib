# SQL Query

This is a receiver helper that can be applied to relational database receivers to extract metrics.

## Configuration

The configuration supports the following top-level fields:

- `driver`(required): The name of the database driver: one of _postgres_, _mysql_, _snowflake_, _sqlserver_, _hdb_ (SAP HANA), or _oracle_ (Oracle DB).
- `datasource`(required): The datasource value passed to [sql.Open](https://pkg.go.dev/database/sql#Open). This is
  a driver-specific string usually consisting of at least a database name and connection information. This is sometimes
  referred to as the "connection string" in driver documentation.
  e.g. _host=localhost port=5432 user=me password=s3cr3t sslmode=disable_
- `queries`(required): A list of queries, where a query is a sql statement and one or more metrics (details below).
- `collection_interval`(optional): The time interval between query executions. Defaults to _10s_.

### Queries

A _query_ consists of a sql statement and one or more _metrics_, where each metric consists of a
`metric_name`, a `value_column`, and additional optional fields.
Each _metric_ in the configuration will produce one OTel metric per row returned from its sql query.

* `metric_name`(required): the name assigned to the OTel metric.
* `value_column`(required): the column name in the returned dataset used to set the value of the metric's datapoint. This may be case-sensitive, depending on the driver (e.g. Oracle DB).
* `attribute_columns`(optional): a list of column names in the returned dataset used to set attibutes on the datapoint. These attributes may be case-sensitive, depending on the driver (e.g. Oracle DB).
* `data_type` (optional): can be `gauge` or `sum`; defaults to `gauge`.
* `value_type` (optional): can be `int` or `double`; defaults to `int`.
* `monotonic` (optional): boolean; whether a cumulative sum's value is monotonically increasing (i.e. never rolls over or resets); defaults to false.
* `aggregation` (optional): only applicable for `data_type=sum`; can be `cumulative` or `delta`; defaults to `cumulative`.
* `description` (optional): the description applied to the metric.
* `unit` (optional): the units applied to the metric.
* `static_attributes` (optional): static attributes applied to the metrics

### Example

```yaml
receivers:
  sql:
    driver: postgres
    datasource: "host=localhost port=5432 user=postgres password=s3cr3t sslmode=disable"
    queries:
      - sql: "select count(*) as count, genre from movie group by genre"
        metrics:
          - metric_name: movie.genres
            value_column: "count"
            attribute_columns: [ "genre" ]
            static_attributes: 
               dbinstance: mydbinstance
```
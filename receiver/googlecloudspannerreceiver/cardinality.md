# Cloud Spanner: Controlling Metrics Cardinality for Open Telemetry Receiver

## Overview
This document describes the algorithm used to control the cardinality of the Top N metrics exported by Cloud Spanner receiver in OpenTelemetry collector.
With cardinality limits enforced in Cloud Monitoring and possibly in Prometheus, it is critical to control the cardinality of the metrics to avoid drops in time series which can limit the usability of Cloud Spanner custom metrics.

## Background
Cloud Monitoring has 200,000 active time series (streams) [limits](https://cloud.google.com/monitoring/quotas) for custom metrics per project.
Active time series translates to streams that are sent in the last 24 hours.
This means that the number of streams that can be sent by OpenTelemetry collector should be controlled to this limit as the metrics will be dropped by Cloud Monitoring.

## Calculation
For simplicity purpose and for explaining of calculation, which is currently done in implementation, lets assume that we have 1 project, 1 instance and 1 database.
Such calculations are done when the collector starts. If the cardinality total limit is different(for calculations below we assume it is 200,000) the numbers will be different.

According to the metrics [metadata configuration](internal/metadataconfig/metrics.yaml)(at the moment of writing this document) we have:
- 30 low cardinality metrics(amount of total N + active queries summary metrics);
- 26 high cardinality metrics(amount of top N metrics).

For low cardinality metrics, the allowed amount of time series is **projects x instances x databases x low cardinality metrics** and in our case is **1 x 1 x 1 x 30 = 30** (1 per each metric).

Thus, the remaining quota of allowed time series in 24 hours is **200,000 - 30 = 199.970**.

For high cardinality metrics we have **199970 / 26 = 7691** allowed time series per metric.

This means each metric converted from these columns should not have a cardinality of 7691 per day.

We have one time series per minute, meaning that we can have up to 1,440 (24 hours x 60 minutes per hour = 1440) values per day.

Taking into account 1440 time series we'll obtain **7691 / 1440 = 5** new time series per minute.


## Labels
For each metric in the Total N table, the labels that create a unique time series are **Project ID + Instance ID + Database ID**.

For each metric in the Top N table, the labels that create a unique time series are **Project ID + Instance ID + Database ID + one of Query/Txn/Read/RowKey**.

While fingerprint and truncated field are part of some Top N table, it does not add to cardinality as there is 1 to 1 correspondence with Query/Txn/Read to their fingerprint and truncated information.

Cardinality is the number of unique sets of labels that are sent to Cloud Monitoring to create a new time series over a 24-hour period.

## Detailed Design
To avoid confusion between labels and queries/txn shapes/read shapes and row key, queries will be loosely used in the below design to make it easier for understanding the design.

Also, the **7691** time series limit is used as an example.
This needs to be made configurable based on the number of databases, number of metrics per table and future growth of the metrics per table.

With **7691** time series limit and each metric is sent once per minute, there are **60 minute x 24 hours  = 1440** time series data will be sent to Cloud Monitoring.

To allow new queries (new Top queries which are not seen in the last 24 hours) to be sent to Cloud Monitoring, each minute should have an opportunity to send new queries.
Assuming a uniform distribution of new queries being created in Top N tables, the algorithm should handle **floor(7691/1440) = 5** new queries per minute.
These 5 new queries should be the Top 5 queries which were not exported in the last 24 hours.

While the existing time series (queries which have been exported in the last 24 hours) should be good to export, exporting more than 100(**top_metrics_query_max_rows** receiver config parameter) queries per minute may be an overkill.
Customers can tune the maximum number of queries per minute per metric to a smaller value if required.

An LRU cache with TTL of 24 hours is maintained per each Top N metric.
For each collection interval(1 minute), cache will be populated with no more than 5 new queries, (i.e) if cache does not have an entry for the query, it will be added to the cache.
This can be done no more than 5 times every time.
This means that after the first minute, collector can send 5 queries, second minute collector can send 10 queries(5 new and 5 existing queries sent in the first minute which is also seen in top 100 for second minute), third minute collector can send 15 queries(5 new and 10 existing queries sent in the first two minute which is also seen in top 100 for third minute).
In a normal state after 20 minutes, the collector will be sending about 100 of queries per minute if the top 95 queries are already in the cache and 5 new queries(if it is in top 100 queries) are added to the cache in that interval.

Since LRU cache is in memory of collector instance - you'll have separate cache and limits per instance-metric.
Ability to use external caching for this is currently out of scope.
In case of multiple collector instances used you need to remember that handling overall total cardinality limit for all metrics will be user responsibility and needs to be properly defined in receiver configuration.
Also, there is ability to turn on/off cardinality handling using receiver configuration(it is turned off by default).

## Limitation
While this algorithm is pessimistic, it provides the guarantee that new 5 queries in top 100 queries will be seen all the time during normal operation without losing information.
If there are 6 new queries which are seen in a minute, the top 5 new queries will be sent and the 6th one will not be sent in that minute.
Next minute may capture that query if that query is still in the top 5 new query.

To alleviate this limitation of 5 queries at the start of the collector, the receiver can collect the last 20 minutes of Top Queries(like replay) and populate the LRU cache.
This will allow the receiver to send 100 queries at the first minute.
Backfilling will also populate the cache with the last one hour of queries which can avoid 5 query limitations and still maintain 5 new query per minute going forward.

## Alternatives Considered
One option that was considered is to allow aggressive collection where the top 100 will always be sent till the limit is reached.
This means if every minute has new top 100 queries(worst case), new queries after the limit of 7691 cannot be sent. The limit can be reached in 76 minutes (7691/100 new queries per minute = 76 minutes).
Though it is impractical to see 100 new queries all the time in top 100 queries, it is quite possible that the limit can be hit easily if the number of time series per metric is limited to 2500 queries per day to allow for more databases to be supported or if the queries are using literals instead of parameters.

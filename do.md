With independent partition processing, these metrics help distinguish whether:

- Partition workers are waiting for Kafka data → consider raising `max_prefetch`.
- Records are being processed slowly → consider raising `max_in_flight`.

All three metrics have `topic` and `partition` attributes:

| Metric | Definition |
|---|---|
| `kafka_receiver_partition_data_wait` | Monotonic seconds Counter of completed waits for data with no queued batch, current batch, or active call. Record elapsed time when each wait ends, including on partition loss or shutdown. Exclude ongoing waits, batch draining, retries, rewinds, and time after shutdown. Stale notifications do not end or restart a wait. |
| `kafka_receiver_partition_in_flight_records` | UpDownCounter of active `handleMessage` calls, including decode, Consume, and retry backoff. Export as a non-monotonic sum; exclude queued records, completed calls awaiting marking, and calls from stopped assignments. |
| `kafka_receiver_record_processing_duration{stage}` | Seconds histogram around each `decode` and `consume` attempt, including failures; exclude retry backoff. |

The SDK accumulates completed wait durations across reassignments. Ongoing waits become visible only when they end, so a long idle period produces a delayed increase in the counter.

Partition telemetry is scoped to one assignment; no telemetry map, snapshot, or collection callbacks are retained. On partition loss, terminal pause, or shutdown, flush the current wait and clear the assignment's in-flight count once. Ignore later starts and completions from that stopped assignment so they cannot affect a replacement assignment. Zero counts can remain visible after an assignment stops.

When Kafka backlog exists:

- High rate of `kafka_receiver_partition_data_wait` → consider raising `max_prefetch`.
- `kafka_receiver_partition_in_flight_records` near `max_in_flight.records` → consider raising `max_in_flight.records` after checking stage durations and retry logs.
- High `kafka_receiver_record_processing_duration{stage="consume"}` → investigate downstream blocking; high `{stage="decode"}` → investigate decoding.

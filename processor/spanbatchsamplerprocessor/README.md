# Span Batch Sampler

| Status                   |                |
| ------------------------ | ---------------|
| Stability                | [alpha]        |
| Supported pipeline types | traces         |
| Distributions            | [contrib]      |

Typically span collection occurs until a memory limit is hit, at which point spans start to get dropped. This doesn’t ensure that you are getting a diverse distribution of spans in terms of the operation name. Spans that happen very frequently can drown out spans that rarely occur.

The span batch sampler takes a batch of spans, organizes them by span name, then evenly exports them. This happens until a configurable upload-quota is reached for the batch (tokens). Excess spans are dropped in each batch, making this an intelligent sampling strategy. This allows you to ensure less frequent spans are still observed with fairness in noisy environments.

To improve the processor's ability to sample spans, this processor should be used downstream from the `batch_processor` if the data source is not batching. This allows configuring the maximum size of batches processed by the span batch processor and allows fine-grained tuning of the volume of data to process (or drop) when operating at scale. The example config below drops a maximum of 90% spans. If batching isn't occurring in the client or upstream in the collector this processor will be ineffective.

Example of using the `span_batch_sampler` processor downstream of the batch processor:
```
  batch:
    send_batch_size: 1000
    send_batch_max_size: 1000
  span_batch_sampler:
    tokens: 100
```


The following settings are accepted:

- `tokens`: (Required) An integer, determines the maximum number of spans to export per batch
- `most_popular`: (default = false) A bool, if set to true more popular spans are prioritized

```yaml
processors:
  span_batch_sampler:
    tokens: 100
```

## Enable with
- `span_batch_sampler` listed as a processor
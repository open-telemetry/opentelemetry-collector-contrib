# Span Batch Filter Processor

| Status                   |                |
| ------------------------ | ---------------|
| Stability                | [alpha]        |
| Supported pipeline types | traces         |
| Distributions            | [contrib]      |

Without this processor span collection occurs until a memory limit is hit, at which point spans start to get dropped. This doesn’t ensure that you are getting a diverse distribution of spans in terms of operation types. Spans that happen very frequently can drown out spans that rarely occur.

This span filter takes a batch of spans, organizes them by span name, then exports them in order of least popular name, until it runs out of tokens in the batch. Excess spans are dropped in each batch, making this an intelligent sampling strategy. This allows you to ensure less frequent spans are still observed with fairness in noisy environments.

This processor can also be used downstream from the `batch_processor`. This would allow you to limit the maximim size of the batch that this popularity processor receives. This allows you to tune how much data you drop at scale. The example config below drops a maximum of 90% spans. Most popular spans can be prioritized by setting `most_popular: True`.

Example with `span_batch_filter` downstream
```
  batch:
    send_batch_size: 1000
    send_batch_max_size: 1000
  span_batch_filter:
    tokens: 100
```


The following settings are accepted:

- `tokens`: (Required) An integer, determines the maximum number of spans per batch
- `most_popular`: (default = False) A bool, more popular spans are prioritized

```yaml
processors:
  span_batch_filter:
    tokens: 100
```

## Enable with
- `span_batch_filter` listed as a processor
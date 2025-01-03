# OpenTelemetry to Sentry Transformation

This document aims to define the transformations between an OpenTelemetry span and a [Sentry Span](https://develop.sentry.dev/sdk/event-payloads/span/). It will also describe how a Sentry transaction is created from a set of Sentry spans.

## Spans

| Sentry              | OpenTelemetry                           | Notes                                                                                                             |
| ------------------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Span.TraceID        | Span.TraceID                            |                                                                                                                   |
| Span.SpanID         | Span.SpanID                             |                                                                                                                   |
| Span.ParentSpanID   | Span.ParentSpanID                       | If a span does not have a parent span ID, it is a root span (considered the start of a new transaction in Sentry) |
| Span.Description    | Span.Name, Span.Attributes, Span.Kind   | The span description is decided using OpenTelemetry Semantic Conventions                                          |
| Span.Op             | Span.Name, Span.Attributes, Span.Kind   | The span op is decided using OpenTelemetry Semantic Conventions                                                   |
| Span.Tags           | Span.Attributes, Span.Kind, Span.Status | The otel span status message and span kind are stored as tags on the Sentry span                                  |
| Span.StartTimestamp | span.StartTime                          |                                                                                                                   |
| Span.EndTimestamp   | span.EndTime                            |                                                                                                                   |
| Span.Status         | Span.Status                             |                                                                                                                   |

As can be seen by the table above, the OpenTelemetry span and Sentry span map fairly reasonably. Currently the OpenTelemetry `Span.Link` and `Span.TraceState` properties are not used when constructing a `SentrySpan`

## Transactions

To ingest spans into Sentry, they must be sorted into transactions, which is made up of a root span and it's corresponding child spans, along with useful metadata.

Long-running traces may be split up into different transactions. This will still appear as a single trace in the Sentry UI.

### Implementation

We first iterate through all spans in a trace to figure out which spans are root spans. As with our definition, this can be easily done by just checking for an empty parent span id. If a root span is found, we can create a new transaction. Along the way, if we find any children that belong to the DAG under that root span, we can assign them to that transaction.

After this first iteration, we are left with two structures, an array of transactions, and an array of orphan spans, which we could not classify under a transaction in the first pass.

We can then try again to classify these orphan spans, but if not possible, we can assume these orphan spans to be a root span (as we could not find their parent in the trace). Those root spans generated from orphan spans can be also be then used to create their respective transactions.

The interface for a Sentry Transaction can be found [here](https://develop.sentry.dev/sdk/event-payloads/transaction/)

| Sentry                        | Used to generate                               |
| ----------------------------- | ---------------------------------------------- |
| Transaction.Contexts["trace"] | RootSpan.TraceID, RootSpan.SpanID, RootSpan.Op |
| Transaction.Spans             | ChildSpans                                     |
| Transaction.Sdk.Name          | `sentry.opentelemetry`                         |
| Transaction.Tags              | Resource.Attributes, RootSpan.Tags             |
| Transaction.StartTimestamp    | RootSpan.StartTimestamp                        |
| Transaction.Timestamp         | RootSpan.EndTimestamp                          |
| Transaction.Transaction       | RootSpan.Description                           |

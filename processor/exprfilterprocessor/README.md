### Expr Filter Processor

The Collector currently provides a filterprocessor that can filter metrics based on their names:
 
https://github.com/open-telemetry/opentelemetry-collector/tree/master/processor/filterprocessor
 
However, users have requested the ability to filter metrics based on their names + label keys + label
values. The expr filter processor intends to provide a way for Collector users to use a simple expression
language to filter metrics by arbitrary conditions, filtering by metric names and/or datapoint labels.

The expr library (https://github.com/antonmedv/expr) is a high-performance expression parser and virtual
machine. It should be able to provide acceptable performance and flexibility to this end.

This filter will accept an expr expression provided and compiled at configuration time, and evaluated
_per datapoint_. The results, however, will be applied _per metric_. If any datapoint in a metric causes the
expression to evaluate to true, the entire metric will be filtered out.

The initial design is that the expr environment will have available to it the following metric attributes:

* MetricName
    * the name of the current datapoint's metric
* Label(string)
    * a function that returns the string value of the passed-in label name, or an empty string if not found
* HasLabel(string)
    * a function that returns true if the current datapoint has the specified label

An example expr filter string might therefore be:

`MetricName == 'my.metric' && Label('my.label') == 'some.value'`

The above string passed to the exprfilter config will cause the expr filter processor to filter out any
metric whose name is 'my.metric' and whose datapoint collection has one datapoint with a label of
'my.label=some.value'.

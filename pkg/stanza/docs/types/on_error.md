# `on_error` parameter
The `on_error` parameter determines the error handling strategy an operator should use when it fails to
process an entry. There are 4 supported values: `drop`, `drop_quiet`, `send` and `send_quiet`.

Regardless of the method selected, all processing errors will be logged by the operator.

### `drop`
In this mode, if an operator fails to process an entry, it will drop the entry altogether.
This will stop the entry from being sent further down the pipeline.

### `drop_quiet`
Same as `drop` with only difference that the failure will be logged in debug level. Useful, when best effort
operators are defined which might flood the logs with errors.

### `send`
In this mode, if an operator fails to process an entry, it will still send the entry down the pipeline.
This may result in downstream operators receiving entries in an undesired format.

### `send_quiet`
Same as `send` with only difference that the failure will be logged in debug level. Useful, when best effort
operators are defined which might flood the logs with errors.

# CheckAPI

**NOTE: CheckAPI is in development and immature. It is used primarily for opentelemetry-collector-contrib. Contributions are welcome.**

CheckAPI is a go tool that parses the AST tree of a Go module, identifying Golang APIs such as structs and functions
and enforcing rules against them.

This is particularly useful to reduce the API surface of a Go module to a specific set of functions.

## Running CheckAPI

```shell
$> checkapi -folder . -config config.yaml
```

## Configuration

The configuration file is in yaml format:

```yaml
ignored_paths:
  - <exact relative paths of Golang modules to ignore>
allowed_functions:
  - <at least one function match must be present.>
  - name: <name of function>
    parameters: <list of parameters by type>
    return_types: <list of return types>

ignored_functions:
  - <regular expressions of ignored functions. At least one match must be present to ignore the function.>
```

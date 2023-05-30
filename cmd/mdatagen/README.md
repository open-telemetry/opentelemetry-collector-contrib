# Metadata Generator

Components must contain a `metadata.yaml` file that documents various aspects of the component including:

* its stability level
* the distributions containing it
* the types of pipelines it supports
* metrics emitted in the case of a scraping receiver

Current examples:

* hostmetricsreceiver scrapers like the [cpuscraper](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/metadata.yaml)

See [metadata-schema.yaml](metadata-schema.yaml) for file format documentation.

If adding a new receiver a `doc.go` file should also be added to trigger the generation. See below for details.

## Generating

`make generate` triggers the following actions:

1. `cd cmd/mdatagen && $(GOCMD) install .` to install `mdatagen` tool in`GOBIN`

2. All `go:generate mdatagen` directives that are usually defined in `doc.go` files of the metric scrapers will be 
   executed. For example,
   [/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/doc.go](../../receiver/hostmetricsreceiver/internal/scraper/cpuscraper/doc.go)
   runs `mdatagen` for the [metadata.yaml](../../receiver/hostmetricsreceiver/internal/scraper/cpuscraper/metadata.yaml)
   which generates the metrics builder code in
   [internal/metadata](../../receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata)
   and [documentation.md](../../receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata) with
   generated documentation about emitted metrics.

## Development

In order to introduce support of a new functionality in metadata.yaml:

1. Make code changes in the (generating code)[./loader.test] and (templates)[./templates/].
2. Add usage of the new functionality in (metadata.yaml)[./metadata.yaml].
3. Run `make mdatagen-test`.
4. Make sure all tests are passing including (generated tests)[./internal/metadata/generated_metrics_test.go].
5. Run `make generate`.


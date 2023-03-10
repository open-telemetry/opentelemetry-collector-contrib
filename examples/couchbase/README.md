# Couchbase Example

This example shows how you can configure the collector to scrape a [Couchbase server](https://www.couchbase.com/products/server) using its built-in Prometheus metrics exporter.

This [example config](./otel-collector-config.yaml) retrieves a subset of metrics important for monitoring a Couchbase server, transforms them to follow OpenTelemetry conventions, then exports again to prometheus so that they may be easily viewed.

## Running

First build the collector docker image by running the following command in the root of this repository:
```sh
make docker-otelcontribcol
```

Then, start Couchbase, the collector, and a Prometheus server by running the following command in this directory:
```sh
docker-compose up -d --remove-orphans --build --force-recreate
```

After Couchbase is running, you'll need to initialize Couchbase and create some buckets:
```sh
./scripts/setup.sh
```

You should now be able to view metrics being scraped from the Couchbase instance every 5 seconds in Prometheus at http://localhost:9090 (all metrics are prefixed with `couchbase_`)

Once you are done with the example, you may shut it down:
```sh
docker-compose down
```

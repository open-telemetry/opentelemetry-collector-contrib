# Prometheus Remote Write Exporter Example
This example uses [Docker Compose](https://docs.docker.com/compose/) to set up:

1. A Python program that creates 5 instruments with 5 unique
aggregators and a randomized load generator
2. An instance of [Cortex](https://cortexmetrics.io/) to receive the metrics
data
3. An instance of [Grafana](https://grafana.com/) to visualizse the exported
data

## Requirements
* Have Docker Compose [installed](https://docs.docker.com/compose/install/)

*Users do not need to install Python as the app will be run in the Docker Container*

## Instructions
1. Run `docker-compose up -d` in the the `examples/` directory

The `-d` flag causes all services to run in detached mode and frees up your
terminal session. This also causes no logs to show up. Users can attach themselves to the service's logs manually using `docker logs ${CONTAINER_ID} --follow`

2. Log into the Grafana instance at [http://localhost:3000](http://localhost:3000)
   * login credentials are `username: admin` and `password: admin`
   * There may be an additional screen on setting a new password. This can be skipped and is optional

3. Navigate to the `Data Sources` page
   * Look for a gear icon on the left sidebar and select `Data Sources`

4. Add a new Prometheus Data Source
   * Use `http://cortex:9009/api/prom` as the URL
   * (OPTIONAl) set the scrape interval to `2s` to make updates appear quickly
   * click `Save & Test`

5. Go to `Metrics Explore` to query metrics
   * Look for a compass icon on the left sidebar
   * click `Metrics` for a dropdown list of all the available metrics
   * (OPTIONAL) Adjust time range by clicking the `Last 6 hours` button on the upper right side of the graph
   * (OPTIONAL) Set up auto-refresh by selecting an option under the dropdown next to the refresh button on the upper right side of the graph
   * Click the refresh button and data should show up on the graph

6. Shutdown the services when finished
   * Run `docker-compose down` in the examples directory

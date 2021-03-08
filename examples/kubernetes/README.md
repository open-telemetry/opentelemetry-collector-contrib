# OpenTelemetry Collector Demo

This demo is a sample app to build the collector and exercise its kubernetes logs scrapping functionality.

## Build and Run

Two steps are required to build and run the demo:

1. Build latest docker image in main repository directory `make docker-otelcontribcol`
1. Switch to this directory and run `docker-compose up`

## Description

`varlogpods` contains example log files placed in kubernetes-like directory structure.
Each of the directory has different formatted logs in one of three formats (either `CRI-O`, `CRI-Containerd` or `Docker`).
This directory is mounted to standard location (`/var/log/pods`).

`otel-collector-config` is a configuration to autodetect and parse logs for all of three mentioned formats

## ToDo

To cover kubernetes system logs, logs from journald should be supported as well.

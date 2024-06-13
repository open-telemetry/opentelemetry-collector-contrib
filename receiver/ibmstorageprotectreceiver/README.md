# sp-diagnostic-tool

## OpenTelemetry Pipeline Setup
### Overview
The OpenTelemetry pipeline facilitates the collection and export of telemetry data. In this setup, we utilize the OTLPJsonFile receiver for data ingestion and a debug exporter for debugging purposes.

### Understanding Components

**receiverOTLPjson** : This directory includes source code of OTLPJsonfile receiver and debug exporter for debugging purposes.
Within this directory, there are following files:

- **builder-config.yaml**: This YAML file includes go.mod modules of the component that we need to include in our collector/pipeline.
As we are building customised receiver and exporter hence, we don't need to go.mod modules of these components. 

- **config.yaml**: This YAML file handles the configuration details about the receiver, processor (optional), exporter and kind of telemetry data to be used i.e. traces, logs or metrics in the pipeline.

- **go.mod**: It defines the module's path and sets it up for dependency management

- **go.sum**: It provides checksums for the exact contents of each dependency at the time it is added to the module.

- **model.go**: This file contains the source code for OTLPJsonfile receiver and Debug exporter.

- **output.json**: The receiver will watch this file. If this file is updated or new entries added, the receiver will read it entirety again.

- **parser.go**: This is the datatransformer script, which takes XML files folder as input and converts it into receiver readable json format.

**otelcol-dev**: This directory is responsible for running the pipeline. It encompasses all the necessary code, including the creation of factory objects for receiver and exporter components, notably defined in the components.go file.

### Start Pipeline: 
#### Run the following command to initiate the pipeline and ensure to specify the respective paths for otelcol-dev and config.yaml:
    go run ./otelcol-dev --config ./otlpjsonfile/config.yaml



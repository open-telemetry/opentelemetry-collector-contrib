# Sflow Receiver

 | Status                   |           |
 | ------------------------ | --------- |
 | Stability                | [alpha]   |
 | Supported pipeline types | logs    |
 | Distributions            | [contrib] |

 ## Overview
 The Sflow receiver accepts and decodes sflow records from device vendors to otlp logs

 ## Configuration

 Example:

 ```yaml
receivers:
  sflow:
    endpoint: 0.0.0.0:9995
 ```

 # Custom labels (Optional)
 
 ```yaml
 receivers:
  sflow:
    endpoint: 0.0.0.0:9995
    labels:
      mylabel1: value1
      mylabel2: value2
 ```
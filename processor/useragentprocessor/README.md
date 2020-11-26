# User Agent Processor
Supported pipeline types: traces

## Description
This processor extracts browser, device and os information from raw useragent and adds them as attributes. It uses [uap-go](https://github.com/ua-parser/uap-go) 
library to extract this information. At the crux of it, it relies on [regex yaml](https://github.com/ua-parser/uap-core/blob/286809e09706ea891b9434ed875574d65e0ff6b7/regexes.yaml) file that is maintained by the community.

Example:
```
processors:
    #user agent regex master file to parse raw useragent data. Default file is picked from resources directory. Source: https://github.com/ua-parser/uap-core/blob/master/regexes.yaml
    #Provision to override.
    useragent_filepath: "/app/config/customUserAgentRegexes.yaml"
```

## Run
- via Docker
    ```
     make docker-otelcontribcol
     docker run --rm -p 8080:8080 -p 8888:8888 \
           -v "${PWD}/processor/useragentprocessor/testdata/otelconfig.yaml":/otel-local-config.yaml \
          -v "${PWD}/processor/useragentprocessor/resources/regexes.yaml":/useragent-regexes.yaml \
          --name otelcol otelcontribcol:latest \
           --config otel-local-config.yaml \
           --metrics-addr=0.0.0.0:8888
    ```

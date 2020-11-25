# User Agent Processor
Supported pipeline types: traces

## Description
This processor extracts browser, device and os information from raw useragent and adds them as attributes. It uses [uap-go](https://github.com/ua-parser/uap-go) 
library to extract this information. At the crux of it, it relies on [regex yaml](https://github.com/ua-parser/uap-core/blob/286809e09706ea891b9434ed875574d65e0ff6b7/regexes.yaml) file that is maintained by the community.

Example:

```
processors:
  useragentprocessor:
    #key to look for in span/trace attributes that contain the raw trace agent.
    #defaults to `userAgent` if nothing is set.
    useragent_tag: "userAgent"

    #user agent regex master file to parse raw useragent data. Default file is picked from resources directory. Source: https://github.com/ua-parser/uap-core/blob/master/regexes.yaml
    #Provision to override.
    useragent_filepath: "/app/config/customUserAgentRegexes.yaml"
```
## Configuration
Refer to [config.yaml](testdata/config.yaml) for detailed examples on using the processor.
  


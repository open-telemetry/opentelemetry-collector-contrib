## Summary
`s3provider` is a `confmap.Provider` implementation for **Amazon S3**, that provides the ability to load collector configuration by fetching and reading config objects stored in Amazon S3.
## How it works
- It will be called by `confmap.Resolver` to load the configuration for the collector.
- By giving a config URI starting with ***prefix-scheme*** `s3://`, this `s3provider` will be used to download config objects from the given S3 URIs, and then use the downloaded configuration during Collector initialization.

Expected URI format:
``` url
s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]
```

### Prerequisites
- Need to setup access keys from IAM console (`aws_access_key_id` and `aws_secret_access_key`) with permissions to access Amazon S3.
- For more details, refer [Amazon SDK](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/).

### Usage
```go
// scheme for s3provider is s3
uris := []string{"s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]"}
providers := make(map[string]confmap.Provider)
providers[s3provider.Scheme()] = s3provider.New() 
// add other providers if you want
// providers["file"] = fileprovider.New()

cp := service.NewConfigProvider(service.ConfigProviderSettings{
    ResolverSettings: confmap.ResolverSettings{
        URIs:       uris,
        Providers:  providers,
        Converters: []confmap.Converter{expandconverter.New()},
    },
)
```
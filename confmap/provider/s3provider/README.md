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
`s3provider` internally uses `aws-sdk-go-v2` which requires credentials (an access key and secret access key) to sign requests to AWS. You can specify your credentials in several locations, depending on your particular use case. For information about obtaining credentials, see [Amazon SDK](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials).

### Usage
```go
// scheme for s3provider is s3
uris := []string{"s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]"}
providers := make(map[string]confmap.Provider)

// opts to define on how the aws-sdk will load the access keys. 
//For more info, refer: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config#LoadOptionsFunc
opts := []func(*config.LoadOptions) error{
    config.WithSharedCredentialsFiles(
        []string{"test/credentials", "data/credentials"},
    ),
    config.WithSharedConfigFiles(
        []string{"test/config", "data/config"},
    ),
}

providers[s3provider.Scheme()] = s3provider.New(opts...) 
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
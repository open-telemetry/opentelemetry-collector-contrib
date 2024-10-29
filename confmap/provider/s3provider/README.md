## Summary
This package provides a `ConfigMapProvider` implementation for Amazon S3 (`s3provider`) that allows the Collector the ability to load configuration by fetching and reading config objects stored in Amazon S3.
## How it works
- It will be called by `ConfigMapResolver` to load configuration for the Collector.
- By giving a config URI starting with prefix `s3://`, this `s3provider` will be used to download config objects from the given S3 URIs, and then use the downloaded configuration during Collector initialization.

Expected URI format:
- s3://[BUCKET].s3.[REGION].amazonaws.com/[KEY]

Prerequistes:
- Need to setup access keys from IAM console (aws_access_key_id and aws_secret_access_key) with permission to access Amazon S3
- For details, can take a look at https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
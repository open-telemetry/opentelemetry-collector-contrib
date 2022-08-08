What is this new component s3mapprovider?
- An implementation of ConfigMapProvider for Amazon S3 (s3mapprovider) allows OTEL Collector the ability to load configuration for itself by fetching and reading config files stored in Amazon S3.

How this new component s3mapprovider works?
- It will be called by ConfigMapResolver to load configurations for OTEL Collector.
- By giving a config URI starting with prefix 's3://', this s3mapprovider will be used to download config files from given S3 URIs, and then used the downloaded config files to deploy the OTEL Collector.
- In our code, we check the validity scheme and string pattern of S3 URIs. And also check if there are any problems on config downloading and config deserialization.
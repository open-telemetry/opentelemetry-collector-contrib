# MongoDB Atlas Receiver

Receives metrics from [MongoDB Atlas](https://www.mongodb.com/cloud/atlas) 
via their [monitoring APIs](https://docs.atlas.mongodb.com/reference/api/monitoring-and-logs/)

Supported pipeline types: metrics

## Getting Started

The MongoDB Atlas receiver takes the following parameters. `public_key` and 
`private_key` are the only two required values and are obtained via the 
"API Keys" tab of the MongoDB Atlas Project Access Manager. In the example
below both values are being pulled from the environment.

- `public_key`
- `private_key`
- `granularity` (default `PT1M` - See [MongoDB Atlas Documentation](https://docs.atlas.mongodb.com/reference/api/process-measurements/))

Examples:

```yaml
receivers:
  mongodbatlas:
    public_key: ${MONGODB_ATLAS_PUBLIC_KEY}
    private_key: ${MONGODB_ATLAS_PRIVATE_KEY}
```



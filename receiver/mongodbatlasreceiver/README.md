# MongoDB Atlas Receiver

Receives metrics from [MongoDB Atlas](https://www.mongodb.com/cloud/atlas) 
via their [monitoring APIs](https://docs.atlas.mongodb.com/reference/api/monitoring-and-logs/)

Supported pipeline types: metrics

## Getting Started

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



# Enrichment Processor

The enrichment processor is a component that enriches telemetry data (traces, metrics, and logs) with additional resource metadata from external inventory data sources. This processor is particularly useful for adding contextual information like ownership details, environment information, configuration data, and other metadata that helps with better monitoring, alerting, and reporting.

## Features

- **Multiple Data Source Support**: HTTP APIs, CSV files, JSON files, and Prometheus endpoints
- **Flexible Enrichment Rules**: Define custom rules for when and how to enrich data
- **Caching**: Built-in caching to improve performance and reduce external API calls
- **Conditional Enrichment**: Apply enrichment rules based on attribute conditions
- **Field Transformations**: Apply transformations to enrichment data before adding to telemetry
- **All Telemetry Types**: Support for traces, metrics, and logs

## Configuration

The processor is configured using the following structure:

```yaml
processors:
  enrichment:
    # Data sources configuration
    data_sources:
      - name: "inventory_api"
        type: "http"
        http:
          url: "https://api.example.com/inventory"
          headers:
            Authorization: "Bearer token123"
          timeout: 30s
          refresh_interval: 5m
          json_path: "$.data"
      
      - name: "host_metadata"
        type: "file"
        file:
          path: "/etc/collector/host_metadata.json"
          format: "json"
          refresh_interval: 1m
    
    # Cache configuration
    cache:
      enabled: true
      ttl: 5m
      max_size: 1000
    
    # Enrichment rules
    enrichment_rules:
      - name: "enrich_database_metrics"
        data_source: "inventory_api"
        lookup_key: "service.name"
        lookup_field: "service_name"
        conditions:
          - attribute: "service.type"
            operator: "equals"
            value: "database"
        mappings:
          - source_field: "owner_org"
            target_attribute: "org.id"
          - source_field: "environment"
            target_attribute: "deployment.environment"
          - source_field: "version"
            target_attribute: "service.version"
            transform: "lower"
      
      - name: "enrich_host_metrics"
        data_source: "host_metadata"
        lookup_key: "host.name"
        lookup_field: "hostname"
        mappings:
          - source_field: "location"
            target_attribute: "host.location"
          - source_field: "manufacturer"
            target_attribute: "host.manufacturer"
          - source_field: "os_version"
            target_attribute: "os.version"
```

## Configuration Parameters

### Data Sources

#### HTTP Data Source
- **url**: HTTP endpoint URL
- **headers**: Optional HTTP headers
- **timeout**: Request timeout (default: 30s)
- **refresh_interval**: How often to refresh data (default: 5m)
- **json_path**: JSONPath expression to extract data from response

#### File Data Source
- **path**: Path to the file
- **format**: File format (json, csv)
- **refresh_interval**: How often to check for file changes (default: 1m)

#### Prometheus Data Source (Future)
- **url**: Prometheus endpoint URL
- **query**: PromQL query to execute
- **headers**: Optional HTTP headers
- **refresh_interval**: How often to refresh data

### Cache Configuration
- **enabled**: Enable/disable caching (default: true)
- **ttl**: Time to live for cache entries (default: 5m)
- **max_size**: Maximum number of cache entries (default: 1000)

### Enrichment Rules
- **name**: Unique name for the rule
- **data_source**: Name of the data source to use
- **lookup_key**: Attribute name to use for lookup
- **lookup_field**: Field name in data source to match against
- **conditions**: Optional conditions for when to apply the rule
- **mappings**: How to map data source fields to telemetry attributes

### Field Mappings
- **source_field**: Field name in the data source
- **target_attribute**: Attribute name in telemetry data
- **transform**: Optional transformation (upper, lower, trim)

### Conditions
- **attribute**: Attribute name to check
- **operator**: Comparison operator (equals, contains, regex, not_equals)
- **value**: Value to compare against

## Use Cases

### Database Metrics Enrichment
Add owner information, environment details, and configuration metadata to database metrics:

```yaml
enrichment_rules:
  - name: "database_enrichment"
    data_source: "cmdb_api"
    lookup_key: "db.name"
    lookup_field: "database_name"
    conditions:
      - attribute: "service.type"
        operator: "equals"
        value: "database"
    mappings:
      - source_field: "owner_team"
        target_attribute: "team.name"
      - source_field: "environment"
        target_attribute: "deployment.environment"
      - source_field: "max_connections"
        target_attribute: "db.max_connections"
```

### Message Queue Metrics Enrichment
Add capacity, ownership, and configuration details to MQ metrics:

```yaml
enrichment_rules:
  - name: "mq_enrichment"
    data_source: "mq_inventory"
    lookup_key: "mq.queue_name"
    lookup_field: "queue_name"
    mappings:
      - source_field: "max_depth"
        target_attribute: "mq.max_depth"
      - source_field: "owner_org"
        target_attribute: "org.id"
      - source_field: "qmgr_name"
        target_attribute: "mq.manager"
```

### Host Metrics Enrichment
Add system information and ownership details to host metrics:

```yaml
enrichment_rules:
  - name: "host_enrichment"
    data_source: "asset_db"
    lookup_key: "host.name"
    lookup_field: "hostname"
    mappings:
      - source_field: "location"
        target_attribute: "host.location"
      - source_field: "environment"
        target_attribute: "deployment.environment"
      - source_field: "os_build"
        target_attribute: "os.build"
```

## Data Source Formats

### JSON File Format
```json
[
  {
    "service_name": "user-service",
    "owner_team": "platform",
    "environment": "production",
    "version": "1.2.3"
  },
  {
    "service_name": "payment-service",
    "owner_team": "payments",
    "environment": "production",
    "version": "2.1.0"
  }
]
```

### CSV File Format
```csv
service_name,owner_team,environment,version
user-service,platform,production,1.2.3
payment-service,payments,production,2.1.0
```

### HTTP API Response Format
```json
{
  "data": [
    {
      "service_name": "user-service",
      "owner_team": "platform",
      "environment": "production",
      "version": "1.2.3"
    }
  ]
}
```

## Performance Considerations

1. **Caching**: Enable caching to reduce external API calls
2. **Refresh Intervals**: Set appropriate refresh intervals based on data change frequency
3. **Conditions**: Use conditions to avoid unnecessary enrichment operations
4. **Data Source Size**: Consider the size of your data sources and cache limits

## Error Handling

The processor handles errors gracefully:
- Failed lookups are logged but don't stop processing
- Network timeouts are handled with retries
- Invalid configurations are caught at startup
- Malformed data sources are logged and skipped

## Metrics

The processor exposes the following metrics:
- `enrichment_requests_total`: Total number of enrichment requests
- `enrichment_failures_total`: Total number of enrichment failures  
- `enrichment_cache_hits`: Number of cache hits
- `enrichment_cache_misses`: Number of cache misses

## Future Enhancements

- Prometheus data source support
- Additional transformation functions
- Support for nested attribute enrichment
- Integration with service discovery systems
- Support for multiple lookup keys

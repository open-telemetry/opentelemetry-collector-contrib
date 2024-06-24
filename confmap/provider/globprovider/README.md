# Glob config provider

A configuration provider that can load multiple yaml files based on a Glob pattern.
This allows the following invocation:

```bash
./otelcol --config "glob:/etc/config/otel/integrations/*.yaml"
```

The config files are loaded and merged in alphabetical order, with later files having
priority. It is recommended not to rely on this, and to make the configurations independent.

# schemagen

`schemagen` is a tiny utility that walks a Go configuration file and emits
JSON Schema that mirrors the exported structs.

Script is only for temporary use. The aim is to facilitate the process of introducing schemas for
Open Telemetry Collector components. Ultimately, the configuration schemas would be maintained manually,
and the generation process would be reversed – Go config files from schemas.

> **Status:** In Development.

## Modes

Script can generate schemas for components and packages. There are two distinct modes of operation: 
`component` and `package`. Schemagen automatically detects the mode based on metadata.yaml file content.

### Component mode

The `component` mode is aimed at generating configurations for individual components like receivers, processors,
exporters, and connectors. 

In component mode, schema is generated in file named `config.schema.<ext>` to the chosen output folder
(by default it's input directory). You can optionally specify the root struct to use for schema generation
using the `-c` flag; if not provided, the tool defaults to using `Config`.

### Package mode

The `package` mode, on the other hand, is designed for generating schemas for entire packages. In this mode,
you provide the path to a directory containing multiple Go files that collectively define the configuration
for a package. This can be used to generate schemas for shared libraries or common configuration structures used across
multiple components.

In package mode every exported struct becomes a definition under `$defs` and the output file name is
`config.schema.<ext>`.

## Usage

Build a local binary:

```bash
make install-tools
```

After installing the tool you can execute it directly or through the helper Make target.

```bash
schemagen [flags] path/to/your/package/dir
```

The repository also provides a convenient wrapper:

```bash
make schemagen SRC=receiver/foo/somecomponent/
```

You can pass any CLI flags through the `FLAGS` variable:

```bash
make schemagen SRC=connector/countconnector/ FLAGS="-o=$(pwd)/schemas -t=json -c=CustomConfig"
```

You can also go to specific component folder and run `make schemagen` directly.

## CLI options

| Flag | Default                                                                | Purpose                                                       |
|------|------------------------------------------------------------------------|---------------------------------------------------------------|
| `-c` | `Config` in component mode                                             | Explicitly set the struct that should become the root schema. |
| `-o` | Given directory (or `outputFolder` from `.schemagen.yaml` if provided) | Folder that will receive the generated `*.schema.<ext>` file. |
| `-t` | `yaml`                                                                 | Output format. Accepts `yaml`, `yml`, or `json`.              |

The schema `$id` is derived from the Go package import path and the `$title` is the package name with the current
run mode (`component`/`package`) appended.

## Settings file

The script looks for a `.schemagen.yaml` file starting from the current working directory and walking up to
the repository root. Although settings file is optional it extends significantly capabilities a `schemagen`.
A practical example:

```yaml
mappings:
  time:
    Duration:
      schemaType: string
      format: duration
allowedRefs:
  - go.opentelemetry.io/collector
  - github.com/open-telemetry/opentelemetry-collector-contrib
componentOverrides:
  receiver/namedpipe:
    configName: 'NamedPipeConfig'
  receiver/filelog:
    configName: 'FileLogConfig'
```

- `mappings` tell schemagen how to treat specific selector expressions as
  primitive schema fields. Each mapping converts the Go type into a scalar
  `schemaType` and can also set the JSON Schema `format`. The original
  Go type shows up under the `x-customType` extension so consumers can still
  see the source information.
- `allowedRefs` lists repositories that schemagen can make references to when
  generating `$ref` pointers. This is useful when your configuration structs
  embed types from other repositories that also have schemagen-generated
  schemas. If repo is not listed here, schemagen will use type `any`.
- `componentOverrides` allow per-component configuration of the root struct
  name. This is useful when a component does not use the conventional
  `Config` struct name.

## Generated schema highlights

- Structs become `type: object` definitions; nested structs are hoisted into
  `#/$defs/<TypeName>` and referenced where needed.
- Maps translate into `additionalProperties` with the schema generated for the
  map value. Slices keep their element type under `items`.
- Pointers compile down to the schema of the pointed type and carry an
  `x-pointer` marker for tooling.
- Anonymous embedded structs are added via `allOf` with the referenced `$defs`.
- Struct fields must expose a `mapstructure` tag; the tag value becomes the
  property name so only documented configuration knobs make it into the schema.
- Selectors that match entries under `mappings` produce
  primitive properties that retain the original Go type under `x-customType`.
- Optional wrapper types (e.g. `Optional[FooConfig]`) set the `x-optional`
  marker so downstream tooling can highlight nullable fields.

You can see end-to-end examples in `cmd/schemagen/internal/testdata`, e.g.
`SimpleConfig.go` → `simple_config.schema.json` or the more involved
`ComplexTypeFieldConfig.go`.

## Limitations

The current parser intentionally skips or errors on:

- Default values
- Validation
- Channels
- Function-typed fields
- Generic type parameters
- Interface fields without concrete type information
- Standard JSON Schema `required` arrays (beyond the `x-optional` marker)
- Exported fields without `mapstructure` tags (ignored)

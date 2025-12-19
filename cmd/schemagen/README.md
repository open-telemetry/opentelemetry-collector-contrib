# schemagen

`schemagen` is a tiny utility that walks a Go configuration file and emits
JSON Schema that mirrors the exported structs.

Script is only for temporary use. The aim is to facilitate the process of introducing schemas for 
Open Telemetry Collector components. Ultimately, the configuration schemas would be maintained manually, 
and the generation process would be reversed – Go config files from schemas.

> **Status:** In Development.

## Roadmap

- [x] Basic struct parsing and schema generation
- [x] Support for maps, slices, pointers, and embedded structs
- [x] CLI tool for generating schema from a file or directory
- [x] Support for descriptions from struct field comments
- [x] Support for std library package references (e.g., `time.Duration`)
- [x] Tests and examples
- [ ] Support for external package references (e.g., `confighttp.HTTPClientSettings`)
- [ ] Support for additional struct tags (e.g., `mapstructure:",squash"`)

## Usage

Build a local binary:

```bash
 make install-tools
```

Run schemagen against a configuration package:

```bash
make schemagen SRC=path/to/your/config.go
```

or against a directory:

```bash
make schemagen SRC=path/to/your/config/dir
````

You can also pass the flags:

```bash
make schemagen SRC=connector/countconnector/ FLAGS="-o=<value> -t=<value>" 
```

The command parses the Go file, constructs the schema model, and writes the
result to disk. By default, it drops `config.schema.yaml` next to your
configuration file, but you can change both the basename and the format using
flags. If you want to experiment without writing files, see the developer
section for the lower-level helpers.

## CLI options

| Flag | Default                                 | Purpose                                                                              |
|------|-----------------------------------------|--------------------------------------------------------------------------------------|
| `-p` | NONE                                    | Prefix used for the schema `id`. The cleaned output path is appended to this prefix. |
| `-r` | Derived from the file name (PascalCase) | Root type name when multiple structs live inside the same file.                      |
| `-o` | `config.schema`                         | Basename for the emitted file (we append the extension automatically).               |
| `-t` | `yaml`                                  | Output type (`yaml` or `json`).                                                      |

When you pass a directory, schemagen assumes the root file is `config.go`. The
output path is resolved as `<dir>/<basename>.<ext>` with the `-o` and `-t`
values applied.

## Generated schema highlights

- Structs become `type: object` definitions; nested structs are hoisted into
  `#/$defs/<TypeName>` and referenced where needed.
- Maps translate into `additionalProperties` with the schema generated for the
  map value. Slices keep their element type under `items`.
- Pointers compile down to the schema of the pointed type; fields track an
  `x-pointer` marker for tooling but the schema does not yet mark fields as
  required/optional.
- Anonymous embedded structs are added via `allOf` with the referenced `$defs`.
- Struct tags prefer `mapstructure:"..."` for field names; otherwise we fall
  back to the exported Go field name.
- The schema `id` is the prefix (`-p`) plus the Go package path, and the schema
  `title` is set to the package name.

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
- Required/optional sets

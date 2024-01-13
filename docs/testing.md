
## Testing your components

- `scraperint` is a wrapper around testcontainers and the golang testing api for use with receivers only (at least, today).  See docs in [`scraperinttest`](../internal/coreinternal/scraperinttest/README.md) for information on usage.
- Add the `integration` [build constraint](#constraining-build-targets) for longer-running, comprehensive tests.  These will be run via github actions when submitting a PR.

## Constraining build targets

This project uses golang [build-constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints) to tag build targets.  Guidance on using existing targets

- `e2e` should be used for end-to-end tests.  These are currently manually configured to be run in `.github/workflows` on a per-component basis.
- `integration` should be used for integration tests (able to be run on local hardware but not require any credentials or environmental infrastructure).  You can run this with `make integration-test` or `cd componentclass/yourcomponent && make mod-integration-test`
- Restrict builds for varying platforms or processor architectures with their relevant platform names.  See [documentation](https://pkg.go.dev/cmd/go#hdr-Build_constraints) on `GOOS` and `GOOARCH` for how these are set.
- Add other build tags as needed, such as has been done for `tools` and `race`.  Please document any needed commands here and ensure to get consensus with the otel collector working group before introducing build constraints.

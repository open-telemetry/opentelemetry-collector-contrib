Before merge:
- Update CI
  - Skip the following if PR has `Changelog Update` label
    - Require no changes to CHANGELOG.md (except when prepping a release, via `Changelog Update` tag?)
    - Require exactly one new (not modified) `changelog/*.yaml` file
    - Require that `make chlog-validate` passes
    - If any of the above fail, explain where to read about new process

Before release:
- Migrate Unreleased entries to yaml files

Later
- Update release proceess to automatically run this with version parameter


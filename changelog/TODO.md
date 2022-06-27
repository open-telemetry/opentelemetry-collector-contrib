The following additional work should be considered:
- Update CI
  - Require no changes to CHANGELOG.md (except when prepping a release)
  - Require exactly one new `changelog/*.yaml` file
  - Run `make chlog-validate`
  - If any of the above fail, explain where to read about new process
- update/preview should accept a version parameter
- Update release proceess to automatically run this with version parameter
- Provide a make target for developers to easily generate a yaml file. eg
  - `make chlog-entry -i 12345 -t enhancement -c somereceiver -m "Add foo"`
  - `make chlog-entry` copies yaml template to `changelog/<branch-name>.yaml`
- Update `CONTRIBUTING.md` to explain process

Code quality:
- Tests
  - Golden files
- Generation with go template

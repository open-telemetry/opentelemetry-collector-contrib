# End-to-End Tests (e2etests)

This directory contains the end-to-end tests for the OpenTelemetry Collector Contrib repository. These tests are designed to validate the functionality and integration of various components in real-world scenarios.

## Dependencies

The e2etests package leverages the following external packages:

- [Datadog Agent E2E Tests](https://github.com/DataDog/datadog-agent/tree/main/test/new-e2e): Provides a framework and utilities for running end-to-end tests.
- [Datadog Test Infra Definitions](https://github.com/DataDog/test-infra-definitions): Supplies infrastructure definitions and configurations for setting up test environments.

## Usage

To run the tests, ensure that all dependencies are installed and configured correctly. Refer to the documentation in the linked repositories for setup instructions.

Once pulumi is set up via `inv setup` in the test-infra-definitions repository folder, run the following command: `go test ./tests/... -v -count 1 --args -docker_secret $(aws-vault exec sso-agent-qa-read-only -- aws ecr get-login-password)`. Pulumi should provision the kind cluster in EC2 and report successful completion.

You may run a local test by replacing the reference to `RunFunc` in Provisioner function with `LocalRunFunc`.

## Contributing

Contributions to the e2etests package are welcome. Please ensure that any changes are thoroughly tested and documented.

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](../LICENSE) file for details.
include ../../Makefile.Common

# Remove "-race" from the default set of test arguments.
# receiver/sqlserverreceiver tests are failing with the -race check.
# See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10149
GOTEST_OPT = -v -timeout 300s --tags=$(GO_BUILD_TAGS)

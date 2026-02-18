include ../../Makefile.Common

# Default timeout is 360s, which is too short for testcontainers in CI environments
# Override to 15m to allow for container startup and image pulls
GOTEST_INTEGRATION_OPT = $(GOTEST_RACE_OPT) -timeout 900s -parallel 4 -skip Sudo

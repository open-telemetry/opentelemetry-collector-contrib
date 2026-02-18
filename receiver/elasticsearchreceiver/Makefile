include ../../Makefile.Common

# Default timeout is 360s, which is too short for testcontainers in CI environments
# Override to 30m to allow for sequential container startup and image pulls
GOTEST_INTEGRATION_OPT = $(GOTEST_RACE_OPT) -timeout 1800s -parallel 4 -skip Sudo

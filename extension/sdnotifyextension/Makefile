include ../../Makefile.Common

# The shared 360s default (Makefile.Common's GOTEST_INTEGRATION_OPT) is tuned for
# integration tests that spin up a single lightweight testcontainer. This
# extension's integration tests build/boot privileged systemd containers and
# exercise real unit lifecycle transitions (boot, SIGHUP reload, stop), which
# takes noticeably longer, especially on a cold CI runner with no image cache.
GOTEST_INTEGRATION_OPT=$(GOTEST_RACE_OPT) -timeout 900s -parallel 4 -skip Sudo

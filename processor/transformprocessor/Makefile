include ../../Makefile.Common

# benchmark-transform runs all transform processor benchmarks across the module.
# The shared `benchmark` target from Makefile.Common only covers the module's
# root package, missing the per-function benchmarks in the internal packages.
.PHONY: benchmark-transform
benchmark-transform:
	$(GOCMD) test -run='^$$' -bench=. -benchmem ./...

include ../../Makefile.Common

# benchmark-ottl runs all OTTL benchmarks across the module (the root package,
# ottlfuncs, and e2e). The shared `benchmark` target from Makefile.Common only
# covers the module's root package.
.PHONY: benchmark-ottl
benchmark-ottl:
	$(GOCMD) test -run='^$$' -bench=. -benchmem ./...

include ../../Makefile.Common

# benchmark-ottl runs all OTTL benchmarks across the module (the root package,
# ottlfuncs, and e2e). The shared `benchmark` target from Makefile.Common only
# covers the module's root package.
.PHONY: benchmark-ottl
benchmark-ottl:
	$(GOCMD) test -run='^$$' -bench=. -benchmem ./...

# regenerate-function-goldens regenerates the golden lists of standard and
# experimental OTTL functions guarded by TestFrozenStandardFunctions. Changes to
# the frozen (gate-disabled) list require OTTL code owner approval.
.PHONY: regenerate-function-goldens
regenerate-function-goldens:
	OTTL_UPDATE_GOLDEN=1 $(GOCMD) test -run=TestFrozenStandardFunctions ./ottlfuncs/...

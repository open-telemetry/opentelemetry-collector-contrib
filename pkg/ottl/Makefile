include ../../Makefile.Common

# benchmark-ottl runs all OTTL benchmarks across the module (the root package,
# ottlfuncs, and e2e). The shared `benchmark` target from Makefile.Common only
# covers the module's root package.
.PHONY: benchmark-ottl
benchmark-ottl:
	$(GOCMD) test -run='^$$' -bench=. -benchmem ./...

# update-ottl-signatures regenerates the golden files that lock the OTTL
# function signature surface (name, argument order, type, optionality) for both
# the stable and experimental function sets. Run it after an intentional change
# to a function's Arguments struct or experimental status, then commit the diff
# alongside a changelog entry.
.PHONY: update-ottl-signatures
update-ottl-signatures:
	OTTL_UPDATE_SIGNATURES=1 $(GOCMD) test ./ottlfuncs -run 'Test_StableFunctionSignatures|Test_ExperimentalFunctionSignatures'

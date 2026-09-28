include ../../../Makefile.Common

# update-ottl-signatures regenerates the golden file that locks the experimental
# OTTL function signature surface (name, argument order, type, optionality). Run
# it after an intentional change to a function's Arguments struct, then commit
# the diff alongside a changelog entry.
.PHONY: update-ottl-signatures
update-ottl-signatures:
	OTTL_UPDATE_SIGNATURES=1 $(GOCMD) test ./ottlfuncs -run 'Test_ExperimentalFunctionSignatures'

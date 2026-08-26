include ../../Makefile.Common

e2e-test:
	make -C ../../ otelcontribcol
	go test -v $(GOTEST_RACE_OPT) --tags=e2e .

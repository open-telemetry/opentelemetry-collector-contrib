include ../../Makefile.Common

e2e-test:
	make -C ../../ otelcontribcol
	go test -v -race --tags=e2e .

EXPORTERS := $(wildcard exporter/*/.)
RECEIVERS := $(wildcard receiver/*/.)

.DEFAULT_GOAL := all

.PHONY: all $(EXPORTERS) $(RECEIVERS)
all: $(EXPORTERS) $(RECEIVERS)
$(EXPORTERS):
	$(MAKE) -C $@
$(RECEIVERS):
	$(MAKE) -C $@

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
	  golang.org/x/lint/golint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck

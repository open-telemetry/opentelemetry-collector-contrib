EXPORTERS := $(wildcard exporter/*/.)

.DEFAULT_GOAL := all

.PHONY: all $(EXPORTERS)
all: $(EXPORTERS)
$(EXPORTERS):
	$(MAKE) -C $@

.PHONY: install-tools
install-tools:
	GO111MODULE=on go install \
	  github.com/google/addlicense \
	  golang.org/x/lint/golint \
	  golang.org/x/tools/cmd/goimports \
	  github.com/client9/misspell/cmd/misspell \
	  honnef.co/go/tools/cmd/staticcheck

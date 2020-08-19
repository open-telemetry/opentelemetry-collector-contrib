ALL_PKG_WITHOUT_PROTO := $(shell go list ./... | grep -v proto)

.PHONY: lint
lint: lint-static-check
	@golangci-lint run --allow-parallel-runners

.PHONY: lint-static-check
lint-static-check:
	@STATIC_CHECK_OUT=`staticcheck $(ALL_PKG_WITHOUT_PROTO) 2>&1`; \
		if [ "$$STATIC_CHECK_OUT" ]; then \
			echo "$(STATICCHECK) FAILED => static check errors:\n"; \
			echo "$$STATIC_CHECK_OUT\n"; \
			exit 1; \
		else \
			echo "Static check finished successfully"; \
		fi

%: force
	@$(MAKE) -f ../../Makefile.Common $@

force: ;

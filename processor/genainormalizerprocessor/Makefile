include ../../Makefile.Common
include ../../Makefile.Weaver

# The semantic-conventions release the gen_ai.* type table is generated from.
# internal/otelsemconv/otelsemconv.go names the targeted version twice more;
# all three must agree.
SEMCONV_TAG ?= v1.40.0
SEMCONV_IDENT = $(shell echo $(SEMCONV_TAG) | tr '.' '_' | sed 's/^v/V/')

# Writes internal/otelsemconv/types_$(SEMCONV_TAG)_gen.go. Weaver fetches the
# registry from the tagged archive; nothing is vendored. Makefile.Weaver
# supplies the container pin and the docker/podman user handling.
.PHONY: generate-semconv-types
generate-semconv-types:
	$(DOCKER_RUN) --rm \
		$(DOCKER_USER_IS_HOST_USER_ARG) \
		--env HOME=/tmp/weaver \
		--mount 'type=bind,source=$(PWD)/templates,target=/home/weaver/templates,readonly' \
		--mount 'type=bind,source=$(PWD)/,target=/home/weaver/target' \
		${WEAVER_CONTAINER} registry generate \
		--registry=https://github.com/open-telemetry/semantic-conventions/archive/refs/tags/$(SEMCONV_TAG).zip[model] \
		--templates=/home/weaver/templates \
		--param tag=$(shell echo $(SEMCONV_TAG) | tr '.' '_') \
		--param ident=$(SEMCONV_IDENT) \
		semconvtypes \
		/home/weaver/target
	gofmt -w internal/otelsemconv/types_$(shell echo $(SEMCONV_TAG) | tr '.' '_')_gen.go

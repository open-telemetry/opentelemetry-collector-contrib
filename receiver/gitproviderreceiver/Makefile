include ../../Makefile.Common

GENQLIENT_DIRS = $(shell find . -type f -name 'genqlient.yaml' -exec dirname {} \;)

.PHONY: genqlient
genqlient:
	for dir in $(GENQLIENT_DIRS); do \
		cd $$dir && \
		if [ -f genqlient.yaml ] && [ -f genqlient.graphql ] && [ -f schema.graphql ]; then \
			echo "running genqlient in $$dir"; \
			go run github.com/Khan/genqlient@latest; \
		else \
			echo "skipping genqlient in $$dir"; \
		fi; \
	done

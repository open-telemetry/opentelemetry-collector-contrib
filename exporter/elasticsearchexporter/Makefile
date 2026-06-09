include ../../Makefile.Common

.PHONY: golden-update
golden-update: ## Update golden files with current document structure
	@echo "Updating golden files..."
	@go test -tags=integration -run TestDocumentStructureGolden -update-golden -v
	@echo ""
	@echo "✓ Golden files updated successfully!"
	@echo "Please review the changes in testdata/golden/ and commit them."

.PHONY: golden-validate
golden-validate: ## Validate that document structure matches golden files
	@echo "Validating golden files..."
	@go test -tags=integration -run TestDocumentStructureGolden -v

.PHONY: test
test: golden-validate ## Run all tests including golden file validation
	@$(MAKE) gotest

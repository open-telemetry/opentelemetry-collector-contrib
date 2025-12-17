include ../../Makefile.Common

.PHONY: install-deps
install-deps:
	go install go.uber.org/mock/mockgen@latest

.PHONY: mockgen
mockgen:
	mockgen -source=./internal/s3.go -destination=./internal/mock_s3_service.go -package=internal
	mockgen -source=./internal/custom_trigger.go -destination=./internal/mock_custom_trigger.go -package=internal

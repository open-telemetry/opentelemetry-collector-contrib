include ../../Makefile.Common

.PHONY: mockgen
mockgen:
	go tool go.uber.org/mock/mockgen -source=./internal/s3.go -destination=./internal/mock_s3_service.go -package=internal
	go tool go.uber.org/mock/mockgen -source=./internal/custom_trigger.go -destination=./internal/mock_custom_trigger.go -package=internal

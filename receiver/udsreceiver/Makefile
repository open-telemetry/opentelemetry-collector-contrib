BUILD        := $(shell git describe --long --tags)
BUILD_DATE   := $(shell date -u +'%Y-%m-%d %H:%M:%S')

ifeq ($(BUILD),)
	BUILD := "v0.0.0"
endif

.PHONY: test
test:
	go test -v -race -cover ./...

.PHONY: lint
lint:
	golint ./...

.PHONY: vet
vet:
	go vet ./...

build: build-linux build-darwin build-windows

build-linux:
	GO111MODULE=on GOOS=linux   GOARCH=amd64 go build -i -ldflags "-X \"main.buildVersion=${BUILD} (Linux)\"   -X \"main.buildDate=${BUILD_DATE}\"" -o build/oc-daemon-linux ./cmd

build-darwin:
	GO111MODULE=on GOOS=darwin  GOARCH=amd64 go build -i -ldflags "-X \"main.buildVersion=${BUILD} (OS X)\"    -X \"main.buildDate=${BUILD_DATE}\"" -o build/oc-daemon-osx   ./cmd

build-windows:
	GO111MODULE=on GOOS=windows GOARCH=amd64 go build -i -ldflags "-X \"main.buildVersion=${BUILD} (Windows)\" -X \"main.buildDate=${BUILD_DATE}\"" -o build/oc-daemon.exe   ./cmd

docker:
	docker build --rm --no-cache -f Dockerfile -t oc-daemon . 

clean:
	@rm -rf build

.PHONY: all
all: clean vet lint test build

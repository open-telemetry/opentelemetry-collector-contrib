# syntax=docker/dockerfile:1

# Uses the official Go image — no toolchain download needed.
FROM golang:1.24-alpine AS bench

RUN apk add --no-cache git gcc musl-dev

WORKDIR /bench

# Copy only go.mod first so the dependency layer is cached.
COPY go.mod ./
RUN GONOSUMDB='*' GOFLAGS='-mod=mod' go mod download || true
RUN GONOSUMDB='*' go mod tidy || true

# Copy benchmark source.
COPY . .
# Re-tidy after source is present so go.sum is complete.
RUN GONOSUMDB='*' go mod tidy

# Run benchmarks. Results are written to stdout and bench.txt.
CMD GONOSUMDB='*' go test \
        -bench=. \
        -benchtime=10s \
        -benchmem \
        -count=1 \
        -v \
        . 2>&1 | tee /results/bench.txt

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go ./
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o /fakebackend .

FROM scratch
COPY --from=build /fakebackend /fakebackend
USER 10001:10001
EXPOSE 4317 13133
ENTRYPOINT ["/fakebackend"]

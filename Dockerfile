FROM golang:1.22
COPY  ./bin/ /
ENTRYPOINT ["/otelcontribcol_amd64"]
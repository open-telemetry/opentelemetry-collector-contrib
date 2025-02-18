FROM golang:1.22
COPY  ./bin/ .
RUN ls .
CMD ["./otelcontribcol"]

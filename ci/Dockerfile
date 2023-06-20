FROM registry.ddbuild.io/images/mirror/golang:1.20 AS build
WORKDIR /src
ADD . /src
RUN make otelcontribcol

FROM  486234852809.dkr.ecr.us-east-1.amazonaws.com/base:xenial
USER root

RUN clean-apt install make \
            ca-certificates 

COPY --from=build /src/bin/otelcontribcol_* /otelcol-contrib
USER 1001
ENTRYPOINT ["/otelcol-contrib"]
EXPOSE 4317 4318 55680 55679

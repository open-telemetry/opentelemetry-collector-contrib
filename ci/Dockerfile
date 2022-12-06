FROM registry.ddbuild.io/images/mirror/golang:1.19 AS build
WORKDIR /src
ADD . /src
RUN make otelcontribcol

FROM  486234852809.dkr.ecr.us-east-1.amazonaws.com/base:xenial
USER root

RUN clean-apt install make \
            ca-certificates 

COPY --from=build /src/bin/otelcontribcol_* /otelcontribcol
USER 1001
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 4317 55680 55679

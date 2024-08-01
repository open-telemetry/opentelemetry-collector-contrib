#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# This script is used to deploy collector on demo account cluster

set -euo pipefail
IFS=$'\n\t'
set -x

install_collector() {
	# Set the namespace and release name
	release_name="opentelemetry-collector"
	release_name_gateway="opentelemetry-collector-gateway"
	namespace="otel-ds-gateway"

	# if repo already exists, helm 3+ will skip
	helm --debug repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts

	# --install will run `helm install` if not already present.
    # gateway with DD exporter
	helm --debug upgrade "${release_name_gateway}" -n "${namespace}" open-telemetry/opentelemetry-collector --install \
		-f ./ci/values.yaml \
		-f ./ci/values-ds-gateway.yaml \
		--set-string image.tag="otelcolcontrib-v$CI_COMMIT_SHORT_SHA" \
		--set-string image.repository="601427279990.dkr.ecr.us-east-1.amazonaws.com/otel-collector-contrib"

    # daemonset with otlp exporter
	helm --debug upgrade "${release_name}" -n "${namespace}" open-telemetry/opentelemetry-collector --install \
		-f ./ci/values.yaml \
		-f ./ci/values-otlp-col.yaml \
		--set-string image.tag="otelcolcontrib-v$CI_COMMIT_SHORT_SHA" \
		--set-string image.repository="601427279990.dkr.ecr.us-east-1.amazonaws.com/otel-collector-contrib"

}

###########################################################################################################
clusterName="dd-otel"
clusterArn="arn:aws:eks:us-east-1:601427279990:cluster/${clusterName}"

aws eks --region us-east-1 update-kubeconfig --name "${clusterName}"
kubectl config use-context "${clusterArn}"

install_collector

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
  namespace="otel"

  # if repo already exists, helm 3+ will skip
  helm --debug repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts

  # --install will run `helm install` if not already present.
  helm --debug upgrade "${release_name}" -n "${namespace}" open-telemetry/opentelemetry-collector --install \
    -f ./ci/values.yaml \
    --set-string image.tag="otelcolcontrib-v$CI_COMMIT_SHORT_SHA"

}

###########################################################################################################
clusterName="otel-demo"
clusterArn="arn:aws:eks:us-east-1:172597598159:cluster/${clusterName}"

aws eks --region us-east-1 update-kubeconfig --name "${clusterName}"
kubectl config use-context "${clusterArn}"

install_collector

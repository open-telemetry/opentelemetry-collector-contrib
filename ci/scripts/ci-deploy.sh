#!/usr/bin/env bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# This script is used to deploy collector on demo account cluster

set -euo pipefail
IFS=$'\n\t'
set -x

namespace=$NAMESPACE
nodegroup=$NODE_GROUP
mode=$MODE
replicaCount=$REPLICA_COUNT
clusterRole=$CLUSTER_ROLE
clusterName=$CLUSTER_NAME
clusterArn=$CLUSTER_ARN
registry=$REGISTRY

install_collector() {
	release_name="opentelemetry-collector"

	# Add open-telemetry helm repo (if repo already exists, helm 3+ will skip)
	helm --debug repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	helm repo update open-telemetry

    helm_cmd="helm --debug upgrade ${release_name} -n ${namespace} open-telemetry/opentelemetry-collector --install \
        -f ./ci/values.yaml \
        --set-string image.repository=${registry} \
		--set-string image.tag=otelcolcontrib-v$CI_COMMIT_SHORT_SHA \
        --set clusterRole.name=${clusterRole} \
        --set clusterRole.clusterRoleBinding.name=${clusterRole} \
        --set mode=${mode} \
        --set replicaCount=${replicaCount}"

    if [ -n "$nodegroup" ]; then
        helm_cmd+=" --set nodeSelector.\"alpha\\.eksctl\\.io/nodegroup-name\"=${nodegroup}"
    fi

	eval $helm_cmd

	# only deploy otlp col for otel-ds-gateway
	if [ "$namespace" == "otel-ds-gateway" ]; then
		install_ds_otlp
	fi
}

install_ds_otlp() {
	release_name="opentelemetry-collector-ds"

	# daemonset with otlp exporter
	helm --debug upgrade "${release_name}" -n "${namespace}" open-telemetry/opentelemetry-collector --install \
		-f ./ci/values.yaml \
		-f ./ci/values-otlp-col.yaml \
		--set-string image.tag="otelcolcontrib-v$CI_COMMIT_SHORT_SHA" \
		--set-string image.repository=${registry}
}

###########################################################################################################
clusterName="${clusterName}"
clusterArn="${clusterArn}"

aws eks --region us-east-1 update-kubeconfig --name "${clusterName}"
kubectl config use-context "${clusterArn}"

install_collector
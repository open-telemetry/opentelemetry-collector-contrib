#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

FPM_DIR="$( cd "$( dirname ${BASH_SOURCE[0]} )" && pwd )"

PKG_NAME="otel-contrib-collector"
PKG_VENDOR="OpenTelemetry Community"
PKG_MAINTAINER="OpenTelemetry Community <cncf-opentelemetry-community@lists.cncf.io>"
PKG_DESCRIPTION="OpenTelemetry Contrib Collector"
PKG_LICENSE="Apache 2.0"
PKG_URL="https://github.com/open-telemetry/opentelemetry-collector-contrib"
PKG_USER="otel"
PKG_GROUP="otel"

SERVICE_NAME="otel-contrib-collector"
PROCESS_NAME="otelcontribcol"

CONFIG_PATH="$REPO_DIR/examples/demo/otel-collector-config.yaml"
SERVICE_PATH="$FPM_DIR/$SERVICE_NAME.service"
ENVFILE_PATH="$FPM_DIR/$SERVICE_NAME.conf"
PREINSTALL_PATH="$FPM_DIR/preinstall.sh"
POSTINSTALL_PATH="$FPM_DIR/postinstall.sh"
PREUNINSTALL_PATH="$FPM_DIR/preuninstall.sh"

docker_cp() {
    local container="$1"
    local src="$2"
    local dest="$3"
    local dest_dir="$( dirname "$dest" )"

    echo "Copying $src to $container:$dest ..."
    podman exec $container mkdir -p "$dest_dir"
    podman cp "$src" $container:"$dest"
}

install_pkg() {
    local container="$1"
    local pkg_path="$2"
    local pkg_base=$( basename "$pkg_path" )

    echo "Installing $pkg_base ..."
    docker_cp $container "$pkg_path" /tmp/$pkg_base
    if [[ "${pkg_base##*.}" = "deb" ]]; then
        podman exec $container dpkg -i /tmp/$pkg_base
    else
        podman exec $container rpm -ivh /tmp/$pkg_base
    fi
}

uninstall_pkg() {
    local container="$1"
    local pkg_type="$2"
    local pkg_name="${3:-"$PKG_NAME"}"

    echo "Uninstalling $pkg_name ..."
    if [[ "$pkg_type" = "deb" ]]; then
        podman exec $container dpkg -r $pkg_name
    else
        podman exec $container rpm -e $pkg_name
    fi
}

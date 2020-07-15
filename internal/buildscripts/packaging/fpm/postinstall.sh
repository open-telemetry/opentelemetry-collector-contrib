#!/bin/sh

if command -v systemctl >/dev/null 2>&1; then
    systemctl enable otel-contrib-collector.service
    if [ -f /etc/otel-contrib-collector/config.yaml ]; then
        systemctl start otel-contrib-collector.service
    fi
fi
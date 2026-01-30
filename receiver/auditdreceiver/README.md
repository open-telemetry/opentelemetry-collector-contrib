Reads [auditd](https://www.man7.org/linux/man-pages/man8/auditd.8.html) events and converts them to logs.

## Configuration

| Field | Default | Description |
|---|---|---|
| `rules` | required | A list of [auditd rules](https://www.man7.org/linux/man-pages/man7/audit.rules.7.html) |


## Description
This receiver will take a set of auditd rules and capture the auditd events as the happen. These events are converted into OTLP logs and sent to the consumer (downstream components).

## Important Prerequisites:

**The auditdreceiver requires privileged access to the host system, use caution and run at your own risk!**

The auditdreceiver will take on the role of the auditd process on the host machine. This means that the following prerequisites need to be fixed before the receiver runs:

- The OtelTelemetryCollector needs to run with `spec.hostPID: true` for any of this to work.
- Various privileged access needs to be configured `spec.securityContext`:
  ```yaml
  securityContext:
    capabilities:
      add:
      - AUDIT_READ
      - AUDIT_WRITE
      - AUDIT_CONTROL
    privileged: true
    runAsUser: 0
  ```
- An init container that executes a necessary shutdown sequence of the auditd process needs to run before. Configured to access the host system through a mounting of volumes.

## Example configuration
```
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: example
  namespace: example
spec:
  mode: daemonset
  securityContext:
    capabilities:
      add:
      - AUDIT_READ
      - AUDIT_WRITE
      - AUDIT_CONTROL
    privileged: true
    runAsUser: 0
  hostPID: true
  image: <add image here>
  ingress:
    route: {}
  initContainers:
  - args:
    - |-
      chroot /host systemctl status auditd
      chroot /host systemctl stop auditd
      chroot /host systemctl disable auditd
      chroot /host systemctl status auditd
      echo "done"
    command:
    - sh
    - -c
    env:
    - name: NODE
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    image: alpine:latest
    name: init
    resources: {}
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /host
      name: host
  volumeMounts:
  - mountPath: /host
    name: host
    readOnly: true
  volumes:
  - hostPath:
      path: /
    name: host
  config:
    receivers:
      auditd/auditd_logs:
        rules:
        - -a always,exit -F path=/host/etc/group -F perm=wa
        - -a always,exit -F path=/host/etc/passwd -F perm=wa
```

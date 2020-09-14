{{- define "opentelemetry-collector.pod" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
serviceAccountName: {{ include "opentelemetry-collector.serviceAccountName" . }}
securityContext:
  {{- toYaml .Values.podSecurityContext | nindent 2 }}
containers:
  - name: {{ .Chart.Name }}
    command:
      - /{{ .Values.command.name }}
      - --config=/conf/relay.yaml
      - --metrics-addr=0.0.0.0:8888
      - --mem-ballast-size-mib={{ template "opentelemetry-collector.getMemBallastSizeMib" .Values.resources.limits.memory }}
      {{- range .Values.command.extraArgs }}
      - {{ . }}
      {{- end }}
    securityContext:
      {{- toYaml .Values.securityContext | nindent 6 }}
    image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
    imagePullPolicy: {{ .Values.image.pullPolicy }}
    ports:
      {{- range $key, $port := .Values.ports }}
      - name: {{ $key }}
        containerPort: {{ $port.containerPort }}
        protocol: {{ $port.protocol }}
        {{- if and $.isAgent $port.hostPort }}
        hostPort: {{ $port.hostPort }}
        {{- end }}
      {{- end }}
    env:
      - name: MY_POD_IP
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: status.podIP
      {{- if and .isAgent .Values.telemetry.metrics.enabled }}
      # Env variables for host metrics receiver
      - name: HOST_PROC
        value: /hostfs/proc
      - name: HOST_SYS
        value: /hostfs/sys
      - name: HOST_ETC
        value: /hostfs/etc
      - name: HOST_VAR
        value: /hostfs/var
      - name: HOST_RUN
        value: /hostfs/run
      - name: HOST_DEV
        value: /hostfs/dev
      {{- end }}
    livenessProbe:
      httpGet:
        path: /
        port: 13133
    readinessProbe:
      httpGet:
        path: /
        port: 13133
    resources:
      {{- toYaml .Values.resources | nindent 6 }}
    volumeMounts:
      - mountPath: /conf
        name: {{ .Chart.Name }}-configmap
      {{- if and .isAgent .Values.telemetry.metrics.enabled }}
      - mountPath: /hostfs
        name: hostfs
        readOnly: true
        mountPropagation: HostToContainer
      {{- end }}
volumes:
  - name: {{ .Chart.Name }}-configmap
    configMap:
      name: {{ include "opentelemetry-collector.fullname" . }}{{ .configmapSuffix }}
      items:
        - key: relay
          path: relay.yaml
  {{- if and .isAgent .Values.telemetry.metrics.enabled }}
  - name: hostfs
    hostPath:
      path: /
  {{- end }}
{{- with .Values.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

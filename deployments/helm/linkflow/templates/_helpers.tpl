{{/*
Expand the name of the chart.
*/}}
{{- define "linkflow.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "linkflow.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s" $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "linkflow.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: linkflow
{{- end }}

{{/*
Selector labels
*/}}
{{- define "linkflow.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service template
*/}}
{{- define "linkflow.service" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .name }}-service
  labels:
    app: {{ .name }}-service
    {{- include "linkflow.labels" .context | nindent 4 }}
spec:
  replicas: {{ .replicas | default 2 }}
  selector:
    matchLabels:
      app: {{ .name }}-service
  template:
    metadata:
      labels:
        app: {{ .name }}-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      containers:
      - name: {{ .name }}-service
        image: "{{ .context.Values.common.image.registry }}/{{ .name }}-service:{{ .context.Values.common.image.tag }}"
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: linkflow-config
        - secretRef:
            name: linkflow-secrets
        resources:
          {{- toYaml .resources | nindent 10 }}
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .name }}-service
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: {{ .name }}-service
{{- end }}

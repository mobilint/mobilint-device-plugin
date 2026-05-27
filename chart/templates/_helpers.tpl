{{/* Expand the name of the chart. */}}
{{- define "mobilint-device-plugin.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully qualified app name. */}}
{{- define "mobilint-device-plugin.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "mobilint-device-plugin.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mobilint-device-plugin.labels" -}}
helm.sh/chart: {{ include "mobilint-device-plugin.chart" . }}
{{ include "mobilint-device-plugin.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: device-plugin
app.kubernetes.io/part-of: mobilint-platform
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "mobilint-device-plugin.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mobilint-device-plugin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

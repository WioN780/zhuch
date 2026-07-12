{{- define "zhuch.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "zhuch.fullname" -}}
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

{{- define "zhuch.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "zhuch.labels" -}}
helm.sh/chart: {{ include "zhuch.chart" . }}
{{ include "zhuch.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "zhuch.selectorLabels" -}}
app.kubernetes.io/name: {{ include "zhuch.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "zhuch.arena.fullname" -}}
{{ include "zhuch.fullname" . }}-arena
{{- end -}}

{{- define "zhuch.server.fullname" -}}
{{ include "zhuch.fullname" . }}-server
{{- end -}}

{{- define "zhuch.mlflow.fullname" -}}
{{ include "zhuch.fullname" . }}-mlflow
{{- end -}}

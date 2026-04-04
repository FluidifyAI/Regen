{{- /*
deploy/helm/fluidify-regen/templates/_helpers.tpl
*/ -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "fluidify-regen.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "fluidify-regen.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart label.
*/}}
{{- define "fluidify-regen.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "fluidify-regen.labels" -}}
helm.sh/chart: {{ include "fluidify-regen.chart" . }}
{{ include "fluidify-regen.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "fluidify-regen.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fluidify-regen.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "fluidify-regen.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "fluidify-regen.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Compute DATABASE_URL.
If secrets.databaseURL is set, use it directly.
Otherwise build from bundled postgresql subchart credentials.
*/}}
{{- define "fluidify-regen.databaseURL" -}}
{{- if .Values.secrets.databaseURL }}
{{- .Values.secrets.databaseURL }}
{{- else }}
{{- printf "postgresql://%s:%s@%s-postgresql:5432/%s?sslmode=disable"
    .Values.postgresql.auth.username
    .Values.postgresql.auth.password
    .Release.Name
    .Values.postgresql.auth.database }}
{{- end }}
{{- end }}

{{/*
Compute REDIS_URL.
If secrets.redisURL is set, use it directly.
Otherwise build from bundled redis subchart.
*/}}
{{- define "fluidify-regen.redisURL" -}}
{{- if .Values.secrets.redisURL }}
{{- .Values.secrets.redisURL }}
{{- else }}
{{- printf "redis://%s-redis-master:6379" .Release.Name }}
{{- end }}
{{- end }}

{{/*
Container image reference.
*/}}
{{- define "fluidify-regen.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

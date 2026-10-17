{{/*
Copyright 2025 Tencent
SPDX-License-Identifier: MIT

AI KB Helm Chart Template Helpers

Best Practices References:
- https://helm.sh/docs/chart_best_practices/templates/
- https://github.com/argoproj/argo-helm/blob/main/charts/argo-cd/templates/_helpers.tpl
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "ai-kb.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "ai-kb.fullname" -}}
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
Resolve a component Service name while preserving names used by existing releases.
Usage: include "ai-kb.serviceName" (dict "component" "postgresql" "context" .)
*/}}
{{- define "ai-kb.serviceName" -}}
{{- $service := (index .context.Values .component).service | default dict -}}
{{- $legacyName := .component -}}
{{- if eq .component "postgresql" -}}
{{- $legacyName = "postgres" -}}
{{- end -}}
{{- default $legacyName $service.name -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
Ref: https://helm.sh/docs/chart_best_practices/labels/
*/}}
{{- define "ai-kb.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels following Kubernetes recommended labels.
Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
*/}}
{{- define "ai-kb.labels" -}}
helm.sh/chart: {{ include "ai-kb.chart" . }}
{{ include "ai-kb.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: aikb
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ai-kb.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai-kb.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Component labels - use for individual components
Usage: {{ include "ai-kb.componentLabels" (dict "component" "app" "context" .) }}
*/}}
{{- define "ai-kb.componentLabels" -}}
{{ include "ai-kb.labels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Component selector labels
Usage: {{ include "ai-kb.componentSelectorLabels" (dict "component" "app" "context" .) }}
*/}}
{{- define "ai-kb.componentSelectorLabels" -}}
{{ include "ai-kb.selectorLabels" .context }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Create the name of the service account to use.
Ref: https://helm.sh/docs/chart_best_practices/rbac/
*/}}
{{- define "ai-kb.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ai-kb.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name - supports existing secret
*/}}
{{- define "ai-kb.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "ai-kb.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
Return the app image with tag.
Defaults to Chart.appVersion if tag is not specified.
*/}}
{{- define "ai-kb.app.image" -}}
{{- $tag := default .Chart.AppVersion .Values.app.image.tag }}
{{- printf "%s:%s" .Values.app.image.repository $tag }}
{{- end }}

{{/*
Return the frontend image with tag.
Defaults to Chart.appVersion if tag is not specified.
*/}}
{{- define "ai-kb.frontend.image" -}}
{{- $tag := default .Chart.AppVersion .Values.frontend.image.tag }}
{{- printf "%s:%s" .Values.frontend.image.repository $tag }}
{{- end }}

{{/*
Return the website image with tag.
Defaults to Chart.appVersion if tag is not specified.
*/}}
{{- define "ai-kb.website.image" -}}
{{- $tag := default .Chart.AppVersion .Values.website.image.tag }}
{{- printf "%s:%s" .Values.website.image.repository $tag }}
{{- end }}

{{/*
Return the docreader image with tag.
Defaults to Chart.appVersion if tag is not specified.
*/}}
{{- define "ai-kb.docreader.image" -}}
{{- $tag := default .Chart.AppVersion .Values.docreader.image.tag }}
{{- printf "%s:%s" .Values.docreader.image.repository $tag }}
{{- end }}

{{/*
Return the PostgreSQL image with tag.
*/}}
{{- define "ai-kb.postgresql.image" -}}
{{- printf "%s:%s" .Values.postgresql.image.repository .Values.postgresql.image.tag }}
{{- end }}

{{/*
Return the Redis image with tag.
*/}}
{{- define "ai-kb.redis.image" -}}
{{- printf "%s:%s" .Values.redis.image.repository .Values.redis.image.tag }}
{{- end }}

{{/*
Return the Neo4j image with tag.
*/}}
{{- define "aikb.neo4j.image" -}}
{{- printf "%s:%s" .Values.neo4j.image.repository .Values.neo4j.image.tag }}
{{- end }}

{{/*
Create image pull secrets list.
*/}}
{{- define "ai-kb.imagePullSecrets" -}}
{{- with .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Return the storage class name.
*/}}
{{- define "ai-kb.storageClass" -}}
{{- if .Values.global.storageClass }}
{{- if eq .Values.global.storageClass "-" }}
storageClassName: ""
{{- else }}
storageClassName: {{ .Values.global.storageClass | quote }}
{{- end }}
{{- end }}
{{- end }}

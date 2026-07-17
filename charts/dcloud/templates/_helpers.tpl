{{- define "dcloud.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "dcloud.fullname" -}}
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

{{- define "dcloud.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "dcloud.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "dcloud.image" -}}
{{- printf "%s/%s/%s:%s" .root.Values.image.registry .root.Values.image.repositoryOwner .repository .root.Values.image.tag -}}
{{- end -}}

{{- /* JWKS URL every backend service uses to verify identity-issued JWTs. */ -}}
{{- define "dcloud.jwksURL" -}}
{{- printf "http://%s-identity:%v/.well-known/jwks.json" (include "dcloud.fullname" .) .Values.identity.service.httpPort -}}
{{- end -}}

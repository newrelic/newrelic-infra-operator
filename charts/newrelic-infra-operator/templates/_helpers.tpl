{{/* vim: set filetype=mustache: */}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "newrelic-infra-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Return the licenseKey
*/}}
{{- define "newrelic-infra-operator.licenseKey" -}}
{{- if .Values.global}}
  {{- if .Values.global.licenseKey }}
      {{- .Values.global.licenseKey -}}
  {{- else -}}
      {{- .Values.licenseKey | default "" -}}
  {{- end -}}
{{- else -}}
    {{- .Values.licenseKey | default "" -}}
{{- end -}}
{{- end -}}

{{/*
Renders a value that contains template.
Usage:
{{ include "tplvalues.render" ( dict "value" .Values.path.to.the.Value "context" $) }}
*/}}
{{- define "tplvalues.render" -}}
    {{- if typeIs "string" .value }}
        {{- tpl .value .context }}
    {{- else }}
        {{- tpl (.value | toYaml) .context }}
    {{- end }}
{{- end -}}

{{- /*
Naming helpers
*/ -}}

{{- define "newrelic-infra-operator.name.admission" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.name" .) "suffix" "admission") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.admission" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "admission") }}
{{- end -}}

{{- define "newrelic-infra-operator.name.admission-create" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.name" .) "suffix" "admission-create") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.admission-create" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "admission-create") }}
{{- end -}}

{{- define "newrelic-infra-operator.name.admission-patch" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.name" .) "suffix" "admission-patch") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.admission-patch" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "admission-patch") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.self-signed-issuer" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "self-signed-issuer") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.root-cert" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "root-cert") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.root-issuer" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "root-issuer") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.webhook-cert" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "webhook-cert") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.infra-agent" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "infra-agent") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.config" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "config") }}
{{- end -}}

{{- define "newrelic-infra-operator.fullname.license" -}}
{{ include "newrelic.common.naming.truncateToDNSWithSuffix" (dict "name" (include "newrelic.common.naming.fullname" .) "suffix" "license") }}
{{- end -}}

{{/*
Return the customSecretName
*/}}
{{- define "newrelic-infra-operator.customSecretName" -}}
{{- if .Values.global }}
  {{- if .Values.global.customSecretName }}
      {{- .Values.global.customSecretName -}}
  {{- else -}}
      {{- .Values.customSecretName | default "" -}}
  {{- end -}}
{{- else -}}
    {{- .Values.customSecretName | default "" -}}
{{- end -}}
{{- end -}}
{{/*
Return the customSecretLicenseKey
*/}}
{{- define "newrelic-infra-operator.customSecretLicenseKey" -}}
{{- if .Values.global }}
  {{- if .Values.global.customSecretLicenseKey }}
      {{- .Values.global.customSecretLicenseKey -}}
  {{- else -}}
      {{- .Values.customSecretLicenseKey | default "" -}}
  {{- end -}}
{{- else -}}
    {{- .Values.customSecretLicenseKey | default "" -}}
{{- end -}}
{{- end -}}

{{/*
Returns fargate
*/}}
{{- define "newrelic.fargate" -}}
{{- if .Values.global }}
  {{- if .Values.global.fargate }}
    {{- .Values.global.fargate -}}
  {{- end -}}
{{- else if .Values.fargate }}
  {{- .Values.fargate -}}
{{- end -}}
{{- end -}}

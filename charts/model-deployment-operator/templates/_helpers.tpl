{{- define "model-deployment-operator.name" -}}
model-deployment-operator
{{- end -}}

{{- define "model-deployment-operator.fullname" -}}
{{ include "model-deployment-operator.name" . }}
{{- end -}}

{{- define "model-deployment-operator.labels" -}}
app.kubernetes.io/name: {{ include "model-deployment-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "tool-node.name" -}}
{{- .Chart.Name -}}
{{- end -}}

{{- define "tool-node.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

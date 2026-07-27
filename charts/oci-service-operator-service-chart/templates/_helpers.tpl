{{- define "osok.serviceAccountName" -}}
{{- default "__OSOK_MANAGER_NAME__" .Values.serviceAccount.name -}}
{{- end -}}

{{- define "osok.controllerImage" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

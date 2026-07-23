{{- define "osok-psql.serviceAccountName" -}}
{{- default "oci-service-operator-psql-controller-manager" .Values.serviceAccount.name -}}
{{- end -}}

{{- define "osok-psql.controllerImage" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}
{{- end -}}

{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "mongodb.fullname" -}}
{{- printf "%s-%s" .Release.Name "mongodb" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "fdbserver.fullname" -}}
{{- printf "%s-%s" .Release.Name "fdbserver" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Render image reference
*/}}
{{- define "monocular.image" -}}
{{ .registry }}/{{ .repository }}:{{ .tag }}
{{- end -}}

{{/*
Sync job pod template
*/}}
{{- define "monocular.sync.podTemplate" -}}
{{- $repo := index . 0 -}}
{{- $global := index . 1 -}}
metadata:
  labels:
    monocular.helm.sh/repo-name: {{ $repo.name }}
    app: {{ template "fullname" $global }}
    release: "{{ $global.Release.Name }}"
spec:
  restartPolicy: OnFailure
  containers:
  - name: sync
    image: {{ template "monocular.image" $global.Values.sync.image }}
    args:
    - sync
    - --debug={{ default false $global.Values.dbDebug }}
    - --user-agent-comment=monocular/{{ $global.Chart.AppVersion }}
    {{- if and $global.Values.global.mongoUrl (and (not $global.Values.mongodb.enabled) (not $global.Values.fdbserver.enabled))}}
    - --mongo-url={{ $global.Values.global.mongoUrl }}
    - --db-type=mongodb
    {{- else if $global.Values.fdbserver.enabled}}
    - --doclayer-url=mongodb://{{ template "fullname" $global }}-fdbdoclayer:{{ $global.Values.fdbdoclayer.service.port }}
    - --db-type=fdb
    {{- else }}
    - --mongo-url={{ template "mongodb.fullname" $global }}
    - --mongo-user=root
    - --db-type=mongodb
    {{- end }}
    - {{ $repo.name }}
    - {{ $repo.url }}
    command:
    - /chart-repo
    {{- if or $global.Values.mongodb.enabled $global.Values.fdbserver.enabled}}
    env:
    - name: HTTP_PROXY
      value: {{ $global.Values.sync.httpProxy }}
    - name: HTTPS_PROXY
      value: {{ $global.Values.sync.httpsProxy }}
    {{- end }}
    resources:
{{ toYaml $global.Values.sync.resources | indent 6 }}
{{- with $global.Values.sync.nodeSelector }}
  nodeSelector:
{{ toYaml . | indent 4 }}
{{- end }}
{{- with $global.Values.sync.affinity }}
  affinity:
{{ toYaml . | indent 4 }}
{{- end }}
{{- with $global.Values.sync.tolerations }}
  tolerations:
{{ toYaml . | indent 4 }}
{{- end }}
{{- end -}}

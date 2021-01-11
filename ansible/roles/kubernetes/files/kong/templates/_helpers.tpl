{{/* vim: set filetype=mustache: */}}
{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}

{{- define "kong.namespace" -}}
{{- default .Release.Namespace .Values.namespace -}}
{{- end -}}

{{- define "kong.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kong.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kong.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kong.metaLabels" -}}
app.kubernetes.io/name: {{ template "kong.name" . }}
helm.sh/chart: {{ template "kong.chart" . }}
app.kubernetes.io/instance: "{{ .Release.Name }}"
app.kubernetes.io/managed-by: "{{ .Release.Service }}"
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{- define "kong.selectorLabels" -}}
app.kubernetes.io/name: {{ template "kong.name" . }}
app.kubernetes.io/component: app
app.kubernetes.io/instance: "{{ .Release.Name }}"
{{- end -}}

{{- define "kong.postgresql.fullname" -}}
{{- $name := default "postgresql" .Values.postgresql.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kong.dblessConfig.fullname" -}}
{{- $name := default "kong-custom-dbless-config" .Values.dblessConfig.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "kong.serviceAccountName" -}}
{{- if .Values.ingressController.serviceAccount.create -}}
    {{ default (include "kong.fullname" .) .Values.ingressController.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.ingressController.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Create KONG_SERVICE_LISTEN strings
Generic tool for creating KONG_PROXY_LISTEN, KONG_ADMIN_LISTEN, etc.
*/}}
{{- define "kong.listen" -}}
  {{- $unifiedListen := list -}}

  {{/* Some services do not support these blocks at all, so these checks are a
       two-stage "is it safe to evaluate this?" and then "should we evaluate
       this?"
  */}}
  {{- if .http -}}
    {{- if .http.enabled -}}
      {{- $listenConfig := dict -}}
      {{- $listenConfig := merge $listenConfig .http -}}
      {{- $_ := set $listenConfig "address" (default "0.0.0.0" .address) -}}
      {{- $httpListen := (include "kong.singleListen" $listenConfig) -}}
      {{- $unifiedListen = append $unifiedListen $httpListen -}}
    {{- end -}}
  {{- end -}}

  {{- if .tls -}}
    {{- if .tls.enabled -}}
      {{/*
      This is a bit of a hack to support always including "ssl" in the parameter
      list for TLS listens. It's not possible to set a variable to an object from
      .Values and then modify one of the objects values locally, although
      https://github.com/helm/helm/issues/4987 indicates it should be. Instead,
      this creates a new object and new parameters list built from the original.
      */}}
      {{- $listenConfig := dict -}}
      {{- $listenConfig := merge $listenConfig .tls -}}
      {{- $parameters := append .tls.parameters "ssl" -}}
      {{- $_ := set $listenConfig "parameters" $parameters -}}
      {{- $_ := set $listenConfig "address" (default "0.0.0.0" .address) -}}
      {{- $tlsListen := (include "kong.singleListen" $listenConfig) -}}
      {{- $unifiedListen = append $unifiedListen $tlsListen -}}
    {{- end -}}
  {{- end -}}

  {{- $listenString := ($unifiedListen | join ", ") -}}
  {{- if eq (len $listenString) 0 -}}
    {{- $listenString = "off" -}}
  {{- end -}}
  {{- $listenString -}}
{{- end -}}

{{/*
Create KONG_PORT_MAPS string
Parameters: takes a service (e.g. .Values.proxy) as its argument and returns KONG_PORT_MAPS for that service.
*/}}
{{- define "kong.port_maps" -}}
  {{- $portMaps := list -}}

  {{- if .http.enabled -}}
		{{- $portMaps = append $portMaps (printf "%d:%d" (int64 .http.servicePort) (int64 .http.containerPort)) -}}
  {{- end -}}

  {{- if .tls.enabled -}}
		{{- $portMaps = append $portMaps (printf "%d:%d" (int64 .tls.servicePort) (int64 .tls.containerPort)) -}}
  {{- end -}}

  {{- $portMapsString := ($portMaps | join ", ") -}}
  {{- $portMapsString -}}
{{- end -}}

{{/*
Create KONG_STREAM_LISTEN string
*/}}
{{- define "kong.streamListen" -}}
  {{- $unifiedListen := list -}}
  {{- range .stream -}}
    {{- $listenConfig := dict -}}
    {{- $listenConfig := merge $listenConfig . -}}
    {{- $_ := set $listenConfig "address" "0.0.0.0" -}}
    {{- $unifiedListen = append $unifiedListen (include "kong.singleListen" $listenConfig ) -}}
  {{- end -}}

  {{- $listenString := ($unifiedListen | join ", ") -}}
  {{- if eq (len $listenString) 0 -}}
    {{- $listenString = "off" -}}
  {{- end -}}
  {{- $listenString -}}
{{- end -}}

{{/*
Create a single listen (IP+port+parameter combo)
*/}}
{{- define "kong.singleListen" -}}
  {{- $listen := list -}}
  {{- $listen = append $listen (printf "%s:%d" .address (int64 .containerPort)) -}}
  {{- range $param := .parameters | default (list) | uniq }}
    {{- $listen = append $listen $param -}}
  {{- end -}}
  {{- $listen | join " " -}}
{{- end -}}

{{/*
Return the local admin API URL, preferring HTTPS if available
*/}}
{{- define "kong.adminLocalURL" -}}
  {{- if .Values.admin.containerPort -}} {{/* TODO: Remove legacy admin behavior */}}
    {{- if .Values.admin.useTLS -}}
https://localhost:{{ .Values.admin.containerPort }}
    {{- else -}}
http://localhost:{{ .Values.admin.containerPort }}
    {{- end -}}
  {{- else -}}
    {{- if .Values.admin.tls.enabled -}}
https://localhost:{{ .Values.admin.tls.containerPort }}
    {{- else if .Values.admin.http.enabled -}}
http://localhost:{{ .Values.admin.http.containerPort }}
    {{- else -}}
http://localhost:9999 # You have no admin listens! The controller will not work unless you set .Values.admin.http.enabled=true or .Values.admin.tls.enabled=true!
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
Create the ingress servicePort value string
*/}}

{{- define "kong.ingress.servicePort" -}}
{{- if .tls.enabled -}}
   {{ .tls.servicePort }}
{{- else -}}
   {{ .http.servicePort }}
{{- end -}}
{{- end -}}

{{/*
Generate an appropriate external URL from a Kong service's ingress configuration
Strips trailing slashes from the path. Manager at least does not handle these
intelligently and will append its own slash regardless, and the admin API cannot handle
the extra slash.
*/}}

{{- define "kong.ingress.serviceUrl" -}}
{{- if .tls -}}
    https://{{ .hostname }}{{ .path | trimSuffix "/" }}
{{- else -}}
    http://{{ .hostname }}{{ .path | trimSuffix "/" }}
{{- end -}}
{{- end -}}

{{/*
The name of the service used for the ingress controller's validation webhook
*/}}

{{- define "kong.service.validationWebhook" -}}
{{ include "kong.fullname" . }}-validation-webhook
{{- end -}}

{{- define "kong.ingressController.env" -}}
{{/*
    ====== AUTO-GENERATED ENVIRONMENT VARIABLES ======
*/}}

{{- $autoEnv := dict -}}
{{- $_ := set $autoEnv "CONTROLLER_KONG_ADMIN_TLS_SKIP_VERIFY" true -}}
{{- $_ := set $autoEnv "CONTROLLER_PUBLISH_SERVICE" (printf "%s/%s-proxy" ( include "kong.namespace" . ) (include "kong.fullname" .)) -}}
{{- $_ := set $autoEnv "CONTROLLER_INGRESS_CLASS" .Values.ingressController.ingressClass -}}
{{- $_ := set $autoEnv "CONTROLLER_ELECTION_ID" (printf "kong-ingress-controller-leader-%s" .Values.ingressController.ingressClass) -}}
{{- $_ := set $autoEnv "CONTROLLER_KONG_ADMIN_URL" (include "kong.adminLocalURL" .) -}}
{{- if .Values.ingressController.admissionWebhook.enabled }}
  {{- $_ := set $autoEnv "CONTROLLER_ADMISSION_WEBHOOK_LISTEN" (printf "0.0.0.0:%d" (int64 .Values.ingressController.admissionWebhook.port)) -}}
{{- end }}

{{/*
    ====== USER-SET ENVIRONMENT VARIABLES ======
*/}}

{{- $userEnv := dict -}}
{{- range $key, $val := .Values.ingressController.env }}
  {{- $upper := upper $key -}}
  {{- $var := printf "CONTROLLER_%s" $upper -}}
  {{- $_ := set $userEnv $var $val -}}
{{- end -}}

{{/*
      ====== MERGE AND RENDER ENV BLOCK ======
*/}}

{{- $completeEnv := mergeOverwrite $autoEnv $userEnv -}}
{{- template "kong.renderEnv" $completeEnv -}}

{{- end -}}

{{- define "kong.volumes" -}}
- name: {{ template "kong.fullname" . }}-prefix-dir
  emptyDir: {}
- name: {{ template "kong.fullname" . }}-tmp
  emptyDir: {}
- name: {{ template "kong.fullname" . }}-bash-wait-for-postgres
  configMap:
    name: {{ template "kong.fullname" . }}-bash-wait-for-postgres
    defaultMode: 0755
{{- range .Values.plugins.configMaps }}
- name: kong-plugin-{{ .pluginName }}
  configMap:
    name: {{ .name }}
{{- range .subdirectories }}
- name: {{ .name }}
  configMap:
    name: {{ .name }}
{{- end }}
{{- end }}
{{- range .Values.plugins.secrets }}
- name: kong-plugin-{{ .pluginName }}
  secret:
    secretName: {{ .name }}
{{- range .subdirectories }}
- name: {{ .name }}
  secret:
    secretName: {{ .name }}
{{- end }}
{{- end }}
{{- if .Values.deployment.kong.enabled }}
- name: custom-nginx-template-volume
  configMap:
    name: {{ template "kong.fullname" . }}-default-custom-server-blocks
{{- end }}
{{- if (and (not .Values.ingressController.enabled) (eq .Values.env.database "off")) }}
- name: kong-custom-dbless-config-volume
  configMap:
    {{- if .Values.dblessConfig.configMap }}
    name: {{ .Values.dblessConfig.configMap }}
    {{- else }}
    name: {{ template "kong.dblessConfig.fullname" . }}
    {{- end }}
{{- end }}
{{- if .Values.ingressController.admissionWebhook.enabled }}
- name: webhook-cert
  secret:
    secretName: {{ template "kong.fullname" . }}-validation-webhook-keypair
{{- end }}
{{- range $secretVolume := .Values.secretVolumes }}
- name: {{ . }}
  secret:
    secretName: {{ . }}
{{- end }}
{{- range .Values.extraConfigMaps }}
- name: {{ .name }}
  configMap:
    name: {{ .name }}
{{- end }}
{{- range .Values.extraSecrets }}
- name: {{ .name }}
  secret:
    secretName: {{ .name }}
{{- end }}
{{- end -}}

{{- define "kong.volumeMounts" -}}
- name: {{ template "kong.fullname" . }}-prefix-dir
  mountPath: /kong_prefix/
- name: {{ template "kong.fullname" . }}-tmp
  mountPath: /tmp
{{- if .Values.deployment.kong.enabled }}
- name: custom-nginx-template-volume
  mountPath: /kong
{{- end }}
{{- if (and (not .Values.ingressController.enabled) (eq .Values.env.database "off")) }}
- name: kong-custom-dbless-config-volume
  mountPath: /kong_dbless/
{{- end }}
{{- range .Values.secretVolumes }}
- name:  {{ . }}
  mountPath: /etc/secrets/{{ . }}
{{- end }}
{{- range .Values.plugins.configMaps }}
{{- $mountPath := printf "/opt/kong/plugins/%s" .pluginName }}
- name:  kong-plugin-{{ .pluginName }}
  mountPath: {{ $mountPath }}
  readOnly: true
{{- range .subdirectories }}
- name: {{ .name  }}
  mountPath: {{ printf "%s/%s" $mountPath ( .path | default .name ) }}
  readOnly: true
{{- end }}
{{- end }}
{{- range .Values.plugins.secrets }}
{{- $mountPath := printf "/opt/kong/plugins/%s" .pluginName }}
- name:  kong-plugin-{{ .pluginName }}
  mountPath: {{ $mountPath }}
  readOnly: true
{{- range .subdirectories }}
- name: {{ .name }}
  mountPath: {{ printf "%s/%s" $mountPath .path }}
  readOnly: true
{{- end }}
{{- end }}

{{- range .Values.extraConfigMaps }}
- name:  {{ .name }}
  mountPath: {{ .mountPath }}

  {{- if .subPath }}
  subPath: {{ .subPath }}
  {{- end }}
{{- end }}
{{- range .Values.extraSecrets }}
- name:  {{ .name }}
  mountPath: {{ .mountPath }}

  {{- if .subPath }}
  subPath: {{ .subPath }}
  {{- end }}
{{- end }}

{{- end -}}

{{- define "kong.plugins" -}}
{{ $myList := list "bundled" }}
{{- range .Values.plugins.configMaps -}}
{{- $myList = append $myList .pluginName -}}
{{- end -}}
{{- range .Values.plugins.secrets -}}
  {{ $myList = append $myList .pluginName -}}
{{- end }}
{{- $myList | join "," -}}
{{- end -}}

{{- define "kong.wait-for-db" -}}
- name: wait-for-db
{{- if .Values.image.unifiedRepoTag }}
  image: "{{ .Values.image.unifiedRepoTag }}"
{{- else }}
  image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
{{- end }}
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  env:
  {{- include "kong.env" . | nindent 2 }}
  command: [ "/bin/sh", "-c", "until kong start; do echo 'waiting for db'; sleep 1; done; kong stop" ]
  volumeMounts:
  {{- include "kong.volumeMounts" . | nindent 4 }}
{{- end -}}

{{- define "kong.controller-container" -}}
- name: ingress-controller
  args:
  - /kong-ingress-controller
  {{ if .Values.ingressController.args}}
  {{- range $val := .Values.ingressController.args }}
  - {{ $val }}
  {{- end }}
  {{- end }}
  env:
  - name: POD_NAME
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: metadata.name
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: metadata.namespace
{{- include "kong.ingressController.env" .  | indent 2 }}
{{- if .Values.ingressController.image.unifiedRepoTag }}
  image: "{{ .Values.ingressController.image.unifiedRepoTag }}"
{{- else }}
  image: "{{ .Values.ingressController.image.repository }}:{{ .Values.ingressController.image.tag }}"
{{- end }}
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  readinessProbe:
{{ toYaml .Values.ingressController.readinessProbe | indent 4 }}
  livenessProbe:
{{ toYaml .Values.ingressController.livenessProbe | indent 4 }}
  resources:
{{ toYaml .Values.ingressController.resources | indent 4 }}
{{- if .Values.ingressController.admissionWebhook.enabled }}
  volumeMounts:
  - name: webhook-cert
    mountPath: /admission-webhook
    readOnly: true
{{- end }}
{{- end -}}

{{- define "secretkeyref" -}}
valueFrom:
  secretKeyRef:
    name: {{ .name }}
    key: {{ .key }}
{{- end -}}

{{/*
Use the Pod security context defined in Values or set the UID by default
*/}}
{{- define "kong.podsecuritycontext" -}}
{{ .Values.securityContext | toYaml }}
{{- end -}}

{{- define "kong.no_daemon_env" -}}
{{- template "kong.env" . }}
- name: KONG_NGINX_DAEMON
  value: "off"
{{- end -}}

{{/*
The environment values passed to Kong; this should come after all
the template that it itself is using form the above sections.
*/}}
{{- define "kong.env" -}}
{{/*
    ====== AUTO-GENERATED ENVIRONMENT VARIABLES ======
*/}}
{{- $autoEnv := dict -}}

{{- $_ := set $autoEnv "KONG_LUA_PACKAGE_PATH" "/opt/?.lua;/opt/?/init.lua;;" -}}

{{- if .Values.ingressController.enabled -}}
  {{- $_ := set $autoEnv "KONG_KIC" "on" -}}
{{- end -}}

{{/*
TODO: remove legacy admin listen behavior at a future date
*/}}

{{- with .Values.admin -}}
  {{- $address := "0.0.0.0" -}}
  {{- if (not .enabled) -}}
    {{- $address = "127.0.0.1" -}}
  {{- end -}}
  {{- if .containerPort -}} {{/* Legacy admin listener */}}
    {{- if .useTLS -}}
      {{- $_ := set $autoEnv "KONG_ADMIN_LISTEN" (printf "%s:%d ssl" $address (int64 .containerPort)) -}}
    {{- else -}}
      {{- $_ := set $autoEnv "KONG_ADMIN_LISTEN" (printf "%s:%d" $address (int64 .containerPort)) -}}
    {{- end -}}
  {{- else -}} {{/* Modern admin listener */}}
    {{- $listenConfig := dict -}}
    {{- $listenConfig := merge $listenConfig . -}}
    {{- $_ := set $listenConfig "address" $address -}}
    {{- $_ := set $autoEnv "KONG_ADMIN_LISTEN" (include "kong.listen" $listenConfig) -}}
  {{- end -}}
{{- end -}}

{{- if .Values.admin.ingress.enabled }}
  {{- $_ := set $autoEnv "KONG_ADMIN_API_URI" (include "kong.ingress.serviceUrl" .Values.admin.ingress) -}}
{{- end -}}

{{- $_ := set $autoEnv "KONG_PROXY_LISTEN" (include "kong.listen" .Values.proxy) -}}

{{- $_ := set $autoEnv "KONG_STREAM_LISTEN" (include "kong.streamListen" .Values.proxy) -}}

{{- $_ := set $autoEnv "KONG_STATUS_LISTEN" (include "kong.listen" .Values.status) -}}

{{- if .Values.proxy.enabled -}}
  {{- $_ := set $autoEnv "KONG_PORT_MAPS" (include "kong.port_maps" .Values.proxy) -}}
{{- end -}}

{{- $_ := set $autoEnv "KONG_CLUSTER_LISTEN" (include "kong.listen" .Values.cluster) -}}

{{- if .Values.enterprise.enabled }}
  {{- $_ := set $autoEnv "KONG_ADMIN_GUI_LISTEN" (include "kong.listen" .Values.manager) -}}
  {{- if .Values.manager.ingress.enabled }}
    {{- $_ := set $autoEnv "KONG_ADMIN_GUI_URL" (include "kong.ingress.serviceUrl" .Values.manager.ingress) -}}
  {{- end -}}

  {{- if not .Values.enterprise.vitals.enabled }}
    {{- $_ := set $autoEnv "KONG_VITALS" "off" -}}
  {{- end }}
  {{- $_ := set $autoEnv "KONG_CLUSTER_TELEMETRY_LISTEN" (include "kong.listen" .Values.clustertelemetry) -}}

  {{- if .Values.enterprise.portal.enabled }}
    {{- $_ := set $autoEnv "KONG_PORTAL" "on" -}}
      {{- $_ := set $autoEnv "KONG_PORTAL_GUI_LISTEN" (include "kong.listen" .Values.portal) -}}
    {{- $_ := set $autoEnv "KONG_PORTAL_API_LISTEN" (include "kong.listen" .Values.portalapi) -}}

    {{- if .Values.portal.ingress.enabled }}
      {{- $_ := set $autoEnv "KONG_PORTAL_GUI_HOST" .Values.portal.ingress.hostname -}}
      {{- if .Values.portal.ingress.tls }}
        {{- $_ := set $autoEnv "KONG_PORTAL_GUI_PROTOCOL" "https" -}}
      {{- else }}
        {{- $_ := set $autoEnv "KONG_PORTAL_GUI_PROTOCOL" "http" -}}
      {{- end }}
    {{- end }}

    {{- if .Values.portalapi.ingress.enabled }}
      {{- $_ := set $autoEnv "KONG_PORTAL_API_URL" (include "kong.ingress.serviceUrl" .Values.portalapi.ingress) -}}
    {{- end }}

    {{- if .Values.enterprise.portal.portal_auth }} {{/* TODO: deprecated, remove in a future version */}}
      {{- $_ := set $autoEnv "KONG_PORTAL_AUTH" .Values.enterprise.portal.portal_auth -}}
      {{- $portalSession := include "secretkeyref" (dict "name" .Values.enterprise.portal.session_conf_secret "key" "portal_session_conf") -}}
      {{- $_ := set $autoEnv "KONG_PORTAL_SESSION_CONF" $portalSession -}}
    {{- end }}
  {{- end }}

  {{- if .Values.enterprise.rbac.enabled }}
    {{- $_ := set $autoEnv "KONG_ENFORCE_RBAC" "on" -}}
    {{- $_ := set $autoEnv "KONG_ADMIN_GUI_AUTH" .Values.enterprise.rbac.admin_gui_auth | default "basic-auth" -}}

    {{- if not (eq .Values.enterprise.rbac.admin_gui_auth "basic-auth") }}
      {{- $guiAuthConf := include "secretkeyref" (dict "name" .Values.enterprise.rbac.admin_gui_auth_conf_secret "key" "admin_gui_auth_conf") -}}
      {{- $_ := set $autoEnv "KONG_ADMIN_GUI_AUTH_CONF" $guiAuthConf -}}
    {{- end }}

    {{- $guiSessionConf := include "secretkeyref" (dict "name" .Values.enterprise.rbac.session_conf_secret "key" "admin_gui_session_conf") -}}
    {{- $_ := set $autoEnv "KONG_ADMIN_GUI_SESSION_CONF" $guiSessionConf -}}
  {{- end }}

  {{- if .Values.enterprise.smtp.enabled }}
    {{- $_ := set $autoEnv "KONG_SMTP_MOCK" "off" -}}
    {{- $_ := set $autoEnv "KONG_PORTAL_EMAILS_FROM" .Values.enterprise.smtp.portal_emails_from -}}
    {{- $_ := set $autoEnv "KONG_PORTAL_EMAILS_REPLY_TO" .Values.enterprise.smtp.portal_emails_reply_to -}}
    {{- $_ := set $autoEnv "KONG_ADMIN_EMAILS_FROM" .Values.enterprise.smtp.admin_emails_from -}}
    {{- $_ := set $autoEnv "KONG_ADMIN_EMAILS_REPLY_TO" .Values.enterprise.smtp.admin_emails_reply_to -}}
    {{- $_ := set $autoEnv "KONG_SMTP_ADMIN_EMAILS" .Values.enterprise.smtp.smtp_admin_emails -}}
    {{- $_ := set $autoEnv "KONG_SMTP_HOST" .Values.enterprise.smtp.smtp_host -}}
    {{- $_ := set $autoEnv "KONG_SMTP_AUTH_TYPE" .Values.enterprise.smtp.smtp_auth_type -}}
    {{- $_ := set $autoEnv "KONG_SMTP_SSL" .Values.enterprise.smtp.smtp_ssl -}}
    {{- $_ := set $autoEnv "KONG_SMTP_PORT" .Values.enterprise.smtp.smtp_port -}}
    {{- $_ := set $autoEnv "KONG_SMTP_STARTTLS" (quote .Values.enterprise.smtp.smtp_starttls) -}}
    {{- if .Values.enterprise.smtp.auth.smtp_username }}
      {{- $_ := set $autoEnv "KONG_SMTP_USERNAME" .Values.enterprise.smtp.auth.smtp_username -}}
      {{- $smtpPassword := include "secretkeyref" (dict "name" .Values.enterprise.smtp.auth.smtp_password_secret "key" "smtp_password") -}}
      {{- $_ := set $autoEnv "KONG_SMTP_PASSWORD" $smtpPassword -}}
    {{- end }}
  {{- else }}
    {{- $_ := set $autoEnv "KONG_SMTP_MOCK" "on" -}}
  {{- end }}

  {{- $lic := include "secretkeyref" (dict "name" .Values.enterprise.license_secret "key" "license") -}}
  {{- $_ := set $autoEnv "KONG_LICENSE_DATA" $lic -}}

{{- end }} {{/* End of the Enterprise settings block */}}

{{- $_ := set $autoEnv "KONG_NGINX_HTTP_INCLUDE" "/kong/servers.conf" -}}

{{- if .Values.postgresql.enabled }}
  {{- $_ := set $autoEnv "KONG_PG_HOST" (include "kong.postgresql.fullname" .) -}}
  {{- $_ := set $autoEnv "KONG_PG_PORT" .Values.postgresql.service.port -}}
  {{- $pgPassword := include "secretkeyref" (dict "name" (include "kong.postgresql.fullname" .) "key" "postgresql-password") -}}
  {{- $_ := set $autoEnv "KONG_PG_PASSWORD" $pgPassword -}}
{{- else if eq .Values.env.database "postgres" }}
  {{- $_ := set $autoEnv "KONG_PG_PORT" "5432" }}
{{- end }}

{{- if (and (not .Values.ingressController.enabled) (eq .Values.env.database "off")) }}
  {{- $_ := set $autoEnv "KONG_DECLARATIVE_CONFIG" "/kong_dbless/kong.yml" -}}
{{- end }}

{{- $_ := set $autoEnv "KONG_PLUGINS" (include "kong.plugins" .) -}}

{{/*
    ====== USER-SET ENVIRONMENT VARIABLES ======
*/}}

{{- $userEnv := dict -}}
{{- range $key, $val := .Values.env }}
  {{- $upper := upper $key -}}
  {{- $var := printf "KONG_%s" $upper -}}
  {{- $_ := set $userEnv $var $val -}}
{{- end -}}

{{/*
      ====== MERGE AND RENDER ENV BLOCK ======
*/}}

{{- $completeEnv := mergeOverwrite $autoEnv $userEnv -}}
{{- template "kong.renderEnv" $completeEnv -}}

{{- end -}}

{{/*
Given a dictionary of variable=value pairs, render a container env block.
Environment variables are sorted alphabetically
*/}}
{{- define "kong.renderEnv" -}}

{{- $dict := . -}}

{{- range keys . | sortAlpha }}
{{- $val := pluck . $dict | first -}}
{{- $valueType := printf "%T" $val -}}
{{ if eq $valueType "map[string]interface {}" }}
- name: {{ . }}
{{ toYaml $val | indent 2 -}}
{{- else if eq $valueType "string" }}
{{- if regexMatch "valueFrom" $val }}
- name: {{ . }}
{{ $val | indent 2 }}
{{- else }}
- name: {{ . }}
  value: {{ $val | quote }}
{{- end }}
{{- else }}
- name: {{ . }}
  value: {{ $val | quote }}
{{- end }}
{{- end -}}

{{- end -}}

{{- define "kong.wait-for-postgres" -}}
- name: wait-for-postgres
{{- if .Values.waitImage.unifiedRepoTag }}
  image: "{{ .Values.waitImage.unifiedRepoTag }}"
{{- else }}
  image: "{{ .Values.waitImage.repository }}:{{ .Values.waitImage.tag }}"
{{- end }}
  imagePullPolicy: {{ .Values.waitImage.pullPolicy }}
  env:
  {{- include "kong.no_daemon_env" . | nindent 2 }}
  command: [ "bash", "/wait_postgres/wait.sh" ]
  volumeMounts:
  - name: {{ template "kong.fullname" . }}-bash-wait-for-postgres
    mountPath: /wait_postgres
{{- end -}}

{{- define "kong.deprecation-warnings" -}}
  {{- $warnings := list -}}
  {{- range $warning := . }}
    {{- $warnings = append $warnings (wrap 80 (printf "WARNING: %s" $warning)) -}}
    {{- $warnings = append $warnings "\n\n" -}}
  {{- end -}}
  {{- $warningString := ($warnings | join "") -}}
  {{- $warningString -}}
{{- end -}}

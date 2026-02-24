# Helm Chart Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship a production-ready Helm chart at `deploy/helm/openincident/` that deploys the full OpenIncident stack with a single `helm install`.

**Architecture:** Single umbrella chart with Bitnami PostgreSQL + Redis as optional subcharts. A `pre-install,pre-upgrade` Helm hook Job runs DB migrations before new pods start. The API Deployment scales via HPA (2–10 replicas) and exposes itself through a configurable Ingress.

**Tech Stack:** Helm v3, Bitnami postgresql ~16.x, Bitnami redis ~21.x, Kubernetes ≥1.25.

---

## Task 1: Add `migrate` CLI subcommand to the backend

The `serve` command auto-migrates at startup, but the Helm pre-install Job needs a command that runs migrations and exits cleanly. This is a small code addition.

**Files:**
- Create: `backend/cmd/openincident/commands/migrate.go`
- Modify: `backend/cmd/openincident/commands/root.go`

**Step 1: Read root.go to understand how commands are registered**

```bash
cat backend/cmd/openincident/commands/root.go
```

**Step 2: Read serve.go lines 50–80 to understand DB setup + migration call**

```bash
sed -n '50,80p' backend/cmd/openincident/commands/serve.go
```

**Step 3: Write `migrate.go`**

```go
package commands

import (
	"log/slog"

	"github.com/openincident/openincident/internal/database"
	"github.com/spf13/cobra"
)

func NewMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbConfig := database.Config{
				URL: mustEnv("DATABASE_URL"),
			}
			if err := database.Connect(dbConfig); err != nil {
				return err
			}
			defer database.Close()

			slog.Info("running database migrations")
			if err := database.RunMigrations(database.DB, "./migrations"); err != nil {
				return err
			}
			slog.Info("migrations complete")
			return nil
		},
	}
}
```

Note: `mustEnv` is already defined in serve.go — verify it's in the same package and accessible, or inline the `os.Getenv` + fatal check.

**Step 4: Register the command in root.go**

Find the line in `root.go` where `serve` is added (e.g. `rootCmd.AddCommand(NewServeCmd())`), and add:

```go
rootCmd.AddCommand(NewMigrateCmd())
```

**Step 5: Build to verify it compiles**

```bash
cd backend && go build ./...
```

Expected: no errors.

**Step 6: Smoke test**

```bash
./bin/openincident migrate --help
```

Expected: help text mentioning "Run database migrations and exit".

**Step 7: Commit**

```bash
git add backend/cmd/openincident/commands/migrate.go backend/cmd/openincident/commands/root.go
git commit -m "feat(cli): add migrate subcommand for Helm pre-install job"
```

---

## Task 2: Scaffold the chart skeleton

**Files:**
- Create: `deploy/helm/openincident/Chart.yaml`
- Create: `deploy/helm/openincident/values.yaml`
- Create: `deploy/helm/openincident/templates/_helpers.tpl`
- Create: `deploy/helm/openincident/.helmignore`

**Step 1: Create Chart.yaml**

```yaml
# deploy/helm/openincident/Chart.yaml
apiVersion: v2
name: openincident
description: Open-source incident management platform
type: application
version: 0.1.0
appVersion: "0.9.0"
keywords:
  - incident-management
  - on-call
  - alerting
home: https://github.com/openincident/openincident
sources:
  - https://github.com/openincident/openincident
maintainers:
  - name: OpenIncident Contributors
dependencies:
  - name: postgresql
    version: ">=16.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
  - name: redis
    version: ">=21.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: redis.enabled
```

**Step 2: Create values.yaml**

```yaml
# deploy/helm/openincident/values.yaml

## Number of replicas (used only when autoscaling.enabled=false)
replicaCount: 2

image:
  repository: ghcr.io/openincident/openincident
  pullPolicy: IfNotPresent
  # Overrides the image tag whose default is the chart appVersion.
  tag: ""

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

podAnnotations: {}
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1001
  fsGroup: 1001

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: true
  className: nginx
  host: ""
  tls: false
  tlsSecretName: ""
  annotations: {}

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

# Non-sensitive configuration
config:
  logLevel: info
  port: "8080"
  appEnv: production
  samlBaseURL: ""
  samlAllowIdpInitiated: "false"
  teamsServiceURL: "https://smba.trafficmanager.net/amer/"
  slackAutoInviteUserIDs: ""

# Sensitive secrets — rendered into a Kubernetes Secret.
# Leave a value empty ("") to omit that env var entirely.
secrets:
  # Override auto-computed DATABASE_URL (leave empty to use bundled postgresql)
  databaseURL: ""
  # Override auto-computed REDIS_URL (leave empty to use bundled redis)
  redisURL: ""
  slackBotToken: ""
  slackSigningSecret: ""
  slackAppToken: ""
  openaiAPIKey: ""
  teamsAppID: ""
  teamsAppPassword: ""
  teamsTenantID: ""
  teamsTeamID: ""
  teamsBotUserID: ""
  samlIDPMetadataURL: ""
  samlEntityID: ""
  webhookSecret: ""

# Bitnami PostgreSQL subchart — https://github.com/bitnami/charts/tree/main/bitnami/postgresql
postgresql:
  enabled: true
  auth:
    database: openincident
    username: openincident
    # IMPORTANT: change this in production
    password: changeme

# Bitnami Redis subchart — https://github.com/bitnami/charts/tree/main/bitnami/redis
redis:
  enabled: true
  architecture: standalone
  auth:
    enabled: false

nodeSelector: {}
tolerations: []
affinity: {}
```

**Step 3: Create `_helpers.tpl`**

```
{{- /*
deploy/helm/openincident/templates/_helpers.tpl
*/ -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "openincident.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "openincident.fullname" -}}
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
{{- define "openincident.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "openincident.labels" -}}
helm.sh/chart: {{ include "openincident.chart" . }}
{{ include "openincident.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "openincident.selectorLabels" -}}
app.kubernetes.io/name: {{ include "openincident.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "openincident.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "openincident.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Compute DATABASE_URL.
If secrets.databaseURL is set, use it directly.
Otherwise build from bundled postgresql subchart credentials.
*/}}
{{- define "openincident.databaseURL" -}}
{{- if .Values.secrets.databaseURL }}
{{- .Values.secrets.databaseURL }}
{{- else }}
{{- printf "postgresql://%s:%s@%s-postgresql:5432/%s?sslmode=disable"
    .Values.postgresql.auth.username
    .Values.postgresql.auth.password
    (include "openincident.fullname" .)
    .Values.postgresql.auth.database }}
{{- end }}
{{- end }}

{{/*
Compute REDIS_URL.
If secrets.redisURL is set, use it directly.
Otherwise build from bundled redis subchart.
*/}}
{{- define "openincident.redisURL" -}}
{{- if .Values.secrets.redisURL }}
{{- .Values.secrets.redisURL }}
{{- else }}
{{- printf "redis://%s-redis-master:6379" (include "openincident.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Container image reference.
*/}}
{{- define "openincident.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
```

**Step 4: Create `.helmignore`**

```
# Patterns to ignore when building packages.
.DS_Store
*.swp
*.bak
*.tmp
*.orig
*~
.git/
.gitignore
*.md
```

**Step 5: Run helm dep update**

```bash
helm dependency update deploy/helm/openincident
```

Expected: Downloads `postgresql-16.x.x.tgz` and `redis-21.x.x.tgz` into `deploy/helm/openincident/charts/`.

**Step 6: Run helm lint**

```bash
helm lint deploy/helm/openincident
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

**Step 7: Commit**

```bash
git add deploy/helm/
git commit -m "feat(helm): scaffold chart skeleton with Bitnami subcharts"
```

---

## Task 3: ConfigMap + Secret templates

**Files:**
- Create: `deploy/helm/openincident/templates/configmap.yaml`
- Create: `deploy/helm/openincident/templates/secret.yaml`

**Step 1: Create configmap.yaml**

```yaml
# deploy/helm/openincident/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
data:
  APP_ENV: {{ .Values.config.appEnv | quote }}
  LOG_LEVEL: {{ .Values.config.logLevel | quote }}
  PORT: {{ .Values.config.port | quote }}
  {{- if .Values.config.samlBaseURL }}
  SAML_BASE_URL: {{ .Values.config.samlBaseURL | quote }}
  {{- end }}
  SAML_ALLOW_IDP_INITIATED: {{ .Values.config.samlAllowIdpInitiated | quote }}
  TEAMS_SERVICE_URL: {{ .Values.config.teamsServiceURL | quote }}
  {{- if .Values.config.slackAutoInviteUserIDs }}
  SLACK_AUTO_INVITE_USER_IDS: {{ .Values.config.slackAutoInviteUserIDs | quote }}
  {{- end }}
```

**Step 2: Create secret.yaml**

```yaml
# deploy/helm/openincident/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
type: Opaque
stringData:
  DATABASE_URL: {{ include "openincident.databaseURL" . | quote }}
  REDIS_URL: {{ include "openincident.redisURL" . | quote }}
  {{- if .Values.secrets.slackBotToken }}
  SLACK_BOT_TOKEN: {{ .Values.secrets.slackBotToken | quote }}
  {{- end }}
  {{- if .Values.secrets.slackSigningSecret }}
  SLACK_SIGNING_SECRET: {{ .Values.secrets.slackSigningSecret | quote }}
  {{- end }}
  {{- if .Values.secrets.slackAppToken }}
  SLACK_APP_TOKEN: {{ .Values.secrets.slackAppToken | quote }}
  {{- end }}
  {{- if .Values.secrets.openaiAPIKey }}
  OPENAI_API_KEY: {{ .Values.secrets.openaiAPIKey | quote }}
  {{- end }}
  {{- if .Values.secrets.teamsAppID }}
  TEAMS_APP_ID: {{ .Values.secrets.teamsAppID | quote }}
  {{- end }}
  {{- if .Values.secrets.teamsAppPassword }}
  TEAMS_APP_PASSWORD: {{ .Values.secrets.teamsAppPassword | quote }}
  {{- end }}
  {{- if .Values.secrets.teamsTenantID }}
  TEAMS_TENANT_ID: {{ .Values.secrets.teamsTenantID | quote }}
  {{- end }}
  {{- if .Values.secrets.teamsTeamID }}
  TEAMS_TEAM_ID: {{ .Values.secrets.teamsTeamID | quote }}
  {{- end }}
  {{- if .Values.secrets.teamsBotUserID }}
  TEAMS_BOT_USER_ID: {{ .Values.secrets.teamsBotUserID | quote }}
  {{- end }}
  {{- if .Values.secrets.samlIDPMetadataURL }}
  SAML_IDP_METADATA_URL: {{ .Values.secrets.samlIDPMetadataURL | quote }}
  {{- end }}
  {{- if .Values.secrets.samlEntityID }}
  SAML_ENTITY_ID: {{ .Values.secrets.samlEntityID | quote }}
  {{- end }}
  {{- if .Values.secrets.webhookSecret }}
  WEBHOOK_SECRET: {{ .Values.secrets.webhookSecret | quote }}
  {{- end }}
```

**Step 3: Lint**

```bash
helm lint deploy/helm/openincident
```

Expected: 0 failures.

**Step 4: Template and inspect output**

```bash
helm template openincident deploy/helm/openincident | grep -A 20 "kind: Secret"
```

Expected: Secret with `DATABASE_URL` auto-computed from postgresql values.

**Step 5: Commit**

```bash
git add deploy/helm/openincident/templates/configmap.yaml deploy/helm/openincident/templates/secret.yaml
git commit -m "feat(helm): add ConfigMap and Secret templates"
```

---

## Task 4: ServiceAccount + Service templates

**Files:**
- Create: `deploy/helm/openincident/templates/serviceaccount.yaml`
- Create: `deploy/helm/openincident/templates/service.yaml`

**Step 1: Create serviceaccount.yaml**

```yaml
# deploy/helm/openincident/templates/serviceaccount.yaml
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "openincident.serviceAccountName" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: false
{{- end }}
```

**Step 2: Create service.yaml**

```yaml
# deploy/helm/openincident/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    {{- include "openincident.selectorLabels" . | nindent 4 }}
```

**Step 3: Lint**

```bash
helm lint deploy/helm/openincident
```

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/serviceaccount.yaml deploy/helm/openincident/templates/service.yaml
git commit -m "feat(helm): add ServiceAccount and Service templates"
```

---

## Task 5: Deployment template

**Files:**
- Create: `deploy/helm/openincident/templates/deployment.yaml`

**Step 1: Create deployment.yaml**

```yaml
# deploy/helm/openincident/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "openincident.selectorLabels" . | nindent 6 }}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      annotations:
        # Roll pods when Secret or ConfigMap changes
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
        checksum/secret: {{ include (print $.Template.BasePath "/secret.yaml") . | sha256sum }}
        {{- with .Values.podAnnotations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      labels:
        {{- include "openincident.selectorLabels" . | nindent 8 }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "openincident.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      terminationGracePeriodSeconds: 30
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              {{- include "openincident.selectorLabels" . | nindent 14 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: {{ include "openincident.image" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args: ["serve"]
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          envFrom:
            - configMapRef:
                name: {{ include "openincident.fullname" . }}
            - secretRef:
                name: {{ include "openincident.fullname" . }}
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 10
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          volumeMounts:
            - name: tmp
              mountPath: /tmp
      volumes:
        # readOnlyRootFilesystem=true requires a writable /tmp
        - name: tmp
          emptyDir: {}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
```

**Step 2: Lint**

```bash
helm lint deploy/helm/openincident
```

**Step 3: Template and spot check**

```bash
helm template openincident deploy/helm/openincident | grep -A 5 "livenessProbe"
```

Expected: shows `path: /health`.

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/deployment.yaml
git commit -m "feat(helm): add Deployment template with probes and topology spread"
```

---

## Task 6: Ingress template

**Files:**
- Create: `deploy/helm/openincident/templates/ingress.yaml`

**Step 1: Create ingress.yaml**

```yaml
# deploy/helm/openincident/templates/ingress.yaml
{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
  {{- with .Values.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.ingress.className }}
  ingressClassName: {{ .Values.ingress.className }}
  {{- end }}
  {{- if .Values.ingress.tls }}
  tls:
    - hosts:
        - {{ .Values.ingress.host | quote }}
      secretName: {{ .Values.ingress.tlsSecretName | quote }}
  {{- end }}
  rules:
    - host: {{ .Values.ingress.host | quote }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "openincident.fullname" . }}
                port:
                  name: http
{{- end }}
```

**Step 2: Lint**

```bash
helm lint deploy/helm/openincident
```

**Step 3: Template with a host set**

```bash
helm template openincident deploy/helm/openincident --set ingress.host=incidents.example.com | grep -A 15 "kind: Ingress"
```

Expected: Ingress with `host: incidents.example.com` and `ingressClassName: nginx`.

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/ingress.yaml
git commit -m "feat(helm): add Ingress template"
```

---

## Task 7: HPA template

**Files:**
- Create: `deploy/helm/openincident/templates/hpa.yaml`

**Step 1: Create hpa.yaml**

```yaml
# deploy/helm/openincident/templates/hpa.yaml
{{- if .Values.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "openincident.fullname" . }}
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "openincident.fullname" . }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetCPUUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetMemoryUtilizationPercentage }}
{{- end }}
```

**Step 2: Lint + verify**

```bash
helm lint deploy/helm/openincident
helm template openincident deploy/helm/openincident | grep -A 5 "kind: HorizontalPodAutoscaler"
```

**Step 3: Verify HPA is suppressed when disabled**

```bash
helm template openincident deploy/helm/openincident --set autoscaling.enabled=false | grep HorizontalPodAutoscaler || echo "HPA correctly absent"
```

Expected: `HPA correctly absent`

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/hpa.yaml
git commit -m "feat(helm): add HPA template (CPU + memory metrics)"
```

---

## Task 8: Migration Job template

**Files:**
- Create: `deploy/helm/openincident/templates/migration-job.yaml`

**Step 1: Create migration-job.yaml**

```yaml
# deploy/helm/openincident/templates/migration-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "openincident.fullname" . }}-migrate
  labels:
    {{- include "openincident.labels" . | nindent 4 }}
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "-5"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  backoffLimit: 3
  template:
    metadata:
      labels:
        {{- include "openincident.selectorLabels" . | nindent 8 }}
        app.kubernetes.io/component: migration
    spec:
      restartPolicy: OnFailure
      serviceAccountName: {{ include "openincident.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: migrate
          image: {{ include "openincident.image" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args: ["migrate"]
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          envFrom:
            - secretRef:
                name: {{ include "openincident.fullname" . }}
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
          volumeMounts:
            - name: tmp
              mountPath: /tmp
      volumes:
        - name: tmp
          emptyDir: {}
```

**Step 2: Lint**

```bash
helm lint deploy/helm/openincident
```

**Step 3: Template and verify hook annotations**

```bash
helm template openincident deploy/helm/openincident | grep -A 5 "helm.sh/hook"
```

Expected: `pre-install,pre-upgrade` and `hook-delete-policy`.

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/migration-job.yaml
git commit -m "feat(helm): add pre-install migration Job hook"
```

---

## Task 9: NOTES.txt

**Files:**
- Create: `deploy/helm/openincident/templates/NOTES.txt`

**Step 1: Create NOTES.txt**

```
OpenIncident has been deployed!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Get the application URL:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
{{- if .Values.ingress.enabled }}
  http{{ if .Values.ingress.tls }}s{{ end }}://{{ .Values.ingress.host }}/

  Wait for the Ingress IP to propagate, then verify:
    curl http{{ if .Values.ingress.tls }}s{{ end }}://{{ .Values.ingress.host }}/health
{{- else }}
  Ingress is disabled. Port-forward to access the API:
    kubectl port-forward svc/{{ include "openincident.fullname" . }} 8080:8080
    curl http://localhost:8080/health
{{- end }}

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Production checklist:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
{{- if .Values.postgresql.enabled }}
  ⚠  Change the default PostgreSQL password:
     --set postgresql.auth.password=<strong-password>
{{- end }}
  ✓  Set secrets (Slack, OpenAI, Teams) via --set secrets.<key>=<value>
     or a sealed-secrets / external-secrets operator.

  📖  Full setup docs:
     https://github.com/openincident/openincident#kubernetes
```

**Step 2: Lint**

```bash
helm lint deploy/helm/openincident
```

**Step 3: Template to preview NOTES output**

```bash
helm template openincident deploy/helm/openincident --set ingress.host=incidents.example.com | grep -A 5 "NOTES" || helm install --dry-run --generate-name deploy/helm/openincident --set ingress.host=incidents.example.com 2>&1 | grep -A 20 "NOTES:"
```

**Step 4: Commit**

```bash
git add deploy/helm/openincident/templates/NOTES.txt
git commit -m "feat(helm): add NOTES.txt with post-install instructions"
```

---

## Task 10: Makefile targets + final lint/dry-run

**Files:**
- Modify: `Makefile`

**Step 1: Read the current Makefile help/phony section**

```bash
grep -n "PHONY\|helm\|##" Makefile | head -20
```

**Step 2: Add Helm targets**

Find the end of the existing targets and add:

```makefile
## Helm
.PHONY: helm-deps helm-lint helm-template helm-test

helm-deps: ## Download Helm chart dependencies
	helm dependency update deploy/helm/openincident

helm-lint: helm-deps ## Lint the Helm chart
	helm lint deploy/helm/openincident

helm-template: helm-deps ## Dry-run render the chart (requires kubectl context)
	helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost \
		| kubectl apply --dry-run=client -f -

helm-test: helm-lint ## Run all Helm checks (lint + template render)
	helm template openincident deploy/helm/openincident \
		--set ingress.host=localhost > /dev/null && \
		echo "✓ helm template: OK"
```

**Step 3: Run helm-test**

```bash
make helm-test
```

Expected:
```
1 chart(s) linted, 0 chart(s) failed
✓ helm template: OK
```

**Step 4: Run helm-lint with strict flag**

```bash
helm lint deploy/helm/openincident --strict
```

Expected: 0 warnings, 0 failures.

**Step 5: Verify external-postgresql path works**

```bash
helm template openincident deploy/helm/openincident \
  --set postgresql.enabled=false \
  --set secrets.databaseURL="postgresql://user:pass@my-rds.example.com:5432/openincident?sslmode=require" \
  --set ingress.host=localhost | grep "DATABASE_URL" || \
helm template openincident deploy/helm/openincident \
  --set postgresql.enabled=false \
  --set secrets.databaseURL="postgresql://user:pass@my-rds.example.com:5432/openincident?sslmode=require" \
  --set ingress.host=localhost | grep -A 30 "kind: Secret"
```

Expected: Secret contains the custom DATABASE_URL, no postgresql subchart objects rendered.

**Step 6: Commit**

```bash
git add Makefile
git commit -m "feat(helm): add helm-deps/lint/template/test Makefile targets"
```

---

## Task 11: Add charts/ to .gitignore + document in README

**Files:**
- Modify: `.gitignore` (repo root)
- Modify: `README.md`

**Step 1: Add charts/ to .gitignore**

Subchart tarballs are downloaded by `helm dep update`, not checked in.

```bash
echo "\n# Helm subchart tarballs (generated by helm dep update)\ndeploy/helm/openincident/charts/" >> .gitignore
```

Verify it looks correct:

```bash
tail -5 .gitignore
```

**Step 2: Add Kubernetes section to README**

Find the existing deployment section in README.md and add after it:

```markdown
## Kubernetes (Helm)

```bash
# Add the Bitnami chart repo (first time only)
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Download subchart dependencies
make helm-deps

# Install with bundled PostgreSQL + Redis (development / evaluation)
helm install openincident deploy/helm/openincident \
  --set ingress.host=incidents.myco.com \
  --set postgresql.auth.password=<strong-password>

# Install pointing at managed DB + Redis (production)
helm install openincident deploy/helm/openincident \
  --set postgresql.enabled=false \
  --set redis.enabled=false \
  --set secrets.databaseURL="postgresql://user:pass@your-rds.example.com:5432/openincident?sslmode=require" \
  --set secrets.redisURL="redis://your-elasticache.example.com:6379" \
  --set ingress.host=incidents.myco.com
```

### Upgrade

```bash
helm upgrade openincident deploy/helm/openincident \
  --reuse-values \
  --set image.tag=0.10.0
```

Migrations run automatically as a pre-upgrade Job before pods are replaced.

### Configuration

See `deploy/helm/openincident/values.yaml` for all available values. Key overrides:

| Key | Default | Description |
|-----|---------|-------------|
| `ingress.host` | `""` | Hostname for the Ingress (**required**) |
| `postgresql.auth.password` | `changeme` | Change in production |
| `autoscaling.enabled` | `true` | Enable HPA (2–10 replicas) |
| `secrets.slackBotToken` | `""` | Slack bot token |
| `secrets.openaiAPIKey` | `""` | OpenAI API key for AI features |
```

**Step 3: Final lint**

```bash
make helm-test
```

Expected: all green.

**Step 4: Commit**

```bash
git add .gitignore README.md
git commit -m "docs(helm): add Kubernetes/Helm section to README"
```

---

## Task 12: Final verification

**Step 1: Full lint with --strict**

```bash
helm lint deploy/helm/openincident --strict
```

Expected: `1 chart(s) linted, 0 chart(s) failed`

**Step 2: Template full render (no errors)**

```bash
helm template openincident deploy/helm/openincident \
  --set ingress.host=incidents.example.com \
  --set postgresql.auth.password=secret123 \
  > /tmp/openincident-manifests.yaml && \
  echo "Lines rendered: $(wc -l < /tmp/openincident-manifests.yaml)"
```

Expected: several hundred lines, no template errors.

**Step 3: Count resource kinds**

```bash
grep "^kind:" /tmp/openincident-manifests.yaml | sort | uniq -c
```

Expected to see: ConfigMap, Deployment, HorizontalPodAutoscaler, Ingress, Job, Secret, Service, ServiceAccount (plus PostgreSQL and Redis subchart resources).

**Step 4: Verify autoscaling=false removes HPA and uses replicaCount**

```bash
helm template openincident deploy/helm/openincident \
  --set autoscaling.enabled=false \
  --set replicaCount=3 \
  --set ingress.host=localhost | grep -E "replicas:|kind: HorizontalPodAutoscaler"
```

Expected: `replicas: 3`, no HPA.

**Step 5: Final commit if any cleanup needed, then push**

```bash
git push
```

# Template Package

> Adaptation reference for `quartz.yaml`, `stage.yaml`, health checks, and new-program setup.

This document provides a starting point for projects that want to use `quartzctl` as their staged OpenTofu orchestrator.

## Project Layout

```text
my-platform/
  quartz.yaml
  stages/
    01-infrastructure/
      main.tf
    02-cluster-addons/
      main.tf
      stage.yaml
    03-apps/
      main.tf
      stage.yaml
```

Default `stage_paths` is `./tofu/stages`, but projects can point anywhere.

## `quartz.yaml` Core Reference

```yaml
name: prog-dev-001
project: quartz
tmp: ./out

dns:
  zone: example.com
  # domain: prog-dev-001.example.com

aws:
  region: us-east-1

github:
  organization: MetroStar
  webhooks:
    build: false

providers:
  cloud: aws
  dns: aws
  source_control: github
  monitoring: cloudwatch
  secrets: aws-ssm-parameter
  oidc: keycloak
  cicd: jenkins

stage_paths:
  - stages

stages: {}

gitops:
  core:
    repo: my-platform
    branch: main
  apps:
    repo: my-platform-apps

administrators: []
applications: {}
```

## Environment Overrides

Any config key can be loaded from environment variables prefixed with `QUARTZ_`, using underscores for dots:

```bash
export QUARTZ_NAME=prog-dev-001
export QUARTZ_AWS_REGION=us-east-1
export QUARTZ_DNS_ZONE=example.com
```

Credential environment variables include:

```bash
export GITHUB_USERNAME=...
export GITHUB_TOKEN=...
export IRONBANK_USERNAME=...
export IRONBANK_PASSWORD=...
export CLOUDFLARE_API_TOKEN=...
```

## Stage Directory Contract

A folder named `20-core` becomes:

```yaml
id: core
order: 20
path: <absolute path>/20-core
type: tofu
```

Use `stage.yaml` only for overrides and stage-specific behavior.

## `stage.yaml` Reference

```yaml
description: Core platform components
dependencies:
  - infrastructure
providers:
  kubernetes: true
vars:
  domain:
    config: dns.domain
  github_token:
    secret: github.token
  cluster_name:
    value: prog-dev-001
  build_number:
    env: BUILD_NUMBER
  vpc_id:
    stage:
      name: infrastructure
      output: vpc.vpc_id
checks:
  pre_install:
    before: [apply]
    kubernetes:
      - name: cert-manager
        namespace: cert-manager
        kind: Deployment
        state: Available
        timeout: 600
  api_ready:
    after: [apply]
    order: 1
    http:
      - url: https://api.example.com/health
        status_codes: [200]
        retry:
          limit: 30
          wait_seconds: 10
destroy:
  exclude:
    - kubernetes_namespace.protected
flux:
  converge_targets:
    - kubernetes_secret.values
```

## Variable Sources

| Key | Meaning |
|-----|---------|
| `value` | Literal value |
| `env` | Environment variable |
| `config` | Path in rendered config |
| `secret` | Path in loaded secrets |
| `stage.name` / `stage.output` | Output from another stage |

## Health Check Patterns

### Kubernetes Readiness

```yaml
checks:
  post_install:
    after: [apply]
    kubernetes:
      - name: my-app
        namespace: my-ns
        kind: Deployment
        state: Available
        timeout: 900
```

### DaemonSet Readiness

```yaml
checks:
  node_agent_ready:
    after: [apply]
    daemonset:
      - name: node-agent
        namespace: kube-system
        retry:
          limit: 30
          wait_seconds: 10
```

### HTTP Content

```yaml
checks:
  api_ready:
    after: [apply]
    http:
      - path: /api/system/status
        app: sonarqube
        content:
          json:
            key: status
          value: UP
        retry:
          limit: 60
          wait_seconds: 15
```

### State Check

```yaml
checks:
  initialized:
    before: [apply]
    state:
      - key: app.initialized
        value: "true"
        retry:
          limit: 12
          wait_seconds: 10
```

## Destroy Controls

| Key | Use |
|-----|-----|
| `destroy.skip` | Do not destroy the stage during `quartz clean` |
| `destroy.include` | Destroy only matching state addresses |
| `destroy.exclude` | Destroy everything except matching state addresses |

Use include/exclude carefully and document why each address is filtered.

## New Program Checklist

**Config**

- [ ] `name`, `dns`, and `aws.region` set.
- [ ] `stage_paths` points at real stage directories.
- [ ] `github.organization` and GitOps repos set if used.
- [ ] Secrets source documented.

**Stages**

- [ ] Stage folder prefixes are ordered.
- [ ] `dependencies` are explicit where order matters.
- [ ] Cross-stage outputs exist and are stable.
- [ ] Health checks use real object names and realistic timeouts.
- [ ] Destroy include/exclude rules are justified.

**Operations**

- [ ] `quartz check` succeeds.
- [ ] `quartz render` output reviewed.
- [ ] First `quartz install --yes` succeeds.
- [ ] `quartz info` works where Kubernetes is present.
- [ ] `quartz clean --yes` tested in non-production.

## Customization Boundaries

- Use `quartz.yaml` for environment-specific inputs.
- Use `stage.yaml` for stage behavior.
- Keep OpenTofu resource logic in stage directories.
- Keep secrets out of source control.
- Prefer CLI state-repair commands over manual backend edits.

# OpenTofu Migration (Phase 1)

This document describes the migration from HashiCorp Terraform to OpenTofu in the Quartz platform.

## Summary

Quartz has migrated from Terraform 1.5.7 (the last MPL-licensed release) to OpenTofu 1.11.6. OpenTofu is a community-driven, open-source fork of Terraform maintained under the Linux Foundation. It is fully compatible with existing Terraform configurations and state files.

## Changes

### quartzctl

| Area | Before | After |
|------|--------|-------|
| IaC binary | Terraform 1.5.7 (via hc-install) | OpenTofu 1.11.6 (via tofudl) |
| Go package | `internal/terraform/` | `internal/tofu/` |
| Config key | `terraform:` (in quartz.yaml) | `tofu:` |
| Log config | `log.terraform.*` | `log.tofu.*` |
| Stage path | `terraform/stages/` | `tofu/stages/` |
| AWS SDK | aws-sdk-go v1 + v2 | aws-sdk-go-v2 only |
| GitHub client | go-github/v63 | go-github/v72 |
| EKS auth | aws-iam-authenticator v0.7.2 | aws-iam-authenticator v0.7.15 |

### quartz (infrastructure repo)

| Area | Before | After |
|------|--------|-------|
| Folder | `terraform/` | `tofu/` |
| mise.toml | `terraform@1.5.7` | `opentofu@latest` |
| Config key | `terraform:` | `tofu:` |

### quartz-pkgs (build-tools)

| Area | Before | After |
|------|--------|-------|
| Dockerfile | Installs Terraform binary | Installs OpenTofu binary |

## CLI Compatibility

The `quartz` CLI command `terraform` is preserved as an alias. Both work:

```bash
quartz terraform plan    # legacy alias
quartz tofu plan         # preferred
quartz tf plan           # short alias
```

## Configuration Migration

Update your `quartz.yaml`:

```yaml
# Before
terraform:
  version: "1.5.7"

log:
  terraform:
    enabled: true
    path: "./log"

# After
tofu:
  version: "1.11.6"

log:
  tofu:
    enabled: true
    path: "./log"
```

## State Compatibility

OpenTofu reads and writes Terraform state files natively. No state migration is required. The `.terraform` directory and `.terraform.lock.hcl` lock files continue to work as before.

## Breaking Changes

1. **Config key rename**: `terraform:` → `tofu:` in `quartz.yaml`. Existing configs must be updated.
2. **Stage path**: Default stage discovery path changed from `terraform/stages/` to `tofu/stages/`. Infrastructure repos must rename their `terraform/` directory.
3. **Minimum version**: OpenTofu 1.11.6 is the minimum supported version.

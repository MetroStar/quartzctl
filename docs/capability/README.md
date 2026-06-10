# quartzctl - Capability Package

Reusable lifecycle automation for staged OpenTofu platforms, delivered as the `quartz` CLI.

`quartzctl` turns a top-level `quartz.yaml` plus ordered stage directories into a repeatable install, update, validation, and teardown workflow. It discovers stages, resolves dependencies, renders configuration and secrets, prepares cloud state backends, runs OpenTofu per stage, gates progress with health checks, and provides recovery commands for state, locks, kubeconfig, secrets, and workload restarts.

This directory contains the documentation package for delivery teams, capture teams, and proposal writers.

## Quick Start

**New to this?** Start with the [Onboarding Guide](onboarding-guide.md) and the [Template Package](template-package.md).

**Evaluating for a capture?** Start with the [Capability Summary](capability-summary.md) and [Value Brief](value-brief.md).

## Capability Assets

| Document | Description | Audience |
|----------|-------------|----------|
| [Capability Summary](capability-summary.md) | Self-contained one-pager suitable for email distribution | Delivery leads, capture managers |
| [Reference Architecture](reference-architecture.md) | CLI architecture, stage lifecycle, command surface, provider boundaries | Solutions architects, platform engineers |
| [Operating Model](operating-model.md) | Day 0/1/2+ ownership, RACI, release and operations model | Delivery teams, operations leads |
| [Onboarding Guide](onboarding-guide.md) | Prerequisites to first successful staged install | New delivery teams |
| [Runbook](runbook.md) | Operational procedures for install, resume, state repair, teardown, and local development | Operators, platform engineers |
| [Troubleshooting Guide](troubleshooting-guide.md) | Decision-tree diagnosis for config, provider, OpenTofu, health-check, and teardown issues | All practitioners |
| [Template Package](template-package.md) | `quartz.yaml` and `stage.yaml` adaptation reference | New program setup |
| [Value Brief](value-brief.md) | Delivery value, reuse argument, and proposal talking points | Capture teams, proposals |
| [Product Page](product-page.md) | Marketing-style narrative for internal or external capability pages | Marketing, capture |

## Supporting References

| Document | Description |
|----------|-------------|
| [Repository README](../../README.md) | Primary command reference and configuration examples |
| [Minimal Sample](../samples/minimal/quartz.yaml) | Small runnable configuration shape |
| [OpenTofu Migration](../OPENTOFU_MIGRATION.md) | Migration notes for Terraform to OpenTofu |
| [Config schema](../../internal/config/schema) | Authoritative Go structs for `quartz.yaml` |
| [Command package](../../internal/cmd) | Authoritative CLI command implementation |

## Maintenance

These documents reference actual commands, flags, config keys, and paths. When the CLI changes:

1. Update the command tables when `internal/cmd` adds or removes commands or flags.
2. Update config examples when structs in `internal/config/schema` change.
3. Re-check stage behavior when `internal/stages` or `internal/tofu` changes.
4. Keep examples aligned with `docs/samples/minimal`.

**Ownership**: The platform engineering team owns this package. Review quarterly or when the CLI command surface changes materially.

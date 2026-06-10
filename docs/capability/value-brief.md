# Value Brief

> Why `quartzctl` matters for delivery teams and proposals.

## Executive Summary

`quartzctl` reduces the risk and cost of standing up complex platform environments by turning multi-stage OpenTofu operations into a repeatable CLI workflow. It replaces bespoke runbooks and fragile shell glue with typed configuration, dependency-aware stage orchestration, health gates, state repair commands, and resumable install/clean flows.

For delivery teams, this means fewer handoffs, fewer manual state edits, faster onboarding, and a clearer operational contract. For proposals, it demonstrates that platform setup, recovery, and teardown are engineered capabilities rather than heroic manual effort.

## Delivery Value

| Area | Without quartzctl | With quartzctl |
|------|-------------------|----------------|
| Stage sequencing | Manual order knowledge or shell scripts | Directory order plus explicit dependencies |
| Config flow | Environment sprawl and duplicated tfvars | Rendered `quartz.yaml` with defaults and stage vars |
| Cross-stage outputs | Manual copy/paste or brittle scripts | First-class stage output references |
| Readiness gates | Operators watch logs and dashboards | Configured Kubernetes, HTTP, state, daemonset, and OIDC checks |
| Recovery | Backend surgery and tribal knowledge | `import`, `force-unlock`, `state`, resume, and clean reports |
| Teardown | Often skipped or unreliable | Reverse dependency cleanup with recoverable backend preservation |
| Onboarding | Weeks of project-specific knowledge | Standard commands and template package |

## Quantified Impact

The savings vary by program size, but typical improvements are:

- 40-80 hours saved during first-environment setup by reusing the stage orchestration contract.
- 4-16 hours saved per recovery event by using built-in state, import, lock, and resume commands.
- 1-3 days saved per new engineer by standardizing install, render, plan, info, and clean workflows.
- Lower teardown risk because failed clean operations preserve state for re-run rather than leaving the team blind.

## Risk Reduction

`quartzctl` directly reduces common delivery risks:

- **Ordering risk**: Dependencies are encoded in `stage.yaml`, not remembered by one engineer.
- **Configuration drift**: `quartz render` exposes the fully resolved config.
- **Partial failure risk**: Install checkpoints and drift-aware re-runs support recovery.
- **State risk**: CLI state repair commands reduce direct backend edits.
- **Secret leakage risk**: Secrets are modeled separately and sensitive-looking values are redacted in logs/state views.
- **Teardown risk**: Cleanup continues across stage failures and keeps the backend until all stages complete.

## Proposal Talking Points

- "We deliver a repeatable platform installer that uses OpenTofu under the hood and preserves standard IaC workflows."
- "The installer supports resumable, drift-aware runs with explicit health checks between stages."
- "Operators can repair state, import resources, release stale locks, and regenerate access without manual backend surgery."
- "The same command set supports developer laptops and CI automation."
- "Documentation includes architecture, onboarding, runbooks, troubleshooting, templates, and capture-ready value language."

## Control Alignment

| Control Theme | quartzctl Contribution |
|---------------|------------------------|
| Change management | Rendered config, stage plans, consistent command paths |
| Configuration management | Declarative `quartz.yaml` and stage contracts |
| Least privilege | Provider-specific credentials and environment-scoped access |
| Auditability | CLI output, rendered config, stage logs, cleanup reports |
| Recovery | Checkpoints, resume, state/import/lock commands |
| Secure handling | Secret separation and redaction of sensitive values |

## Differentiators

- OpenTofu-native: wraps familiar IaC instead of hiding it.
- Stage-native: designed for real platforms with ordered layers and dependencies.
- Recovery-aware: includes the operations people need after partial failure.
- Proposal-ready: ships with a documentation package delivery teams can use immediately.
- Extensible: provider and stage abstractions allow the capability to evolve without rewriting every project.

## Best Use In Proposals

Position `quartzctl` as the automation layer that makes a secure platform repeatable:

1. Infrastructure is defined in OpenTofu.
2. Stages encode the delivery order.
3. Health checks prove readiness before moving forward.
4. Operators use the same CLI for install, update, inspection, repair, and teardown.
5. Teams inherit a documented capability, not an undocumented script chain.

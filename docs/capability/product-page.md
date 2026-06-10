# Product Page

> Reusable staged infrastructure automation for delivery teams.

## quartzctl

Complex platforms are not one command because they are simple. They become one command when the order, inputs, health checks, and recovery paths have been engineered.

`quartzctl` is that engineering layer. It packages staged OpenTofu operations into the `quartz` CLI so teams can install, update, inspect, recover, and tear down complex platform environments with a consistent workflow.

## What It Does

`quartzctl` reads a declarative `quartz.yaml`, discovers ordered stage directories, resolves dependencies, prepares the remote state backend, injects stage variables, runs OpenTofu, and waits for configured health checks before moving forward.

When something fails, it gives operators recovery commands instead of sending them into backend tables and one-off scripts.

## Highlights

| Capability | Outcome |
|------------|---------|
| Multi-stage orchestration | Complex platform setup becomes repeatable |
| OpenTofu-native execution | Teams keep standard IaC workflows |
| Health gates | Install only advances when dependencies are actually ready |
| Resumable installs | Transient failures do not require starting over |
| State repair commands | Imports, lock release, state inspection, and state removal are first-class |
| Resilient cleanup | Teardown proceeds across failures and preserves recoverable state |
| Rendered config | Operators can see the exact config the CLI will use |

## Built For Real Delivery

Delivery environments have dependencies: networks before clusters, clusters before controllers, controllers before applications, identity before SSO, and secrets before workloads. `quartzctl` gives each layer a stage, each stage a contract, and each operation a repeatable command path.

Typical commands:

```bash
quartz check
quartz render --out ./out/quartz.generated.yaml
quartz install --yes
quartz info
quartz tofu plan --stage core --init
quartz tofu import --stage core ADDRESS ID --init
quartz clean --yes
```

## Who It Helps

- **Platform engineers** get a reusable orchestration framework for complex OpenTofu projects.
- **Delivery teams** get a standard onboarding, install, and recovery workflow.
- **Operations teams** get runbooks that map to actual commands.
- **Capture teams** get a documented capability that proves platform setup is automated and supportable.

## Why It Is Different

`quartzctl` does not replace OpenTofu, Kubernetes, GitOps, or cloud provider tooling. It coordinates them. That means teams can still inspect plans, manage state, and troubleshoot providers with familiar tools, while the CLI handles the repetitive lifecycle mechanics that usually become brittle custom scripts.

## Documentation Included

The capability package includes architecture, onboarding, operating model, runbook, troubleshooting, template, value brief, capability summary, and product-page documents. Teams can adopt the tool and the delivery language together.

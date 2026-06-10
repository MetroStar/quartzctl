# Runbook

> Operational procedures for projects managed by `quartzctl`.

Commands assume you are in the target project root with `quartz.yaml` present. Use `--config <path>` when operating from another directory.

## Procedure 1 - Preflight A Project

```bash
quartz check
quartz render --out ./out/quartz.generated.yaml
```

Review the rendered file for expected `name`, `dns.domain`, provider settings, stage ids, and stage paths.

## Procedure 2 - Run Or Resume A Full Install

```bash
quartz install
quartz install --yes
```

If the run failed at a known stage:

```bash
quartz install --resume-from <stage> --yes
```

If OpenTofu needs deferred actions for a specific install:

```bash
quartz install --allow-deferral --yes
```

## Procedure 3 - Plan Or Apply A Single Stage

```bash
quartz tofu plan --stage <stage> --init
quartz tofu apply --stage <stage> --init
```

Use this for targeted validation before a full `quartz install`. The `--init` flag initializes the stage backend first.

## Procedure 4 - Inspect Outputs

```bash
quartz tofu output --stage <stage> --init
```

Use outputs to confirm cross-stage values before debugging downstream failures.

## Procedure 5 - Repair Missing State With Import

When a resource exists in the cloud but is missing from OpenTofu state:

```bash
quartz tofu import --stage <stage> ADDRESS ID --init
```

Flag form is also supported:

```bash
quartz tofu import --stage <stage> --address ADDRESS --id ID --init
```

Then run:

```bash
quartz tofu plan --stage <stage>
```

## Procedure 6 - Release A Stale Lock

When OpenTofu reports a stale lock:

```bash
quartz tofu force-unlock --stage <stage> LOCK_ID --init
```

Flag form:

```bash
quartz tofu force-unlock --stage <stage> --lock-id LOCK_ID --init
```

Prefer this command to manual backend lock-table edits.

## Procedure 7 - Inspect Or Remove State Entries

```bash
quartz tofu state list --stage <stage> --init
quartz tofu state show --stage <stage> ADDRESS --init
quartz tofu state rm --stage <stage> ADDRESS --init
```

`state show` redacts sensitive values. `state rm` removes objects from state without destroying real infrastructure; use it only when the resource should no longer be managed.

## Procedure 8 - Generate Or Refresh Kubeconfig

```bash
quartz login
quartz login --out /tmp/kubeconfig
export KUBECONFIG=./out/kubeconfig
```

Aliases:

```bash
quartz kubeconfig
quartz refresh-kubeconfig
```

## Procedure 9 - Refresh External Secrets

```bash
quartz refresh-secrets
```

Alias:

```bash
quartz rs
```

Use after rotating a secret in the upstream secret manager.

## Procedure 10 - Restart Workloads

Restart all deployments, daemonsets, and statefulsets:

```bash
quartz restart
```

Restart a specific workload:

```bash
quartz restart --kind deployment --namespace <namespace> --name <name>
```

`--kind` can be repeated.

## Procedure 11 - Export Configured Kubernetes Resources

```bash
quartz export
```

Resources are selected by `export.objects` and written under `export.path`.

## Procedure 12 - Validate Or Format Stages

```bash
quartz tofu validate --stage <stage>
quartz tofu format --stage <stage>
quartz tofu format-all
```

`format` is also available as `fmt`.

## Procedure 13 - Clean / Teardown

```bash
quartz clean
quartz clean --yes
```

`clean` destroys stages in reverse dependency order. If a stage fails, cleanup continues and reports the failures. The remote backend is kept until all stage destroys succeed so operators can re-run cleanup with recoverable state.

## Procedure 14 - Build And Deploy A Local CLI

For CLI developers:

```bash
cd quartzctl
mise run test
mise run deploy
quartz --version
```

`mise run deploy` builds the local binary and atomically replaces common local install locations.

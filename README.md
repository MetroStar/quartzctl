# quartzctl

[![Go Report Card](https://goreportcard.com/badge/github.com/MetroStar/quartzctl)](https://goreportcard.com/report/github.com/MetroStar/quartzctl)
[![GoDoc](https://godoc.org/github.com/MetroStar/quartzctl?status.svg)](https://godoc.org/github.com/MetroStar/quartzctl)

## Development

1. Install [devbox](https://www.jetify.com/devbox/docs/quickstart/)
2. Run `devbox shell`
3. Run `task build`, should see something similar to the following if everything is working

```console
❯ task build
task: [build] mkdir -p ./bin
task: [build] BUILD_VERSION=v0.2.5-11-g7c4377f-dirty BUILD_DATE=1734710181 goreleaser build --clean --snapshot --single-target --output ./bin/quartz
  • skipping validate...
  • cleaning distribution directory
  • loading environment variables
  • getting and validating git state
    • git state                                      commit=7c4377f1ecb2bcb6793dd9e384044f2fe7524bd6 branch=mage_replacement current_tag=v0.2.5 previous_tag=v0.2.4 dirty=true
    • pipe skipped                                   reason=disabled during snapshot mode
  • parsing tag
  • setting defaults
  • partial
  • snapshotting
    • building snapshot...                           version=0.2.5-SNAPSHOT-7c4377f
  • running before hooks
    • running                                        hook=go mod tidy
  • ensuring distribution directory
  • setting up metadata
  • writing release metadata
  • loading go mod information
  • build prerequisites
  • building binaries
    • partial build                                  match=target=linux_amd64_v1
    • building                                       binary=dist/quartz_linux_amd64_v1/quartz
  • writing artifacts metadata
  • copying binary to "./bin/quartz"
  • build succeeded after 3s
  • thanks for using goreleaser!
task: [build] ./bin/quartz --version

 Quartz v0.2.5-11-g7c4377f-dirty
 Build Date: 2024-12-20 15:56 UTC
```

## Environment variables

| Key | Description | Default | Required |
|-----|-------------|---------|----------|
| GITHUB_USERNAME | Github username for Jenkins and ArgoCD connections | "" | yes |
| GITHUB_TOKEN | Github PAT for Jenkins and ArgoCD connections | "" | yes |
| REGISTRY_USERNAME | Ironbank or alternate private registry username for Bigbang sourced images | "" | if `mirror.enabled = false` |
| REGISTRY_PASSWORD |Ironbank or alternate private registry password for Bigbang sourced images | "" | if `mirror.enabled = false` |
| REGISTRY_EMAIL | Ironbank or alternate private registry email for Bigbang sourced images | "" | if `mirror.enabled = false` |
| CLOUDFLARE_EMAIL | Cloudflare username/email for automated DNS and certificate management | "" | if `providers.dns = "cloudflare"` |
| CLOUDFLARE_API_TOKEN | Cloudflare password for automated DNS and certificate management | "" | if `providers.dns = "cloudflare"` |

## Configuration

The [quartz.yaml](./quartz.yaml) config file is necessary as the primary source of configuration values for a Quartz environment. The following
represents only the minimal config for an AWS deployment though many other fields are available to customize as needed. Please see [quartz-sample.yaml](./docs/quartz-sample.yaml) for a complete example.

```yaml

name: <my-quartz> # required, unique name of the environment

dns:
  zone: example.com # required, dns zone of the environment

aws:
  region: us-east-1 # required, AWS region for the cloud infrastructure

```

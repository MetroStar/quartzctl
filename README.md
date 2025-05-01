# quartzctl

[![Build Status](https://github.com/MetroStar/quartzctl/actions/workflows/pr.yaml/badge.svg)](https://github.com/MetroStar/quartzctl/actions/workflows/pr.yaml)
[![codecov](https://codecov.io/gh/MetroStar/quartzctl/graph/badge.svg?token=7RIVIXH7A3)](https://codecov.io/gh/MetroStar/quartzctl)
[![GoDoc](https://godoc.org/github.com/MetroStar/quartzctl?status.svg)](https://godoc.org/github.com/MetroStar/quartzctl)
[![Go Report Card](https://goreportcard.com/badge/github.com/MetroStar/quartzctl)](https://goreportcard.com/report/github.com/MetroStar/quartzctl)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/MetroStar/quartzctl/badge)](https://scorecard.dev/viewer/?uri=github.com/MetroStar/quartzctl)
[![License](https://img.shields.io/github/license/MetroStar/quartzctl)](LICENSE)

> **quartzctl** is an open-source CLI tool designed to automate the installation and maintenance of multi-stage Terraform projects. It leverages a single YAML configuration to define stages, their dependencies, input variables, environment variables, and health checks, streamlining complex infrastructure deployments.

## Problem Statement

We were tasked with building a fully automated DevSecOps platform with [Platform One](https://p1.dso.mil/) and [Ironbank](https://registry1.dso.mil/) as its core, while minimizing installation times and risk of transient failures. A high degree of flexibility was also needed, so as to facilitate rapid development of applications and infrastructure for hackathon style environments, all while keeping an eye on security for government customers. Initially developed as a loose conglomeration of bash scripts glued together by a makefile, this eventually became unmaintainable due to increasing complexity of requirements coupled with the expectation of quick turnaround with change requests.

## About Quartz

Quartz is an open-source CLI tool designed to automate the full lifecycle of Kubernetes-based platform infrastructure with a focus on DevSecOps enablement. Originally inspired by U.S. government platform initiatives like PlatformOne and BigBang, Quartz simplifies the provisioning, configuration, and teardown of secure, GitOps-enabled environments. It leverages a top-level YAML configuration to drive installations, source secrets, and orchestrate tools like FluxCD or ArgoCD. With robust health checks, retry logic, and zero-interaction execution, Quartz empowers teams to build reproducible, production-like platforms in development or staging deployments.

## Roadmap

- Plugin framework to expand beyond AWS and Terraform
- Unwind tightly coupled assumptions of the platform (ex: separate repositories vs monorepo, use of gitops, core application stack, etc...)

---

## 📖 Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [Usage](#usage)
- [Configuration](#configuration)
- [Development](#development)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)

---

## 🚀 Features

- **Multi-Stage Management**: Define and manage multiple Terraform stages with interdependencies.
- **YAML Configuration**: Centralized configuration file specifying stages, variables, and settings.
- **Dependency Handling**: Automatically determines the order of stage execution based on dependencies.
- **Dynamic Variables**: Pass output variables from one stage as input to another.
- **Environment Management**: Set environment variables and configuration values per stage.
- **Health Checks**: Execute pre- and post-apply/destroy health checks to ensure application stability.
- **Kubernetes Integration**: Monitor Kubernetes deployment statuses and other conditions before proceeding.
- **Extensibility**: Support for additional features and integrations as needed.

---

## 🛠️ Getting Started

### Prerequisites

- [Go](https://golang.org/doc/install) version 1.24 or higher
- [Docker](https://www.docker.com/get-started) (optional, for containerized deployments)

### Installation

You can install `quartz` using one of the following methods:

#### Go Install

```bash
go install github.com/MetroStar/quartzctl@latest
```

#### Download Binary

Download the latest release from the [Releases](https://github.com/MetroStar/quartzctl/releases) page and add it to your system's PATH.

---

## 📚 Usage

```bash
quartz [command] [flags]
```

### Available Commands

- `check`: Check environment, configuration and access for installer prerequisites.
- `clean`: Perform a full cleanup/teardown of the system.
- `export`: Export configured Kubernetes resources to yaml.
- `info`: Output configuration info for the current cluster.
- `install`: Perform a full install/update of the system.
- `login`: Generate a kubeconfig for the current cluster.
- `refresh-secrets`: Trigger all external secrets to be refreshed immediately.
- `render`: Write internal configuration to yaml (For development use).
- `restart`: Restart target resource(s).
- `terraform`: Terraform subcommands for configured stages.
  - `apply`: Run `terraform apply` for a stage (`--stage <name>` required).
  - `destroy`: Run `terraform destroy` for a stage (`--stage <name>` required).
  - `format`: Run `terraform fmt` for a stage (`--stage <name>` required).
  - `format-all`: Run `terraform fmt` for all stages.
  - `init`: Run `terraform init` for a stage (`--stage <name>` required).
  - `init-all`: Run `terraform init` for all stages.
  - `output`: Run `terraform output` for a stage (`--stage <name>` required).
  - `plan`: Run `terraform plan` for a stage (`--stage <name>` required).
  - `refresh`: Run `terraform refresh` for a stage (`--stage <name>` required).
  - `refresh-all`: Run `terraform refresh` for all stages.
  - `validate`: Run `terraform validate` for a stage (`--stage <name>` required).
  - `version`: Run `terraform version`.
- `help`: Shows a list of commands or help for one command

### Global Flags

- `--config`: Path to the YAML configuration file (Optional, default: `quartz.yaml`).
- `--secrets`: Path to a YAML file containing secrets as an alternative to environment variables. For development use only (Optional).
- `--help`: Shows a list of commands or help for one command.
- `--version`: Print the version and build time.

### Example

```bash
quartz install --config=quartz.yaml
```

---

## ⚙️ Configuration

The `quartz.yaml` file defines the stages and their configurations.

### Sample Cluster Configuration (Minimal)

```yaml

name: sampleenv # unique name of quartz cluster/environment

dns: # either of domain or zone must be specified
    domain: "" # default <name>.<dns.zone>
    zone: example.com # default parsed from dns.domain

aws:
    region: us-east-1

```

The `stage.yaml` file allows for stage directories to override configuration from the cluster `quartz.yaml` or convention defaults.

### Sample Stage Configuration

```yaml

# define input variables for the terraform stage and their source
# NOTE: all stages assume the existence of a `settings` input variable that recieves the entire rendered config map unless overridden
vars:
  # input variable <my_env_val> defined in variables.tf
  my_env_val:
    # populate with an environment variable
    env: HOSTNAME
  my_secret_val:
    # populate with a value from the rendered secrets
    secret: github.token
  my_config_val:
    # populate with a value from the rendered config
    config: dns.domain
  my_stage_output_val:
    stage:
      name: previous_stage
      output: cluster.name

# health checks that determine if the dependent resources are available before or after performing an action on the stage
checks:
  # group name, only shows up in logs
  pre_install:
    # when to run the checks in this group, before/after apply/destroy
    before:
    - apply
    # define health checks derived from the state of a kubernetes resource
    kubernetes:
    - name: public-cert
      namespace: cert-manager
      kind: Certificate
      state: Ready
      timeout: 1200
    - name: istio
      kind: HelmRelease
      state: Ready
  init:
    before:
    - apply
    # explicit ordering
    order: 1
    # check the quartz global configmap for a key/value, useful for confirming one time jobs were successful (Ex. initial admin password change, database setup)
    state:
    - key: "myapp.initialized"
      value: "true"
  api:
    before:
    - apply
    order: 2
    # perform http requests against the endpoint in a loop until success or timeout
    http:
    - path: /api/system/status
      app: myapp
      content:
        json:
          key: status
        value: UP

# options for controlling what is or isn't destroyed (Ex. I'm tearing down the entire cluster, no reason to unconfigure Keycloak and waste time or risk it erroring)
# typically will only use either the include or exclude sections as the logic for using them both is messy and rarely useful
destroy:
  include:
  - "module.to_destroy"
  exclude:
  - "module.skip_destroy"

```

See the included [samples](./docs/samples/) for more details.

---

## 🧪 Development

### Setting Up the Development Environment

1. Clone the repository:

   ```bash
   git clone https://github.com/MetroStar/quartzctl.git
   cd quartzctl
   ```

2. Install [taskfile](https://taskfile.dev/installation):

   ```bash
   sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d
   ```

3. Build the application:

   ```bash
   task build
   ```

### Running Tests

```bash
task test
```

### Linting

We use [golangci-lint](https://golangci-lint.run/) for linting.

```bash
task lint
```

---

## 🔒 Security

### Reporting Vulnerabilities

If you discover a security vulnerability, please follow the guidelines in our [SECURITY.md](SECURITY.md) file.

### Security Best Practices

- **Dependencies**: We use [Dependabot](https://docs.github.com/en/code-security/supply-chain-security/keeping-your-dependencies-updated-automatically) to keep dependencies up to date.
- **CI/CD**: All commits are tested via GitHub Actions workflows.
- **Code Scanning**: Static analysis is performed using [CodeQL](https://github.com/github/codeql) and other tools.

---

## 🤝 Contributing

We welcome contributions! Please see our [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to get started.

---

## 📄 License

This project is licensed under the [Apache 2.0 License](LICENSE).

---

## 📬 Contact

For questions or support, please open an issue or contact [sblair@metrostar.com](mailto:sblair@metrostar.com).

---

## 🏆 OpenSSF Best Practices

This project aims to comply with the [OpenSSF Best Practices](https://best.openssf.org/) and has a [Scorecard](https://securityscorecards.dev/viewer/?uri=github.com/MetroStar/quartzctl) to reference.

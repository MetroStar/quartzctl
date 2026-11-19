# Contributing to quartzctl

Thank you for your interest in contributing to **quartzctl**! We welcome contributions from the community and are committed to maintaining a safe, inclusive, and high-quality open source project.

This guide outlines how to contribute effectively, responsibly, and in alignment with [CNCF](https://www.cncf.io/) and [OpenSSF](https://openssf.org/) best practices.

---

## 🧑‍💻 How to Contribute

We welcome:

- New features
- Bug fixes
- Documentation improvements
- CI/CD and security enhancements
- Test coverage and performance optimizations

Please start by searching for existing [issues](https://github.com/MetroStar/quartzctl/issues). If none exist, feel free to [open one](https://github.com/MetroStar/quartzctl/issues/new/choose) to discuss your idea before submitting a PR.

---

## 🧭 Development Environment Setup

1. **Fork and clone** the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/quartzctl.git
   cd quartzctl
   ```

2. **Install the declared toolchain** with [mise](https://mise.jdx.dev/):
   ```bash
   mise install
   ```

3. **Download dependencies** without rewriting the manifests:

   ```bash
   mise exec -- go mod download
   ```

4. **Build the CLI without release-signing credentials**:
   ```bash
   mkdir -p bin
   mise exec -- go build -o ./bin/quartz ./cmd/quartz
   ./bin/quartz --version
   ```
   On Windows, use `./bin/quartz.exe` for the output and executable.
   Development builds report development metadata, not a release version.

5. **Run tests**:
   ```bash
   mise run test
   ```

6. **Run linter (required)**:
   ```bash
   mise run lint
   ```

---

The tasks are defined in [mise.toml](mise.toml); there is no Taskfile.
`mise run build` wraps GoReleaser and attempts binary signing even in snapshot
mode. It currently needs a configured private GPG key; use the direct Go build
above for ordinary contribution. Never copy release signing keys to work around
this local-build requirement.
`mise run deploy` also replaces local installed executables and may use sudo;
it is not needed for a normal build/test workflow.

## 🧪 Pull Request (PR) Process

- **Fork the repo** and create a new branch from `main`.
- **Write clear, atomic commits**. Use conventional commit messages (see below).
- **Test locally** and ensure the code lints and passes all tests.
- **Include tests** for new functionality.
- **Update documentation** if applicable.
- **Open a pull request** against the `main` branch.

### PR Checklist

Before submitting a PR, ensure that:

- [ ] Code compiles without errors.
- [ ] Unit/integration tests cover the changes.
- [ ] `go fmt`, `go vet`, and `golangci-lint run` show no issues.
- [ ] All existing and new tests pass.
- [ ] Documentation is updated, if necessary.
- [ ] PR description clearly explains what the change does and why.

---

## ✍️ Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification:

```
<type>(optional scope): <description>
```

### Common types:

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation-only changes
- `style`: Changes that do not affect meaning (white-space, formatting)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to build process or auxiliary tools

### Examples:

- `feat(config): add support for multiple environments`
- `fix(apply): prevent crash when tofu is missing`
- `docs: update usage examples in README.md`

PRs that do not follow this format may be asked to rebase before being merged.

### Release workflow

Merge reviewed changes to `main` through a PR. The
[Tag Version workflow](.github/workflows/tag.yaml) runs semantic-release and
dispatches [Release](.github/workflows/release.yaml) only when a `v*` tag points
at that exact commit. Documentation-only commits may legitimately produce no
new tag and no release. An older tag on an ancestor is not a release of HEAD.

For an authorized release retry, verify the existing tag/revision and dispatch
Release **from that tag**, never from `main`:

```bash
gh workflow run release.yaml --repo MetroStar/quartzctl --ref v1.8.0
```

Replace the example with the intended existing tag. Do not retag an existing
release or rerun untagged historical failures. The release job rejects branch
refs. Confirm the new workflow result and published assets; a successful tag
workflow alone does not prove a release was published.

---

## 🔐 Security

If you discover a security vulnerability, **please do not open a public issue**. Instead, follow the responsible disclosure process defined in our [SECURITY.md](./SECURITY.md).

---

## ✅ Code of Conduct

This project follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md). By participating, you agree to uphold this standard.

---

## 📦 Licensing and DCO

All contributions are subject to the terms of the [Apache 2.0 License](./LICENSE). By submitting a pull request, you certify that:

- You have the right to submit the code or documentation.
- You release your contributions under the project license.

We follow the [Developer Certificate of Origin (DCO)](https://developercertificate.org/). Please ensure your commits are signed with `--signoff`:

```bash
git commit -s -m "feat: add new stage validation logic"
```

---

## 🙏 Thank You!

Your contributions help make `quartzctl` better for everyone. We value your input and look forward to your ideas, improvements, and suggestions.

For questions or to get involved in deeper discussions, please join or open an issue.

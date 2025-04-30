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

2. **Install dependencies**:
   ```bash
   go mod tidy
   ```

3. Install [taskfile](https://taskfile.dev/installation):

   ```bash
   sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d
   ```

4. **Build the CLI**:
   ```bash
   task build
   ```

5. **Run tests**:
   ```bash
   task test
   ```

6. **Run linter (required)**:
   ```bash
   task lint
   ```

---

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
- `fix(apply): prevent crash when terraform is missing`
- `docs: update usage examples in README.md`

PRs that do not follow this format may be asked to rebase before being merged.

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

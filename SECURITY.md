# Security Policy

Thank you for helping to keep **quartzctl** secure and trustworthy. We take security seriously and encourage responsible disclosure of any vulnerabilities you may discover.

---

## Supported Versions

| Version | Supported |
|---------|-----------|
| `main`  | ✅        |
| older   | ❌        |

We only support the latest version of the `main` branch. Older versions are not guaranteed to receive security updates.

---

## Reporting a Vulnerability

If you believe you’ve found a security vulnerability in `quartzctl`, **please do not open a GitHub issue or pull request**. Instead, report it privately by emailing:

📫 **sblair@metrostar.com**

Please include:

- A detailed description of the vulnerability.
- Steps to reproduce.
- A proof-of-concept (if applicable).
- Any known mitigations or workarounds.

Due to limited resources maintaining this project, response times cannot be guaranteed.

---

## Coordinated Disclosure Policy

We follow the principle of **responsible disclosure** and will work with you to coordinate a fix and a disclosure timeline. We request you do not publicly disclose details of the vulnerability until we have confirmed and published a patch.

We credit all researchers who responsibly disclose issues (unless you request otherwise).

---

## Security Best Practices for Contributors

All contributors are expected to:

- Follow secure coding practices (e.g., input validation, error handling).
- Avoid introducing hardcoded secrets or credentials.
- Ensure dependencies are up-to-date and do not introduce known vulnerabilities.
- Use signed commits (`git commit -s`) and follow the [CONTRIBUTING.md](./CONTRIBUTING.md) guidelines.
- Run and pass all static analysis, lint, and vulnerability scanning tools included in the CI pipeline.

---

## Dependencies and Supply Chain

`quartzctl` uses the following tools to help detect known CVEs and maintain secure dependencies:

- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
- [gosec](https://github.com/securego/gosec)
- GitHub Dependabot (enabled for this repository)

We are working toward full [OpenSSF Scorecard](https://github.com/ossf/scorecard) compliance.

---

## GPG Key & Signing

Future releases may be cryptographically signed. Details will be published here when that process is in place.

---

## Contact

For non-security issues, please use the [issue tracker](https://github.com/MetroStar/quartzctl/issues). For security-related matters, use the private disclosure process outlined above.

---

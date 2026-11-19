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
- Use DCO signoffs (`git commit -s`) and follow the [CONTRIBUTING.md](./CONTRIBUTING.md)
  guidelines. A signoff is not a cryptographic signature; use `git commit -S`
  when cryptographic commit signing is required.
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

The [release workflow](.github/workflows/release.yaml) imports the maintainers'
GPG key. [GoReleaser](.goreleaser.yaml) publishes platform archives, checksums,
SBOMs and detached signatures. Download the archive for your OS/architecture,
not a signature asset that happens to resemble a binary name.

1. Select the reviewed release tag and its corresponding archive.
2. Obtain and verify the maintainer signing-key fingerprint through a trusted
   channel; do not trust an arbitrary key merely because it accompanies a download.
3. Use `gpg --verify <signature> <artifact>` on the checksum file and archive,
   using the actual asset names from that release.
4. Compare the archive's SHA-256 with the authenticated checksum entry, then
   extract the executable.

A matching unsigned checksum only detects corruption, not authenticity.
If the signing key cannot be authenticated, record that limitation rather than
claiming a verified release signature. Checksums/signatures also do not replace
review of the release revision or vulnerability findings.

---

## Contact

For non-security issues, please use the [issue tracker](https://github.com/MetroStar/quartzctl/issues). For security-related matters, use the private disclosure process outlined above.

---

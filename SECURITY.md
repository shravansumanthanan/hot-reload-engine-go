# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub Issues.**

To report a security issue, please email the maintainer directly or use
GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
feature.

Include the following in your report:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Suggested fix (if any)

You will receive a response within **72 hours**, and a patch within **7 days**
for critical issues.

## Security Considerations for Users

### Shell Command Injection
`hotreload` executes the values of `--build` and `--exec` (and their config-file
equivalents) via the system shell (`sh -c` on Unix, `cmd /c` on Windows).

> **Warning**: Never share or commit a `.hotreload.yaml` file that contains
> secrets, tokens, or commands from untrusted sources. A compromised config file
> is equivalent to arbitrary code execution on the developer's machine.

### Threat Model
This tool is designed for **local development use only**. It:
- Runs without authentication or TLS.
- Exposes an HTTP proxy and SSE endpoint on localhost.
- Is **not intended for production or multi-user environments**.

### Recommendations
- Add `.hotreload.yaml` to `.gitignore` (it is by default).
- Run hotreload only on trusted development machines.
- Do not expose the proxy ports to untrusted networks.

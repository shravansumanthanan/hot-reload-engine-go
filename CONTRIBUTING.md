# Contributing to hotreload

Thank you for your interest in contributing! Whether it's a bug report, a feature idea, or a pull request — all contributions are welcome.

---

## Getting Started

**New to the project?** Start here:

- 🐛 Browse [`good first issue`](https://github.com/shravansumanthanan/hot-reload-engine-go/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) issues — they are specifically scoped for first-time contributors.
- 💬 Start or join a [Discussion](https://github.com/shravansumanthanan/hot-reload-engine-go/discussions) if you have a design question or want to propose something bigger before writing code.

---

## How to Contribute

### Reporting Bugs

Open a [GitHub Issue](https://github.com/shravansumanthanan/hot-reload-engine-go/issues/new) and include:

- A clear description of the problem
- Steps to reproduce
- Expected vs. actual behaviour
- Your OS, Go version, and `hotreload` version (`hotreload --version`)

### Proposing Features

Open a [GitHub Discussion](https://github.com/shravansumanthanan/hot-reload-engine-go/discussions) before opening a PR for significant changes. This avoids wasted effort if the direction isn't a fit.

### Submitting a Pull Request

1. **Fork** the repository and create your branch from `main`.
2. **Write tests** for any new behaviour. PRs that add features without tests will not be merged.
3. **Run the full test suite** with the race detector before pushing:
   ```bash
   make test-race
   ```
4. **Ensure code is formatted**:
   ```bash
   gofmt -w .
   ```
5. **Run the linter** and resolve any issues:
   ```bash
   make lint
   ```
6. **Update `CHANGELOG.md`** under the `[Unreleased]` section to describe your change.
7. Open a PR using the provided [pull request template](.github/PULL_REQUEST_TEMPLATE.md).

---

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep functions focused and small — prefer many small functions over one large one.
- All exported symbols must have a GoDoc comment.
- Avoid committing binary files, log files (`*.log`), or secrets.
- Do not commit `.hotreload.yaml` files — they may contain machine-specific or sensitive values.

---

## Branch Conventions

| Branch | Purpose |
|---|---|
| `main` | Stable, released code |
| `develop` | Integration branch for in-progress work |
| `feature/<name>` | New features |
| `fix/<name>` | Bug fixes |
| `docs/<name>` | Documentation-only changes |

Open PRs against `main` for bug fixes and `develop` for new features, unless otherwise discussed.

---

## PR Checklist

Before submitting, confirm:

- [ ] `make test-race` passes locally
- [ ] `make lint` passes
- [ ] New behaviour is covered by tests
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
- [ ] No binary files, logs, or secrets committed
- [ ] GoDoc comments added for any new exported symbols

---

## Security Issues

Please **do not** report security vulnerabilities through public GitHub Issues. See [SECURITY.md](SECURITY.md) for the responsible disclosure process.

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

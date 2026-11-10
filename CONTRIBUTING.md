# Contributing

Thank you for considering contributing to VoiceOTP Gateway.

## How to contribute

1. Fork the repository.
2. Create a feature branch from `main`.
3. Make your changes.
4. Run the checks: `go test ./...`, `go vet ./...`, `go build`, `php -l` on all PHP files.
5. Submit a pull request.

## Guidelines

- Keep changes minimal and within the defined scope.
- Do not add third-party dependencies to the Go gateway (standard library only).
- Do not add third-party PHP libraries to the WordPress plugin.
- Never commit credentials or real configuration.
- Update documentation alongside code changes.

## Code of Conduct

Be respectful and constructive. Report harassment to the maintainers.

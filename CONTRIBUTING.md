# Contributing to shadowsend-cli

Thanks for your interest in contributing. Here's how to get started.

## Development Setup

1. Install [Go 1.21+](https://go.dev/dl/)
2. Clone the repo:
   ```sh
   git clone https://github.com/pankajsattadhish/shadowsend-cli.git
   cd shadowsend-cli
   ```
3. Build:
   ```sh
   go build -o shadowsend .
   ```
4. Run:
   ```sh
   ./shadowsend --help
   ```

## Testing against local server

Set the `SHADOWSEND_API_URL` environment variable to point to your local Next.js dev server:

```sh
export SHADOWSEND_API_URL=http://localhost:3000
./shadowsend send test.txt
```

## Project Structure

```
cmd/              Command handlers (send, get, root)
internal/
  api/            HTTP client for shadowsend API
  crypto/         AES-256-GCM encryption (compatible with web app)
  ui/             Terminal output, colors, branding
main.go           Entry point
```

## Pull Requests

1. Fork the repo and create your branch from `main`
2. Make your changes
3. Ensure `go build` and `go vet` pass
4. Write a clear PR description explaining what and why
5. Keep PRs focused — one feature or fix per PR

## Code Style

- Follow standard Go conventions (`gofmt`)
- Keep functions small and focused
- Error messages should use the UPPER_SNAKE_CASE style (e.g., `ENCRYPTION_FAILED`)
- UI output goes to stderr, data (URLs) goes to stdout

## Reporting Bugs

Use the [Bug Report](https://github.com/pankajsattadhish/shadowsend-cli/issues/new?template=bug_report.md) issue template.

## Requesting Features

Use the [Feature Request](https://github.com/pankajsattadhish/shadowsend-cli/issues/new?template=feature_request.md) issue template.

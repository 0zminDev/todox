# Contributing to TodoX

Thanks for your interest in contributing! Here's a short guide.

## Before you start

- For larger changes, open an issue first describing what and why — this saves time in case something's already in progress or doesn't fit the project's direction.
- For small fixes (typos, bugfixes) feel free to open a pull request directly.

## Local setup

Requires Go 1.26+.

```bash
git clone https://github.com/0zminDev/todox.git
cd todox
go run ./cmd/todox
```

The app starts on `http://localhost:8080`; the `todos.db` file is created automatically in the project directory.

## Project structure

```
cmd/todox/          entry point (main)
internal/server/    HTTP logic: routing, sessions, handlers, SQLite access
templates/           HTML templates (Go templates + HTMX)
static/              CSS
```

## Before submitting a PR

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .   # should print nothing
```

## Commits and PRs

- Commits should be atomic — one change, one commit, with a descriptive message.
- PRs target `master`.
- CI (`go build`, `go vet`, `go test`) must pass before merging.

## Reporting bugs / proposals

Use the templates in `.github/ISSUE_TEMPLATE/`. The more specific (steps to reproduce, expected vs. actual behavior), the faster it'll get addressed.

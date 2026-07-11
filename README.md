# TodoX

[![CI](https://github.com/0zminDev/todox/actions/workflows/ci.yml/badge.svg)](https://github.com/0zminDev/todox/actions/workflows/ci.yml)
[![Deploy](https://github.com/0zminDev/todox/actions/workflows/deploy.yml/badge.svg)](https://github.com/0zminDev/todox/actions/workflows/deploy.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)

A simple Todo app: Go + HTMX + SQLite, with registration/login and a private todo list per user.

## Table of contents

- [Features](#features)
- [Project structure](#project-structure)
- [Running locally](#running-locally)
- [Running with Docker](#running-with-docker)
- [Deploying to Fly.io](#deploying-to-flyio)
- [CI/CD](#cicd)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Features

- Registration, login, logout — sessions stored in SQLite, passwords hashed with bcrypt
- A separate, isolated todo list per user
- Add / toggle / delete todos without a page reload (HTMX)
- User profile: change name and password
- Landing page for signed-out visitors

## Project structure

```
cmd/todox/          entry point (func main)
internal/server/    routing, sessions/auth, HTTP handlers, SQLite access
templates/           HTML templates (html/template + HTMX)
static/              CSS
.github/workflows/   CI (build/vet/test) and CD (deploy to Fly.io)
```

## Running locally

Requires Go 1.26+.

```bash
go run ./cmd/todox
```

The app starts on `http://localhost:8080`. The `todos.db` SQLite file is created automatically in the project directory.

Environment variables (optional):
- `PORT` — HTTP port (default `8080`)
- `DB_PATH` — path to the SQLite file (default `todos.db`)

## Running with Docker

```bash
docker build -t todox .
docker run -p 8080:8080 -v todox_data:/data todox
```

## Deploying to Fly.io

1. Install `flyctl` and log in:
   ```bash
   curl -L https://fly.io/install.sh | sh
   fly auth login
   ```

2. Create the app (the name and region in `fly.toml` must be globally unique — change them if taken):
   ```bash
   fly launch --no-deploy
   ```
   (confirm the existing `fly.toml`, don't overwrite it)

3. Create a volume for the SQLite database (region must match `primary_region` in `fly.toml`):
   ```bash
   fly volumes create todox_data --region waw --size 1
   ```

4. First manual deploy (to confirm everything works):
   ```bash
   fly deploy
   ```

5. Automatic deploys via GitHub Actions — generate a token and add it as a repo secret:
   ```bash
   fly tokens create deploy
   ```
   Add the **entire** output (including the `FlyV1 ` prefix) as `FLY_API_TOKEN` in **Settings → Secrets and variables → Actions** on GitHub.

From then on, every push to `master` (once CI passes) automatically deploys the app to Fly.io.

## CI/CD

- `.github/workflows/ci.yml` — build + vet + test on PRs and branches other than `master`
- `.github/workflows/deploy.yml` — build + deploy to Fly.io on push to `master`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for local setup, commit/PR expectations, and how to report issues. Contributors are expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

Report vulnerabilities per [SECURITY.md](SECURITY.md) — not as a public issue.

## License

[MIT](LICENSE)

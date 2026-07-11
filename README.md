# TodoX

[![CI](https://github.com/0zminDev/todox/actions/workflows/ci.yml/badge.svg)](https://github.com/0zminDev/todox/actions/workflows/ci.yml)
[![Deploy](https://github.com/0zminDev/todox/actions/workflows/deploy.yml/badge.svg)](https://github.com/0zminDev/todox/actions/workflows/deploy.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)

Prosta appka Todo: Go + HTMX + SQLite, z rejestracją/logowaniem i osobną listą zadań dla każdego użytkownika.

## Spis treści

- [Funkcje](#funkcje)
- [Struktura projektu](#struktura-projektu)
- [Uruchomienie lokalnie](#uruchomienie-lokalnie)
- [Uruchomienie przez Docker](#uruchomienie-przez-docker)
- [Deploy na Fly.io](#deploy-na-flyio)
- [CI/CD](#cicd)
- [Współtworzenie](#współtworzenie)
- [Bezpieczeństwo](#bezpieczeństwo)
- [Licencja](#licencja)

## Funkcje

- Rejestracja, logowanie, wylogowanie — sesje trzymane w SQLite, hasła hashowane bcryptem
- Osobna, izolowana lista zadań dla każdego użytkownika
- Dodawanie / odznaczanie / usuwanie zadań bez przeładowania strony (HTMX)
- Profil użytkownika: zmiana imienia i hasła
- Landing page dla niezalogowanych

## Struktura projektu

```
cmd/todox/          punkt wejścia (func main)
internal/server/    routing, sesje/auth, handlery HTTP, dostęp do SQLite
templates/           szablony HTML (html/template + HTMX)
static/              CSS
.github/workflows/   CI (build/vet/test) i CD (deploy na Fly.io)
```

## Uruchomienie lokalnie

Wymaga Go 1.26+.

```bash
go run ./cmd/todox
```

Aplikacja wystartuje na `http://localhost:8080`. Baza `todos.db` tworzy się automatycznie w katalogu projektu.

Zmienne środowiskowe (opcjonalne):
- `PORT` — port HTTP (domyślnie `8080`)
- `DB_PATH` — ścieżka do pliku SQLite (domyślnie `todos.db`)

## Uruchomienie przez Docker

```bash
docker build -t todox .
docker run -p 8080:8080 -v todox_data:/data todox
```

## Deploy na Fly.io

1. Zainstaluj `flyctl` i zaloguj się:
   ```bash
   curl -L https://fly.io/install.sh | sh
   fly auth login
   ```

2. Stwórz aplikację (nazwa i region w `fly.toml` muszą być globalnie unikalne — zmień jeśli zajęte):
   ```bash
   fly launch --no-deploy
   ```
   (potwierdź istniejący `fly.toml`, nie nadpisuj go)

3. Stwórz wolumen na bazę SQLite (region musi się zgadzać z `primary_region` w `fly.toml`):
   ```bash
   fly volumes create todox_data --region waw --size 1
   ```

4. Pierwszy deploy ręczny (żeby sprawdzić, że wszystko działa):
   ```bash
   fly deploy
   ```

5. Automatyczny deploy z GitHub Actions — wygeneruj token i dodaj go jako sekret repo:
   ```bash
   fly tokens create deploy
   ```
   Dodaj **cały** wynik (razem z prefiksem `FlyV1 `) jako `FLY_API_TOKEN` w **Settings → Secrets and variables → Actions** w repo na GitHubie.

Od tej pory każdy push do `master` (po przejściu buildu w CI) automatycznie deployuje aplikację na Fly.io.

## CI/CD

- `.github/workflows/ci.yml` — build + vet + test na PR-ach i branchach innych niż `master`
- `.github/workflows/deploy.yml` — build + deploy na Fly.io przy pushu do `master`

## Współtworzenie

Zobacz [CONTRIBUTING.md](CONTRIBUTING.md) — jak skonfigurować środowisko, jakich commitów/PR-ów oczekujemy, jak zgłaszać błędy. Uczestnicy tego projektu zobowiązani są przestrzegać [Kodeksu postępowania](CODE_OF_CONDUCT.md).

## Bezpieczeństwo

Podatności zgłaszaj zgodnie z [SECURITY.md](SECURITY.md) — nie jako publiczny issue.

## Licencja

[MIT](LICENSE)

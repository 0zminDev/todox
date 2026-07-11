# TodoX

Prosta appka Todo: Go + HTMX + SQLite, z rejestracją/logowaniem i osobną listą zadań dla każdego użytkownika.

## Uruchomienie lokalnie

```bash
go run .
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

2. Stwórz aplikację (nazwa w `fly.toml` — `todox` — musi być globalnie unikalna, zmień jeśli zajęta):
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
   Dodaj wynik jako `FLY_API_TOKEN` w **Settings → Secrets and variables → Actions** w repo na GitHubie.

Od tej pory każdy push do `main` (po przejściu buildu w CI) automatycznie deployuje aplikację na Fly.io — patrz `.github/workflows/deploy.yml`.

## CI

- `.github/workflows/ci.yml` — build + vet + test na PR-ach i branchach innych niż `main`
- `.github/workflows/deploy.yml` — build + deploy na Fly.io przy pushu do `main`

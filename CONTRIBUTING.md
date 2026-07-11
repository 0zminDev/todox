# Współtworzenie TodoX

Dzięki za chęć pomocy! Krótki przewodnik, jak wnieść zmiany.

## Zanim zaczniesz

- Dla większych zmian najpierw otwórz issue i opisz co i dlaczego chcesz zrobić — zaoszczędzi to czas, jeśli okaże się że coś już jest w toku albo nie pasuje do kierunku projektu.
- Dla drobnych poprawek (literówki, bugfixy) możesz od razu otworzyć pull request.

## Środowisko lokalne

Wymagania: Go 1.26+.

```bash
git clone https://github.com/0zminDev/todox.git
cd todox
go run ./cmd/todox
```

Aplikacja wystartuje na `http://localhost:8080`, baza `todos.db` tworzy się automatycznie w katalogu projektu.

## Struktura projektu

```
cmd/todox/          punkt wejścia (main)
internal/server/    logika HTTP: routing, sesje, handlery, dostęp do SQLite
templates/          szablony HTML (Go templates + HTMX)
static/             CSS
```

## Przed wysłaniem PR-a

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .   # nie powinno nic wypisać
```

## Commity i PR-y

- Commity powinny być atomowe — jedna zmiana, jeden commit, opisowy message.
- PR-y kierujemy do `master`.
- CI (`go build`, `go vet`, `go test`) musi przechodzić przed mergem.

## Zgłaszanie błędów / propozycji

Użyj szablonów w `.github/ISSUE_TEMPLATE/`. Im więcej konkretów (kroki do odtworzenia, oczekiwane vs. rzeczywiste zachowanie), tym szybciej się tym zajmiemy.

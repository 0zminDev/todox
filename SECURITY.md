# Polityka bezpieczeństwa

## Zgłaszanie podatności

Jeśli znajdziesz lukę bezpieczeństwa w TodoX, **nie zgłaszaj jej jako publiczny issue**. Zamiast tego napisz bezpośrednio na **0zminDev@pm.me** z opisem:

- rodzaju podatności i jej potencjalnego wpływu,
- kroków potrzebnych do jej odtworzenia,
- wersji/commita, którego dotyczy.

Postaram się potwierdzić otrzymanie zgłoszenia w ciągu kilku dni i informować o postępie prac nad poprawką.

## Wspierane wersje

Projekt jest rozwijany na bieżąco na branchu `master` — poprawki bezpieczeństwa trafiają tylko do najnowszej wersji, nie ma osobnych gałęzi LTS.

## Zakres

Ten projekt to aplikacja demonstracyjna/hobbystyczna. Obszary szczególnie wrażliwe na błędy bezpieczeństwa:

- obsługa sesji i ciasteczek (`internal/server/session.go`),
- hashowanie haseł (bcrypt, `internal/server/handlers_auth.go`),
- izolacja danych między użytkownikami (zapytania SQL filtrowane po `user_id`).

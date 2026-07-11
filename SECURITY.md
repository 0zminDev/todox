# Security Policy

## Reporting a vulnerability

If you find a security vulnerability in TodoX, **please don't report it as a public issue**. Instead, email **0zminDev@pm.me** with:

- the type of vulnerability and its potential impact,
- steps to reproduce it,
- the affected version/commit.

I'll try to acknowledge the report within a few days and keep you posted on progress toward a fix.

## Supported versions

This project is developed on an ongoing basis on the `master` branch — security fixes only land on the latest version; there are no separate LTS branches.

## Scope

This is a demo/hobby project. Areas particularly sensitive to security issues:

- session and cookie handling (`internal/server/session.go`),
- password hashing (bcrypt, `internal/server/handlers_auth.go`),
- per-user data isolation (SQL queries filtered by `user_id`).

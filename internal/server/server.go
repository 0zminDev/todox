// Package server implements the TodoX HTTP application: routing, session
// authentication, and SQLite-backed persistence for users and todos.
package server

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

var (
	db  *sql.DB
	tpl *template.Template
)

// Run opens the SQLite database at dbPath, applies migrations, and serves
// the application on the given port until the process exits or ListenAndServe
// returns an error.
func Run(dbPath, port string) error {
	var err error
	db, err = sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(); err != nil {
		return err
	}

	tpl = template.Must(template.ParseGlob("templates/*.html"))

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("GET /{$}", handleLanding)

	mux.HandleFunc("GET /register", handleRegisterForm)
	mux.HandleFunc("POST /register", handleRegisterSubmit)
	mux.HandleFunc("GET /login", handleLoginForm)
	mux.HandleFunc("POST /login", handleLoginSubmit)
	mux.HandleFunc("POST /logout", handleLogout)

	mux.HandleFunc("GET /app", requireAuth(handleApp))
	mux.HandleFunc("POST /todos", requireAuth(handleCreate))
	mux.HandleFunc("GET /todos/{id}", requireAuth(handleTodoView))
	mux.HandleFunc("PUT /todos/{id}", requireAuth(handleUpdate))
	mux.HandleFunc("DELETE /todos/{id}", requireAuth(handleDelete))
	mux.HandleFunc("GET /todos/{id}/edit", requireAuth(handleTodoEditForm))
	mux.HandleFunc("POST /todos/{id}/toggle", requireAuth(handleToggle))

	mux.HandleFunc("GET /profile", requireAuth(handleProfile))
	mux.HandleFunc("POST /profile", requireAuth(handleProfileUpdate))
	mux.HandleFunc("POST /profile/password", requireAuth(handlePasswordUpdate))

	log.Println("listening on http://localhost:" + port)
	return http.ListenAndServe(":"+port, mux)
}

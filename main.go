package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"
)

var (
	db  *sql.DB
	tpl *template.Template
)

func main() {
	dbPath := envOr("DB_PATH", "todos.db")
	port := envOr("PORT", "8080")

	var err error
	db, err = sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrate(); err != nil {
		log.Fatal(err)
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
	mux.HandleFunc("POST /todos/{id}/toggle", requireAuth(handleToggle))
	mux.HandleFunc("DELETE /todos/{id}", requireAuth(handleDelete))

	mux.HandleFunc("GET /profile", requireAuth(handleProfile))
	mux.HandleFunc("POST /profile", requireAuth(handleProfileUpdate))
	mux.HandleFunc("POST /profile/password", requireAuth(handlePasswordUpdate))

	log.Println("listening on http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

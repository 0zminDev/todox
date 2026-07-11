package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type ctxKey string

const userCtxKey ctxKey = "user"

const (
	sessionCookieName = "todox_session"
	sessionDuration    = 7 * 24 * time.Hour
)

type User struct {
	ID    int64
	Email string
	Name  string
}

type Todo struct {
	ID   int64
	Text string
	Done bool
}

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

func migrate() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			email         TEXT UNIQUE NOT NULL,
			name          TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS todos (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			text    TEXT NOT NULL,
			done    INTEGER NOT NULL DEFAULT 0
		);
	`)
	return err
}

// ---------- auth helpers ----------

func genToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func createSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := genToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(sessionDuration)
	if _, err := db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt.Unix()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func userFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	var u User
	var expiresAt int64
	row := db.QueryRow(`
		SELECT u.id, u.email, u.name, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, cookie.Value)
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &expiresAt); err != nil {
		return nil, err
	}
	if time.Now().Unix() > expiresAt {
		return nil, errors.New("session expired")
	}
	return &u, nil
}

func currentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userCtxKey).(*User)
	return u
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := userFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next(w, r.WithContext(ctx))
	}
}

// ---------- public pages ----------

func handleLanding(w http.ResponseWriter, r *http.Request) {
	if u, err := userFromRequest(r); err == nil && u != nil {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	render(w, "landing.html", nil)
}

func handleRegisterForm(w http.ResponseWriter, r *http.Request) {
	if u, err := userFromRequest(r); err == nil && u != nil {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	render(w, "register.html", map[string]any{})
}

func handleRegisterSubmit(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	name := r.FormValue("name")
	password := r.FormValue("password")
	password2 := r.FormValue("password2")

	data := map[string]any{"Email": email, "Name": name}

	if email == "" || name == "" || password == "" {
		data["Error"] = "Wypełnij wszystkie pola."
		render(w, "register.html", data)
		return
	}
	if password != password2 {
		data["Error"] = "Hasła nie są takie same."
		render(w, "register.html", data)
		return
	}
	if len(password) < 8 {
		data["Error"] = "Hasło musi mieć co najmniej 8 znaków."
		render(w, "register.html", data)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := db.Exec(`INSERT INTO users (email, name, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		email, name, string(hash), time.Now().Unix())
	if err != nil {
		data["Error"] = "Ten adres e-mail jest już zarejestrowany."
		render(w, "register.html", data)
		return
	}
	userID, _ := res.LastInsertId()

	if err := createSession(w, r, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if u, err := userFromRequest(r); err == nil && u != nil {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	render(w, "login.html", map[string]any{})
}

func handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	var userID int64
	var hash string
	row := db.QueryRow(`SELECT id, password_hash FROM users WHERE email = ?`, email)
	err := row.Scan(&userID, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		render(w, "login.html", map[string]any{
			"Email": email,
			"Error": "Nieprawidłowy e-mail lub hasło.",
		})
		return
	}

	if err := createSession(w, r, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		db.Exec(`DELETE FROM sessions WHERE token = ?`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------- profile ----------

func handleProfile(w http.ResponseWriter, r *http.Request) {
	render(w, "profile.html", map[string]any{"User": currentUser(r)})
}

func handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := r.FormValue("name")
	data := map[string]any{"User": u}

	if name == "" {
		data["Error"] = "Nazwa nie może być pusta."
		render(w, "profile.html", data)
		return
	}
	if _, err := db.Exec(`UPDATE users SET name = ? WHERE id = ?`, name, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	u.Name = name
	data["Saved"] = true
	render(w, "profile.html", data)
}

func handlePasswordUpdate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	current := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	newPass2 := r.FormValue("new_password2")
	data := map[string]any{"User": u}

	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		data["PasswordError"] = "Aktualne hasło jest nieprawidłowe."
		render(w, "profile.html", data)
		return
	}
	if len(newPass) < 8 {
		data["PasswordError"] = "Nowe hasło musi mieć co najmniej 8 znaków."
		render(w, "profile.html", data)
		return
	}
	if newPass != newPass2 {
		data["PasswordError"] = "Nowe hasła nie są takie same."
		render(w, "profile.html", data)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data["PasswordSaved"] = true
	render(w, "profile.html", data)
}

// ---------- todos (per-user) ----------

func fetchTodos(userID int64) ([]Todo, error) {
	rows, err := db.Query(`SELECT id, text, done FROM todos WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Text, &t.Done); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func handleApp(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	todos, err := fetchTodos(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "app.html", map[string]any{"User": u, "Todos": todos})
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	text := r.FormValue("text")
	if text == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	res, err := db.Exec(`INSERT INTO todos (user_id, text, done) VALUES (?, ?, 0)`, u.ID, text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	t := Todo{ID: id, Text: text, Done: false}
	if err := tpl.ExecuteTemplate(w, "todo-item-new.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleToggle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`UPDATE todos SET done = NOT done WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var t Todo
	row := db.QueryRow(`SELECT id, text, done FROM todos WHERE id = ? AND user_id = ?`, id, u.ID)
	if err := row.Scan(&t.ID, &t.Text, &t.Done); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := tpl.ExecuteTemplate(w, "todo-item.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`DELETE FROM todos WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos WHERE user_id = ?`, u.ID).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		tpl.ExecuteTemplate(w, "empty-state.html", nil)
	}
}

// ---------- render helper ----------

func render(w http.ResponseWriter, name string, data any) {
	if err := tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

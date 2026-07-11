package server

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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

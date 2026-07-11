package main

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

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

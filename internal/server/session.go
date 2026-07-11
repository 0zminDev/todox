package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

const (
	sessionCookieName = "todox_session"
	sessionDuration   = 7 * 24 * time.Hour
)

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
	if _, err := db.Exec(`UPDATE users SET last_ip = ? WHERE id = ?`, clientIP(r), userID); err != nil {
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
		SELECT u.id, u.email, u.name, u.is_admin, u.banned, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ?`, cookie.Value)
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &u.Banned, &expiresAt); err != nil {
		return nil, err
	}
	if time.Now().Unix() > expiresAt {
		return nil, errors.New("session expired")
	}
	if u.Banned {
		db.Exec(`DELETE FROM sessions WHERE token = ?`, cookie.Value)
		return nil, errors.New("account banned")
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

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

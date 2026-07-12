package server

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// anonymizeUser implements a GDPR-style "right to erasure": it scrubs every
// personally identifying field on the user's row rather than deleting the
// row outright, so historical admin stats/audit references to this user id
// stay intact, and deletes all of their content. Every session is destroyed
// so any device signed in as this account is immediately logged out.
func anonymizeUser(userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	unusablePassword, err := randomUnusablePasswordHash()
	if err != nil {
		return err
	}
	anonymizedEmail := fmt.Sprintf("deleted-user-%d@anonymized.local", userID)

	if _, err := tx.Exec(`
		UPDATE users
		SET email = ?, name = ?, password_hash = ?, last_ip = '', is_admin = 0, deleted_at = ?
		WHERE id = ?`,
		anonymizedEmail, "Deleted user", unusablePassword, time.Now().Unix(), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM todos WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM lists WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM boards WHERE user_id = ?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}

// randomUnusablePasswordHash produces a valid bcrypt hash of a random value
// nobody knows, so the anonymized account can never be logged into again
// even though its password_hash column still passes NOT NULL / looks valid.
func randomUnusablePasswordHash() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(raw)), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

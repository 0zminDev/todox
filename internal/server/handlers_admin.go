package server

import (
	"net/http"
	"strconv"
	"time"
)

func fetchStats() (Stats, error) {
	var s Stats
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE banned = 1`).Scan(&s.BannedUsers); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE deleted_at IS NOT NULL`).Scan(&s.DeletedUsers); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE date(created_at, 'unixepoch') = date('now')`).Scan(&s.NewUsersToday); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM banned_ips`).Scan(&s.BannedIPs); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos`).Scan(&s.TotalTodos); err != nil {
		return s, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos WHERE done = 1`).Scan(&s.DoneTodos); err != nil {
		return s, err
	}
	s.PendingTodos = s.TotalTodos - s.DoneTodos
	return s, nil
}

func fetchAllUsers() ([]AdminUserRow, error) {
	rows, err := db.Query(`
		SELECT u.id, u.email, u.name, u.created_at, u.last_ip, u.is_admin, u.banned,
		       (u.deleted_at IS NOT NULL), COUNT(t.id) AS todo_count
		FROM users u
		LEFT JOIN todos t ON t.user_id = u.id
		GROUP BY u.id
		ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []AdminUserRow
	for rows.Next() {
		var u AdminUserRow
		var createdAt int64
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &createdAt, &u.LastIP, &u.IsAdmin, &u.Banned, &u.Deleted, &u.TodoCount); err != nil {
			return nil, err
		}
		u.CreatedAt = time.Unix(createdAt, 0)
		users = append(users, u)
	}
	return users, rows.Err()
}

func fetchBannedIPs() ([]BannedIP, error) {
	rows, err := db.Query(`SELECT ip, reason, created_at FROM banned_ips ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []BannedIP
	for rows.Next() {
		var b BannedIP
		var createdAt int64
		if err := rows.Scan(&b.IP, &b.Reason, &createdAt); err != nil {
			return nil, err
		}
		b.CreatedAt = time.Unix(createdAt, 0)
		ips = append(ips, b)
	}
	return ips, rows.Err()
}

func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := fetchStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	users, err := fetchAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bannedIPs, err := fetchBannedIPs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	render(w, "admin.html", map[string]any{
		"User":      currentUser(r),
		"Stats":     stats,
		"Users":     users,
		"BannedIPs": bannedIPs,
	})
}

func handleBanUser(w http.ResponseWriter, r *http.Request) {
	admin := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if id == admin.ID {
		http.Error(w, "You can't ban yourself.", http.StatusBadRequest)
		return
	}

	var lastIP string
	if err := db.QueryRow(`SELECT last_ip FROM users WHERE id = ?`, id).Scan(&lastIP); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if _, err := db.Exec(`UPDATE users SET banned = 1 WHERE id = ?`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := db.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if lastIP != "" {
		if _, err := db.Exec(`
			INSERT INTO banned_ips (ip, reason, banned_by, created_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(ip) DO NOTHING`,
			lastIP, "banned via user #"+strconv.FormatInt(id, 10), admin.ID, time.Now().Unix()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func handleUnbanUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if _, err := db.Exec(`UPDATE users SET banned = 0 WHERE id = ?`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	admin := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if id == admin.ID {
		http.Error(w, "Use your profile page to delete your own account.", http.StatusBadRequest)
		return
	}

	if err := anonymizeUser(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func handleUnbanIP(w http.ResponseWriter, r *http.Request) {
	ip := r.FormValue("ip")
	if ip == "" {
		http.Error(w, "missing ip", http.StatusBadRequest)
		return
	}
	if _, err := db.Exec(`DELETE FROM banned_ips WHERE ip = ?`, ip); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

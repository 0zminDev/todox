package server

import (
	"database/sql"
	"net"
	"net/http"
	"strings"
)

// clientIP extracts the real client IP, preferring proxy headers set by
// Fly.io (Fly-Client-IP) or a generic reverse proxy (X-Forwarded-For) over
// the raw connection address, which behind a proxy would just be the proxy.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isIPBanned(ip string) (bool, error) {
	var exists int
	err := db.QueryRow(`SELECT 1 FROM banned_ips WHERE ip = ?`, ip).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// banGuard blocks requests from a banned IP before routing, so it applies
// equally to unauthenticated requests (e.g. registering a new account from
// the same address). A request carrying a valid session for a non-banned
// user bypasses the IP check — otherwise an admin could accidentally lock
// themselves (or other legitimate users on a shared/NAT'd IP) out by banning
// someone who happens to share their address. The banned user's own session
// is already deleted at ban time, so this doesn't let them back in.
func banGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, err := userFromRequest(r); err == nil && u != nil {
			next.ServeHTTP(w, r)
			return
		}

		banned, err := isIPBanned(clientIP(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if banned {
			http.Error(w, "Access denied.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

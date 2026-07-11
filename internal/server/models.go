package server

import "time"

type ctxKey string

const userCtxKey ctxKey = "user"

type User struct {
	ID      int64
	Email   string
	Name    string
	IsAdmin bool
	Banned  bool
	Deleted bool
}

// AdminUserRow is a row in the admin user listing — a wider view of a user
// than the auth-scoped User struct above.
type AdminUserRow struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
	LastIP    string
	IsAdmin   bool
	Banned    bool
	Deleted   bool
	TodoCount int
}

type BannedIP struct {
	IP        string
	Reason    string
	CreatedAt time.Time
}

type Stats struct {
	TotalUsers    int
	NewUsersToday int
	BannedUsers   int
	DeletedUsers  int
	BannedIPs     int
	TotalTodos    int
	DoneTodos     int
	PendingTodos  int
}

type Todo struct {
	ID          int64
	Text        string
	Description string
	DueDate     string // YYYY-MM-DD, empty if unset
	Tag         string
	Done        bool
	Overdue     bool // computed: DueDate is in the past and Done is false
}

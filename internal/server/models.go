package server

type ctxKey string

const userCtxKey ctxKey = "user"

type User struct {
	ID    int64
	Email string
	Name  string
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

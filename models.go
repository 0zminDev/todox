package main

type ctxKey string

const userCtxKey ctxKey = "user"

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

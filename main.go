package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"

	_ "modernc.org/sqlite"
)

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
	var err error
	db, err = sql.Open("sqlite", "file:todos.db?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			text TEXT NOT NULL,
			done INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		log.Fatal(err)
	}

	tpl = template.Must(template.ParseGlob("templates/*.html"))

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("POST /todos", handleCreate)
	mux.HandleFunc("POST /todos/{id}/toggle", handleToggle)
	mux.HandleFunc("DELETE /todos/{id}", handleDelete)

	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func fetchTodos() ([]Todo, error) {
	rows, err := db.Query(`SELECT id, text, done FROM todos ORDER BY id DESC`)
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

func handleIndex(w http.ResponseWriter, r *http.Request) {
	todos, err := fetchTodos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tpl.ExecuteTemplate(w, "index.html", todos); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	if text == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	res, err := db.Exec(`INSERT INTO todos (text, done) VALUES (?, 0)`, text)
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`UPDATE todos SET done = NOT done WHERE id = ?`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var t Todo
	row := db.QueryRow(`SELECT id, text, done FROM todos WHERE id = ?`, id)
	if err := row.Scan(&t.ID, &t.Text, &t.Done); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := tpl.ExecuteTemplate(w, "todo-item.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`DELETE FROM todos WHERE id = ?`, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos`).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		tpl.ExecuteTemplate(w, "empty-state.html", nil)
	}
}

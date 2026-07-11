package server

import (
	"net/http"
	"strconv"
)

func fetchTodos(userID int64) ([]Todo, error) {
	rows, err := db.Query(`SELECT id, text, done FROM todos WHERE user_id = ? ORDER BY id DESC`, userID)
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

func handleApp(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	todos, err := fetchTodos(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "app.html", map[string]any{"User": u, "Todos": todos})
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	text := r.FormValue("text")
	if text == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	res, err := db.Exec(`INSERT INTO todos (user_id, text, done) VALUES (?, ?, 0)`, u.ID, text)
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
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`UPDATE todos SET done = NOT done WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var t Todo
	row := db.QueryRow(`SELECT id, text, done FROM todos WHERE id = ? AND user_id = ?`, id, u.ID)
	if err := row.Scan(&t.ID, &t.Text, &t.Done); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := tpl.ExecuteTemplate(w, "todo-item.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := db.Exec(`DELETE FROM todos WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos WHERE user_id = ?`, u.ID).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		tpl.ExecuteTemplate(w, "empty-state.html", nil)
	}
}

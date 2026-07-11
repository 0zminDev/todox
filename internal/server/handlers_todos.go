package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func isOverdue(dueDate string, done bool) bool {
	return dueDate != "" && !done && dueDate < time.Now().Format("2006-01-02")
}

func fetchTodos(userID int64) ([]Todo, error) {
	rows, err := db.Query(`SELECT id, text, description, due_date, tag, done FROM todos WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Text, &t.Description, &t.DueDate, &t.Tag, &t.Done); err != nil {
			return nil, err
		}
		t.Overdue = isOverdue(t.DueDate, t.Done)
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func fetchTodo(userID, id int64) (Todo, error) {
	var t Todo
	row := db.QueryRow(`SELECT id, text, description, due_date, tag, done FROM todos WHERE id = ? AND user_id = ?`, id, userID)
	if err := row.Scan(&t.ID, &t.Text, &t.Description, &t.DueDate, &t.Tag, &t.Done); err != nil {
		return Todo{}, err
	}
	t.Overdue = isOverdue(t.DueDate, t.Done)
	return t, nil
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
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	description := strings.TrimSpace(r.FormValue("description"))
	dueDate := strings.TrimSpace(r.FormValue("due_date"))
	tag := strings.TrimSpace(r.FormValue("tag"))

	res, err := db.Exec(`INSERT INTO todos (user_id, text, description, due_date, tag, done) VALUES (?, ?, ?, ?, ?, 0)`,
		u.ID, text, description, dueDate, tag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	t := Todo{ID: id, Text: text, Description: description, DueDate: dueDate, Tag: tag, Done: false}
	t.Overdue = isOverdue(t.DueDate, t.Done)
	if err := tpl.ExecuteTemplate(w, "todo-item-new.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTodoView(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t, err := fetchTodo(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "todo-item.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleTodoEditForm(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t, err := fetchTodo(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "todo-item-edit.html", t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	description := strings.TrimSpace(r.FormValue("description"))
	dueDate := strings.TrimSpace(r.FormValue("due_date"))
	tag := strings.TrimSpace(r.FormValue("tag"))

	if _, err := db.Exec(`UPDATE todos SET text = ?, description = ?, due_date = ?, tag = ? WHERE id = ? AND user_id = ?`,
		text, description, dueDate, tag, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := fetchTodo(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "todo-item.html", t); err != nil {
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

	t, err := fetchTodo(u.ID, id)
	if err != nil {
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

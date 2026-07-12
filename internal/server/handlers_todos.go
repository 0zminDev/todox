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

func fetchTodosInList(userID, listID int64) ([]Todo, error) {
	rows, err := db.Query(`
		SELECT id, list_id, position, text, description, due_date, tag, done
		FROM todos WHERE user_id = ? AND list_id = ? ORDER BY position ASC`, userID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.ListID, &t.Position, &t.Text, &t.Description, &t.DueDate, &t.Tag, &t.Done); err != nil {
			return nil, err
		}
		t.Overdue = isOverdue(t.DueDate, t.Done)
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func fetchTodo(userID, id int64) (Todo, error) {
	var t Todo
	row := db.QueryRow(`
		SELECT id, list_id, position, text, description, due_date, tag, done
		FROM todos WHERE id = ? AND user_id = ?`, id, userID)
	if err := row.Scan(&t.ID, &t.ListID, &t.Position, &t.Text, &t.Description, &t.DueDate, &t.Tag, &t.Done); err != nil {
		return Todo{}, err
	}
	t.Overdue = isOverdue(t.DueDate, t.Done)
	return t, nil
}

// resolveOwnedListID validates that raw is a list id owned by userID within
// boardID, falling back to (creating, if necessary) that board's default
// list when raw is missing, malformed, or points at a list from a different
// board — e.g. a stale browser tab whose add-form still has an old list_id
// cached. This fallback only ever lands inside the board the caller already
// validated (see handleCreate) — it must never guess a *different* board,
// that's handled as a loud failure one layer up.
func resolveOwnedListID(userID, boardID int64, raw string) (int64, error) {
	if raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			var owns int
			if err := db.QueryRow(`SELECT 1 FROM lists WHERE id = ? AND board_id = ? AND user_id = ?`, id, boardID, userID).Scan(&owns); err == nil {
				return id, nil
			}
		}
	}
	if err := ensureDefaultList(userID, boardID); err != nil {
		return 0, err
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM lists WHERE board_id = ? ORDER BY position ASC LIMIT 1`, boardID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
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

	boardID, err := strconv.ParseInt(r.FormValue("board_id"), 10, 64)
	if err != nil || !isOwnedBoard(u.ID, boardID) {
		// Unlike the list_id fallback below, a bad board_id must not
		// silently land the card somewhere else — bounce home instead.
		w.Header().Set("HX-Redirect", "/app")
		return
	}

	listID, err := resolveOwnedListID(u.ID, boardID, r.FormValue("list_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pos, err := appendPosition(db, "todos", "list_id", listID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := db.Exec(`
		INSERT INTO todos (user_id, list_id, position, text, description, due_date, tag, done)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		u.ID, listID, pos, text, description, dueDate, tag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	t := Todo{ID: id, ListID: listID, Position: pos, Text: text, Description: description, DueDate: dueDate, Tag: tag, Done: false}
	t.Overdue = isOverdue(t.DueDate, t.Done)
	if err := tpl.ExecuteTemplate(w, "todo-item-new.html", t); err != nil {
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

	var listID int64
	if err := db.QueryRow(`SELECT list_id FROM todos WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&listID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if _, err := db.Exec(`DELETE FROM todos WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos WHERE list_id = ?`, listID).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		tpl.ExecuteTemplate(w, "todo-empty-state.html", listID)
	}
}

// handleTodoMove persists a drag-and-drop or dropdown-driven card move. Both
// callers already moved the DOM node client-side before calling this (see
// static/board.js), so it always just writes state and returns 204 — no
// HTML is rendered here.
func handleTodoMove(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	listID, err := strconv.ParseInt(r.FormValue("list_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid list_id", http.StatusBadRequest)
		return
	}
	afterID, err := parseAfterID(r.FormValue("after_id"))
	if err != nil {
		http.Error(w, "invalid after_id", http.StatusBadRequest)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var owns int
	if err := tx.QueryRow(`SELECT 1 FROM todos WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&owns); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := tx.QueryRow(`SELECT 1 FROM lists WHERE id = ? AND user_id = ?`, listID, u.ID).Scan(&owns); err != nil {
		http.Error(w, "list not found", http.StatusNotFound)
		return
	}
	if afterID != 0 {
		if err := tx.QueryRow(`SELECT 1 FROM todos WHERE id = ? AND list_id = ? AND user_id = ?`, afterID, listID, u.ID).Scan(&owns); err != nil {
			http.Error(w, "after_id not found", http.StatusNotFound)
			return
		}
	}

	pos, err := positionBetween(tx, "todos", "list_id", listID, afterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(`UPDATE todos SET list_id = ?, position = ? WHERE id = ? AND user_id = ?`, listID, pos, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

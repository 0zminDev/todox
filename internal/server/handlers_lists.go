package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func handleApp(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if err := ensureDefaultList(u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lists, err := fetchLists(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "app.html", map[string]any{"User": u, "Lists": lists})
}

// ensureDefaultList gives a user with zero lists a starting "To Do" list and
// backfills any of their todos left over from before this feature (list_id
// IS NULL) into it. Wrapped in a transaction so two concurrent callers (e.g.
// two open tabs on first load) can't both create a duplicate default list.
func ensureDefaultList(userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM lists WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	pos, err := appendPosition(tx, "lists", "user_id", userID)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO lists (user_id, name, position, created_at) VALUES (?, ?, ?, ?)`,
		userID, "To Do", pos, time.Now().Unix())
	if err != nil {
		return err
	}
	listID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE todos SET list_id = ? WHERE user_id = ? AND list_id IS NULL`, listID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func fetchLists(userID int64) ([]List, error) {
	rows, err := db.Query(`SELECT id, name, position FROM lists WHERE user_id = ? ORDER BY position ASC`, userID)
	if err != nil {
		return nil, err
	}
	var lists []List
	for rows.Next() {
		var l List
		if err := rows.Scan(&l.ID, &l.Name, &l.Position); err != nil {
			rows.Close()
			return nil, err
		}
		lists = append(lists, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for i := range lists {
		todos, err := fetchTodosInList(userID, lists[i].ID)
		if err != nil {
			return nil, err
		}
		lists[i].Todos = todos
	}
	return lists, nil
}

func fetchList(userID, id int64) (List, error) {
	var l List
	row := db.QueryRow(`SELECT id, name, position FROM lists WHERE id = ? AND user_id = ?`, id, userID)
	if err := row.Scan(&l.ID, &l.Name, &l.Position); err != nil {
		return List{}, err
	}
	todos, err := fetchTodosInList(userID, l.ID)
	if err != nil {
		return List{}, err
	}
	l.Todos = todos
	return l, nil
}

func handleListCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	pos, err := appendPosition(db, "lists", "user_id", u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := db.Exec(`INSERT INTO lists (user_id, name, position, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, name, pos, time.Now().Unix())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	list := List{ID: id, Name: name, Position: pos}
	if err := tpl.ExecuteTemplate(w, "list-column-new.html", list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListView(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	list, err := fetchList(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "list-column.html", list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListEditForm(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	list, err := fetchList(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "list-column-edit.html", list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListUpdate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	res, err := db.Exec(`UPDATE lists SET name = ? WHERE id = ? AND user_id = ?`, name, id, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	list, err := fetchList(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "list-column.html", list); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleListDelete(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	res, err := db.Exec(`DELETE FROM lists WHERE id = ? AND user_id = ?`, id, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lists WHERE user_id = ?`, u.ID).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		tpl.ExecuteTemplate(w, "board-empty-state.html", nil)
	}
}

func handleListMove(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
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
	if err := tx.QueryRow(`SELECT 1 FROM lists WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&owns); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if afterID != 0 {
		if err := tx.QueryRow(`SELECT 1 FROM lists WHERE id = ? AND user_id = ?`, afterID, u.ID).Scan(&owns); err != nil {
			http.Error(w, "after_id not found", http.StatusNotFound)
			return
		}
	}

	pos, err := positionBetween(tx, "lists", "user_id", u.ID, afterID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(`UPDATE lists SET position = ? WHERE id = ? AND user_id = ?`, pos, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

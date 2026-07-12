package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ensureDefaultList gives a board with zero lists a starting "To Do" list
// and backfills any of the user's todos left over from before the lists
// feature (list_id IS NULL) into it. Wrapped in a transaction so two
// concurrent callers (e.g. two open tabs on first load) can't both create a
// duplicate default list.
func ensureDefaultList(userID, boardID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM lists WHERE board_id = ?`, boardID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	pos, err := appendPosition(tx, "lists", "board_id", boardID)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO lists (user_id, board_id, name, position, created_at) VALUES (?, ?, ?, ?, ?)`,
		userID, boardID, "To Do", pos, time.Now().Unix())
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

func fetchLists(userID, boardID int64) ([]List, error) {
	rows, err := db.Query(`SELECT id, board_id, name, position FROM lists WHERE user_id = ? AND board_id = ? ORDER BY position ASC`, userID, boardID)
	if err != nil {
		return nil, err
	}
	var lists []List
	for rows.Next() {
		var l List
		if err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Position); err != nil {
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
	row := db.QueryRow(`SELECT id, board_id, name, position FROM lists WHERE id = ? AND user_id = ?`, id, userID)
	if err := row.Scan(&l.ID, &l.BoardID, &l.Name, &l.Position); err != nil {
		return List{}, err
	}
	todos, err := fetchTodosInList(userID, l.ID)
	if err != nil {
		return List{}, err
	}
	l.Todos = todos
	return l, nil
}

// isOwnedBoard reports whether boardID belongs to userID.
func isOwnedBoard(userID, boardID int64) bool {
	var owns int
	return db.QueryRow(`SELECT 1 FROM boards WHERE id = ? AND user_id = ?`, boardID, userID).Scan(&owns) == nil
}

func handleListCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	boardID, err := strconv.ParseInt(r.FormValue("board_id"), 10, 64)
	if err != nil || !isOwnedBoard(u.ID, boardID) {
		// A stale tab pointing at a board that's gone (or never was theirs)
		// must not silently create the list somewhere else — bounce home
		// instead of guessing.
		w.Header().Set("HX-Redirect", "/app")
		return
	}

	pos, err := appendPosition(db, "lists", "board_id", boardID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := db.Exec(`INSERT INTO lists (user_id, board_id, name, position, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, boardID, name, pos, time.Now().Unix())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	list := List{ID: id, BoardID: boardID, Name: name, Position: pos}
	if err := tpl.ExecuteTemplate(w, "list-column-new.html", list); err != nil {
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

	var boardID int64
	if err := db.QueryRow(`SELECT board_id FROM lists WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&boardID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if _, err := db.Exec(`DELETE FROM lists WHERE id = ? AND user_id = ?`, id, u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lists WHERE board_id = ?`, boardID).Scan(&count); err != nil {
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

	var boardID int64
	if err := tx.QueryRow(`SELECT board_id FROM lists WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&boardID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if afterID != 0 {
		var owns int
		if err := tx.QueryRow(`SELECT 1 FROM lists WHERE id = ? AND board_id = ? AND user_id = ?`, afterID, boardID, u.ID).Scan(&owns); err != nil {
			http.Error(w, "after_id not found", http.StatusNotFound)
			return
		}
	}

	pos, err := positionBetween(tx, "lists", "board_id", boardID, afterID)
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

package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ensureDefaultBoard gives a user with zero boards a starting "My Board" and
// backfills any of their lists left over from before this feature (board_id
// IS NULL) into it. Wrapped in a transaction so two concurrent callers can't
// both create a duplicate default board — same shape as ensureDefaultList.
func ensureDefaultBoard(userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM boards WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	pos, err := appendPosition(tx, "boards", "user_id", userID)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO boards (user_id, name, position, created_at) VALUES (?, ?, ?, ?)`,
		userID, "My Board", pos, time.Now().Unix())
	if err != nil {
		return err
	}
	boardID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE lists SET board_id = ? WHERE user_id = ? AND board_id IS NULL`, boardID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func fetchBoards(userID int64) ([]Board, error) {
	rows, err := db.Query(`
		SELECT b.id, b.name, b.position, COUNT(l.id) AS list_count
		FROM boards b
		LEFT JOIN lists l ON l.board_id = b.id
		WHERE b.user_id = ?
		GROUP BY b.id
		ORDER BY b.position ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Name, &b.Position, &b.ListCount); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func fetchBoard(userID, id int64) (Board, error) {
	var b Board
	row := db.QueryRow(`SELECT id, name, position FROM boards WHERE id = ? AND user_id = ?`, id, userID)
	if err := row.Scan(&b.ID, &b.Name, &b.Position); err != nil {
		return Board{}, err
	}
	return b, nil
}

func handleApp(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if err := ensureDefaultBoard(u.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	boards, err := fetchBoards(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "boards.html", map[string]any{"User": u, "Boards": boards})
}

func handleBoardCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	pos, err := appendPosition(db, "boards", "user_id", u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := db.Exec(`INSERT INTO boards (user_id, name, position, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, name, pos, time.Now().Unix())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	board := Board{ID: id, Name: name, Position: pos}
	if err := tpl.ExecuteTemplate(w, "board-tile-new.html", board); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleBoardView(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	board, err := fetchBoard(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := ensureDefaultList(u.ID, board.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lists, err := fetchLists(u.ID, board.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "board.html", map[string]any{"User": u, "Board": board, "Lists": lists})
}

func handleBoardEditForm(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	board, err := fetchBoard(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := tpl.ExecuteTemplate(w, "board-tile-edit.html", board); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleBoardUpdate(w http.ResponseWriter, r *http.Request) {
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

	res, err := db.Exec(`UPDATE boards SET name = ? WHERE id = ? AND user_id = ?`, name, id, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	board, err := fetchBoard(u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM lists WHERE board_id = ?`, board.ID).Scan(&board.ListCount); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tpl.ExecuteTemplate(w, "board-tile.html", board); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBoardDelete is a plain form POST (not htmx DELETE) because deleting
// a board is a full-page navigation away, the same shape as account
// deletion in handlers_profile.go, rather than an in-page fragment swap.
func handleBoardDelete(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	res, err := db.Exec(`DELETE FROM boards WHERE id = ? AND user_id = ?`, id, u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

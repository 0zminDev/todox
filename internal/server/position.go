package server

import (
	"database/sql"
	"strconv"
)

// dbtx is satisfied by both *sql.DB and *sql.Tx, so position helpers can run
// standalone or inside a transaction.
type dbtx interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

const (
	positionGap    = 1.0
	minPositionGap = 1e-9 // below this, two float64 positions are considered a collision
)

// table/scopeCol arguments below are always Go string literals from this
// package's own call sites, never user input, so building queries with them
// via string concatenation carries no injection risk.

// appendPosition returns a position placing a new row after every existing
// row sharing scopeCol = scopeVal. Appending can never collide (the new
// value is strictly greater than the current max), so no rebalance check
// is needed here.
func appendPosition(q dbtx, table, scopeCol string, scopeVal int64) (float64, error) {
	var maxPos sql.NullFloat64
	if err := q.QueryRow(`SELECT MAX(position) FROM `+table+` WHERE `+scopeCol+` = ?`, scopeVal).Scan(&maxPos); err != nil {
		return 0, err
	}
	if !maxPos.Valid {
		return positionGap, nil
	}
	return maxPos.Float64 + positionGap, nil
}

// positionBetween returns a position placing a row immediately after the
// row with id = afterID (or at the very start of the ordering if afterID is
// 0) within the rows sharing scopeCol = scopeVal. If the two neighboring
// positions are too close to safely split, the whole scope is rebalanced to
// evenly-spaced integers first.
func positionBetween(q dbtx, table, scopeCol string, scopeVal, afterID int64) (float64, error) {
	lo, hi, hasLo, hasHi, err := neighborPositions(q, table, scopeCol, scopeVal, afterID)
	if err != nil {
		return 0, err
	}
	if hasLo && hasHi && (hi-lo) < minPositionGap {
		if err := rebalance(q, table, scopeCol, scopeVal); err != nil {
			return 0, err
		}
		lo, hi, hasLo, hasHi, err = neighborPositions(q, table, scopeCol, scopeVal, afterID)
		if err != nil {
			return 0, err
		}
	}

	switch {
	case hasLo && hasHi:
		return (lo + hi) / 2, nil
	case hasLo:
		return lo + positionGap, nil
	case hasHi:
		return hi - positionGap, nil
	default:
		return positionGap, nil
	}
}

func neighborPositions(q dbtx, table, scopeCol string, scopeVal, afterID int64) (lo, hi float64, hasLo, hasHi bool, err error) {
	if afterID != 0 {
		if err = q.QueryRow(`SELECT position FROM `+table+` WHERE id = ? AND `+scopeCol+` = ?`, afterID, scopeVal).Scan(&lo); err != nil {
			return
		}
		hasLo = true
	}

	var next sql.NullFloat64
	if hasLo {
		err = q.QueryRow(`SELECT MIN(position) FROM `+table+` WHERE `+scopeCol+` = ? AND position > ?`, scopeVal, lo).Scan(&next)
	} else {
		err = q.QueryRow(`SELECT MIN(position) FROM `+table+` WHERE `+scopeCol+` = ?`, scopeVal).Scan(&next)
	}
	if err != nil {
		return
	}
	if next.Valid {
		hi, hasHi = next.Float64, true
	}
	return
}

// rebalance rewrites every row's position in a scope to evenly-spaced
// integers, preserving current order. Cheap at this app's scale (tens of
// rows per list/board) and only runs when positionBetween detects a
// collision, which after repeated same-slot inserts takes dozens of moves.
func rebalance(q dbtx, table, scopeCol string, scopeVal int64) error {
	rows, err := q.Query(`SELECT id FROM `+table+` WHERE `+scopeCol+` = ? ORDER BY position ASC`, scopeVal)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for i, id := range ids {
		pos := float64(i+1) * 1000
		if _, err := q.Exec(`UPDATE `+table+` SET position = ? WHERE id = ?`, pos, id); err != nil {
			return err
		}
	}
	return nil
}

// parseAfterID parses the "after_id" form value used by the move endpoints:
// empty string means "insert at the start", matching positionBetween's
// afterID == 0 convention.
func parseAfterID(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

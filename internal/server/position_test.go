package server

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newPositionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, scope INTEGER NOT NULL, position REAL NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return d
}

func insertItem(t *testing.T, d *sql.DB, scope int64, pos float64) int64 {
	t.Helper()
	res, err := d.Exec(`INSERT INTO items (scope, position) VALUES (?, ?)`, scope, pos)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func positionOf(t *testing.T, d *sql.DB, id int64) float64 {
	t.Helper()
	var pos float64
	if err := d.QueryRow(`SELECT position FROM items WHERE id = ?`, id).Scan(&pos); err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestAppendPosition(t *testing.T) {
	d := newPositionTestDB(t)

	pos, err := appendPosition(d, "items", "scope", 1)
	if err != nil {
		t.Fatal(err)
	}
	if pos != positionGap {
		t.Errorf("first append = %v, want %v", pos, positionGap)
	}
	insertItem(t, d, 1, pos)

	pos2, err := appendPosition(d, "items", "scope", 1)
	if err != nil {
		t.Fatal(err)
	}
	if pos2 <= pos {
		t.Errorf("second append %v should be greater than first %v", pos2, pos)
	}

	// A different scope must not see the other scope's items.
	otherPos, err := appendPosition(d, "items", "scope", 2)
	if err != nil {
		t.Fatal(err)
	}
	if otherPos != positionGap {
		t.Errorf("append into empty scope 2 = %v, want %v", otherPos, positionGap)
	}
}

func TestPositionBetween_StartAndEnd(t *testing.T) {
	d := newPositionTestDB(t)
	aID := insertItem(t, d, 1, 10)
	bID := insertItem(t, d, 1, 20)

	// afterID = 0 means "insert at the very start" — must land before aID.
	start, err := positionBetween(d, "items", "scope", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if start >= positionOf(t, d, aID) {
		t.Errorf("start position %v should be before first item %v", start, positionOf(t, d, aID))
	}

	// afterID = last item means "append at the end" — must land after bID.
	end, err := positionBetween(d, "items", "scope", 1, bID)
	if err != nil {
		t.Fatal(err)
	}
	if end <= positionOf(t, d, bID) {
		t.Errorf("end position %v should be after last item %v", end, positionOf(t, d, bID))
	}
}

// TestPositionBetween_CollisionTriggersRebalance repeatedly inserts (then
// removes) an item in the same slot between two fixed neighbors — the
// scenario that empirically collides after 52 float64 splits of the same
// gap. On every iteration it re-reads the neighbors' current positions
// (which the rebalance guard may have just rewritten) and asserts the
// returned position is still strictly between them, well past the
// confirmed collision threshold.
func TestPositionBetween_CollisionTriggersRebalance(t *testing.T) {
	d := newPositionTestDB(t)
	aID := insertItem(t, d, 1, 1)
	bID := insertItem(t, d, 1, 2)

	for i := 0; i < 60; i++ {
		pos, err := positionBetween(d, "items", "scope", 1, aID)
		if err != nil {
			t.Fatalf("iteration %d: positionBetween: %v", i, err)
		}

		lo := positionOf(t, d, aID)
		hi := positionOf(t, d, bID)
		if pos <= lo || pos >= hi {
			t.Fatalf("iteration %d: pos %v not strictly between neighbors [%v, %v]", i, pos, lo, hi)
		}

		newID := insertItem(t, d, 1, pos)
		if _, err := d.Exec(`DELETE FROM items WHERE id = ?`, newID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRebalance_PreservesOrder(t *testing.T) {
	d := newPositionTestDB(t)
	ids := []int64{
		insertItem(t, d, 1, 0.001),
		insertItem(t, d, 1, 0.0011),
		insertItem(t, d, 1, 0.00111),
	}

	if err := rebalance(d, "items", "scope", 1); err != nil {
		t.Fatal(err)
	}

	var last float64
	for i, id := range ids {
		pos := positionOf(t, d, id)
		if i > 0 && pos <= last {
			t.Fatalf("rebalance did not preserve order: item %d position %v <= previous %v", i, pos, last)
		}
		last = pos
	}
}

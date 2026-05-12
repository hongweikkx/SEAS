package gorm

import (
	"database/sql"
	"math"
	"testing"
)

const epsilon = 1e-9

// openMem 打开一个注册了 STDDEV 聚合函数的内存 SQLite 库用于测试。
func openMem(t *testing.T) *sql.DB {
	t.Helper()
	if err := RegisterAggregates(); err != nil {
		t.Fatalf("RegisterAggregates: %v", err)
	}
	db, err := sql.Open(SeasDriverName, ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE t (x REAL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

// insertValues 向 t 表插入若干浮点值。
func insertValues(t *testing.T, db *sql.DB, vs []float64) {
	t.Helper()
	for _, v := range vs {
		if _, err := db.Exec(`INSERT INTO t(x) VALUES (?)`, v); err != nil {
			t.Fatalf("insert %v: %v", v, err)
		}
	}
}

// queryStddev 执行聚合查询并返回结果指针(NULL 时为 nil)。
func queryStddev(t *testing.T, db *sql.DB, fn string) *float64 {
	t.Helper()
	var v sql.NullFloat64
	if err := db.QueryRow(`SELECT ` + fn + `(x) FROM t`).Scan(&v); err != nil {
		t.Fatalf("query %s: %v", fn, err)
	}
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func TestStddevPop_KnownSample(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{2, 4, 4, 4, 5, 5, 7, 9})
	got := queryStddev(t, db, "STDDEV_POP")
	if got == nil {
		t.Fatalf("STDDEV_POP got NULL, want ~2.0")
	}
	if math.Abs(*got-2.0) > 1e-6 {
		t.Fatalf("STDDEV_POP = %v, want ~2.0", *got)
	}
}

func TestStddevSamp_KnownSample(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{2, 4, 4, 4, 5, 5, 7, 9})
	got := queryStddev(t, db, "STDDEV_SAMP")
	if got == nil {
		t.Fatalf("STDDEV_SAMP got NULL, want ~2.138089")
	}
	if math.Abs(*got-2.138089935) > 1e-6 {
		t.Fatalf("STDDEV_SAMP = %v, want ~2.138089935", *got)
	}
}

func TestStddev_EmptySet(t *testing.T) {
	db := openMem(t)
	if got := queryStddev(t, db, "STDDEV_POP"); got != nil {
		t.Fatalf("STDDEV_POP on empty: got %v, want NULL", *got)
	}
	if got := queryStddev(t, db, "STDDEV_SAMP"); got != nil {
		t.Fatalf("STDDEV_SAMP on empty: got %v, want NULL", *got)
	}
}

func TestStddev_SingleValue(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{5})
	pop := queryStddev(t, db, "STDDEV_POP")
	if pop == nil || math.Abs(*pop) > epsilon {
		t.Fatalf("STDDEV_POP single: got %v, want 0", pop)
	}
	if samp := queryStddev(t, db, "STDDEV_SAMP"); samp != nil {
		t.Fatalf("STDDEV_SAMP single: got %v, want NULL", *samp)
	}
}

func TestStddev_AllSame(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{3, 3, 3, 3})
	if pop := queryStddev(t, db, "STDDEV_POP"); pop == nil || math.Abs(*pop) > 1e-6 {
		t.Fatalf("STDDEV_POP all-same: got %v, want 0", pop)
	}
	if samp := queryStddev(t, db, "STDDEV_SAMP"); samp == nil || math.Abs(*samp) > 1e-6 {
		t.Fatalf("STDDEV_SAMP all-same: got %v, want 0", samp)
	}
}

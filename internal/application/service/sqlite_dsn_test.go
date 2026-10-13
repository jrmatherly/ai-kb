package service

import (
	"fmt"
	"sync/atomic"
	"testing"
)

var testMemoryDBSeq atomic.Uint64

// testMemoryDSN names a shared-cache in-memory SQLite database that belongs to
// one test run. A name built from t.Name() alone repeats under -count>1, and
// the database outlives the test because gorm keeps its connections open, so
// the second run would insert into the first run's rows.
func testMemoryDSN(t testing.TB) string {
	return fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), testMemoryDBSeq.Add(1))
}

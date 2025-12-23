package mysql

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestBuildInsert(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	id := 3
	tenant_id := "tenant-1"
	name := "Takeo"
	email := "<EMAIL>"
	created_at := "2025-12-20 10:00:00"
	deleted_at := "2025-12-20 10:00:00"
	expectedSQL := "INSERT INTO users VALUES (?, ?, ?, ?, ?, ?)"

	mock.ExpectExec(regexp.QuoteMeta(expectedSQL)).
		WithArgs(id, tenant_id, name, email, created_at, deleted_at).
		WillReturnResult(sqlmock.NewResult(3, 0))

	insVal := InsertCond{Arg: []any{id, tenant_id, name, email, created_at, deleted_at}}
	ins, err := InsertFrom("users").Values(&insVal).Exec(ctx, db)
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	t.Logf("ins: %d", ins)
}

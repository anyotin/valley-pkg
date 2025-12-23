package mysql

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestDelete(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	tenant_id := "tenant-1"
	expectedSQL := "DELETE FROM users WHERE tenant_id = ?"

	mock.ExpectExec(regexp.QuoteMeta(expectedSQL)).
		WithArgs(tenant_id).
		WillReturnResult(sqlmock.NewResult(0, 2)) // 2行更新された想定

	del, err := DeleteFrom("users").Where(Eq("tenant_id", tenant_id)).Exec(ctx, db)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	t.Logf("delete: %d", del)
}

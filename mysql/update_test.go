package mysql

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestUpdateBuilder(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	name := "Alice"
	tenant_id := "tenant-1"
	expectedSQL := "UPDATE users SET name = ? WHERE tenant_id = ?"

	mock.ExpectExec(regexp.QuoteMeta(expectedSQL)).
		WithArgs(name, tenant_id).
		WillReturnResult(sqlmock.NewResult(0, 2)) // 2行更新された想定

	upd, err := UpdateFrom[User]("users").Set(UpdateCond{"name", "Alice"}).Where(Eq("tenant_id", tenant_id)).Exec(ctx, db)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	t.Logf("upd: %d", upd)
}

func TestUpdateBuilder_Slice(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	name := "Alice"
	tenant_id := "tenant-1"
	email := "<EMAIL>"
	expectedSQL := "UPDATE users SET name = ?, email = ? WHERE tenant_id = ?"

	mock.ExpectExec(regexp.QuoteMeta(expectedSQL)).
		WithArgs(name, email, tenant_id).
		WillReturnResult(sqlmock.NewResult(0, 2)) // 2行更新された想定

	upd, err := UpdateFrom[User]("users").Set(UpdateCond{"name", "Alice"}, UpdateCond{"email", email}).Where(Eq("tenant_id", tenant_id)).Exec(ctx, db)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	t.Logf("upd: %d", upd)
}

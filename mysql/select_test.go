package mysql

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"regexp"
	"testing"
	"time"
)

type User struct {
	ID        int        `db:"id"`
	TenantID  string     `db:"tenant_id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	CreatedAt time.Time  `db:"created_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	db := sqlx.NewDb(rawDB, "mysql")

	cleanup := func() {
		_ = db.Close()
	}
	return db, mock, cleanup
}

func prepareRows() *sqlmock.Rows {
	now := time.Date(2025, 12, 20, 10, 0, 0, 0, time.UTC)

	return sqlmock.NewRows([]string{
		"id", "tenant_id", "name", "email", "created_at", "deleted_at",
	}).AddRow(
		1, "tenant-1", "Alice", "alice@example.com", now, nil,
	).AddRow(
		2, "tenant-1", "Bob", "bob@example.com", now.Add(time.Minute), nil,
	)
}

func TestSelectBuilder_Where(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	tenant_id := "tenant-1"
	name := "Alice"
	expectedSQL := "SELECT * FROM users WHERE ((tenant_id = ?) AND (tenant_id = ?)) OR (name = ?)"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WithArgs(tenant_id, tenant_id, name).
		WillReturnRows(prepareRows())

	//got, err := SelectFrom[User]("users").
	//	Where(Or(And(Eq("tenant_id", tenant_id), Eq("tenant_id", tenant_id)), Eq("name", name))).
	//	FetchAll(ctx, db)
	got, err := SelectFrom2[User]("users").
		Where(Or(And(Eq("tenant_id", tenant_id), Eq("tenant_id", tenant_id)), Eq("name", name))).
		FetchAll(ctx, db)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != 1 || got[0].Name != "Alice" {
		t.Fatalf("got[0] = %+v", got[0])
	}

	// キューに追加された期待条件がすべて順に満たされたかどうかを最終確認
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestSelectBuilder_WithoutWhere(t *testing.T) {
	ctx := context.Background()

	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	expectedSQL := "SELECT * FROM users"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WillReturnRows(prepareRows())

	got, err := SelectFrom[User]("users").FetchAll(ctx, db)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != 1 || got[0].Name != "Alice" {
		t.Fatalf("got[0] = %+v", got[0])
	}

	// キューに追加された期待条件がすべて順に満たされたかどうかを最終確認
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestSelectBuilder_OrderBy(t *testing.T) {
	ctx := context.Background()
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	tid := "tenant-1"
	expectedSQL := "SELECT * FROM users WHERE tenant_id = ? ORDER BY created_at ASC"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WithArgs(tid).
		WillReturnRows(prepareRows())

	got, err := SelectFrom[User]("users").
		Where(Eq("tenant_id", tid)).
		OrderBy(&OrderbyCond{Column: "created_at", Direction: ASC}).
		FetchAll(ctx, db)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	t.Logf("got: %+v", got)
}

func TestSelectBuilder_LimitOffset(t *testing.T) {
	ctx := context.Background()
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	tid := "tenant-1"
	expectedSQL := "SELECT id,tenant_id,name,email FROM users WHERE tenant_id = ?"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WithArgs(tid).
		WillReturnRows(prepareRows())

	got, err := SelectFrom[User]("users").
		Limit(10).Offset(10).
		FetchAll(ctx, db)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}

	t.Logf("got: %+v", got)
}

func TestSelectBuilder_Except(t *testing.T) {
	ctx := context.Background()
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	tid := "tenant-1"
	expectedSQL := "SELECT id,tenant_id,name,email FROM users WHERE tenant_id = ?"

	mock.ExpectQuery(regexp.QuoteMeta(expectedSQL)).
		WithArgs(tid).
		WillReturnRows(prepareRows())

	SetRegistry(MapRegistry{
		"users": {"id", "tenant_id", "name", "email", "created_at", "deleted_at"},
	})

	got, err := SelectFrom[User]("users").Except("created_at", "deleted_at").
		Where(Eq("tenant_id", tid)).
		FetchAll(ctx, db)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}

	t.Logf("got: %+v", got)
}

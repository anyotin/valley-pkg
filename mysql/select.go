package mysql

import (
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strconv"
	"strings"
)

var (
	ErrWhereRequired     = errors.New("where clause is required")
	ErrColumnsNotFound   = errors.New("columns registry not found for table")
	ErrExceptNeedsSchema = errors.New("except() requires registered columns for the table")
)

// ==== Column registry（Except対応用） ====

type ColumnRegistry interface {
	Columns(table string) ([]string, bool)
}

type MapRegistry map[string][]string

func (r MapRegistry) Columns(table string) ([]string, bool) {
	cols, ok := r[table]
	return cols, ok
}

// パッケージ内で使う registry
var registry ColumnRegistry = MapRegistry{}

// SetRegistry ベースのカラム設定
func SetRegistry(r ColumnRegistry) { registry = r }

// ==== WHERE強制のための phantom types ====

type WhereState interface{ isWhereState() }

type WithoutWhere struct{} // WithoutWhere Where句を所持していない状態
type WithWhere struct{}    // WithWhere Where句を所持している状態

func (WithoutWhere) isWhereState() {}
func (WithWhere) isWhereState()    {}

// ---- 共通：identifier の超最低限チェック（任意） ----
// ※本気でやるなら “テーブル名/列名は定数のみ” 運用に寄せるのが安全
func safeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '.' ||
			(r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

// ---- SELECT ----

type SelectBuilder[S any, W WhereState] struct {
	table   string
	cols    []string
	except  []string
	where   *WhereCond
	orderBy *OrderbyCond
	limit   int
	offset  int
}

type SelectBuilder2[S any] struct {
	table   string
	cols    []string
	except  []string
	where   *WhereCond
	orderBy *OrderbyCond
	limit   int
	offset  int
}
type SelectWithoutWhere[S any] struct{ b SelectBuilder2[S] }
type SelectWithWhere[S any] struct{ b SelectBuilder2[S] }

// SelectFrom は指定されたテーブル名で SelectBuilder を初期化
func SelectFrom[S any](table string) SelectBuilder[S, WithoutWhere] {
	return SelectBuilder[S, WithoutWhere]{table: table}
}

func SelectFrom2[S any](table string) SelectWithoutWhere[S] {
	return SelectWithoutWhere[S]{b: SelectBuilder2[S]{table: table}}
}

// Columns は取得するカラムを指定
func (b SelectBuilder[S, W]) Columns(cols ...string) SelectBuilder[S, W] {
	b.cols = append([]string{}, cols...)
	return b
}

func (b SelectBuilder2[S]) Columns(cols ...string) SelectBuilder2[S] {
	b.cols = append([]string{}, cols...)
	return b
}

func (b SelectBuilder[S, W]) Except(cols ...string) SelectBuilder[S, W] {
	b.except = append([]string{}, cols...)
	return b
}

func (b SelectBuilder2[S]) Except(cols ...string) SelectBuilder2[S] {
	b.except = append([]string{}, cols...)
	return b
}

// Where WHERE句の指定
func (b SelectBuilder[S, WithoutWhere]) Where(cond *WhereCond) SelectBuilder[S, WithWhere] {
	nb := SelectBuilder[S, WithWhere](b)
	nb.where = cond
	return nb
}

func (s SelectWithoutWhere[S]) Where(cond *WhereCond) SelectWithWhere[S] {
	s.b.where = cond
	return SelectWithWhere[S]{b: s.b}
}

// OrderBy OrderBy
func (b SelectBuilder[S, W]) OrderBy(cond *OrderbyCond) SelectBuilder[S, W] {
	b.orderBy = cond
	return b
}

// OrderBy OrderBy
func (b SelectBuilder2[S]) OrderBy(cond *OrderbyCond) SelectBuilder2[S] {
	b.orderBy = cond
	return b
}

// Limit 取得データ数の制限
func (b SelectBuilder[S, W]) Limit(n int) SelectBuilder[S, W] {
	b.limit = n
	return b
}

func (b SelectBuilder2[S]) Limit(n int) SelectBuilder2[S] {
	b.limit = n
	return b
}

// Offset データの取得を行う最初の位置を指定
func (b SelectBuilder[S, W]) Offset(n int) SelectBuilder[S, W] {
	b.offset = n
	return b
}

func (b SelectBuilder2[S]) Offset(n int) SelectBuilder2[S] {
	b.offset = n
	return b
}

// FetchAll 実行：複数行
//func (b SelectBuilder[S, WithWhere]) FetchAll(ctx context.Context, db *sqlx.DB) ([]S, error) {
//	q, args, err := b.build()
//	if err != nil {
//		return nil, err
//	}
//	q = db.Rebind(q)
//
//	var dest []S
//	if err := db.SelectContext(ctx, &dest, q, args...); err != nil {
//		return nil, err
//	}
//	return dest, nil
//}

func (b SelectWithWhere[S]) FetchAll(ctx context.Context, db *sqlx.DB) ([]S, error) {
	q, args, err := b.b.build()
	if err != nil {
		return nil, err
	}
	q = db.Rebind(q)

	var dest []S
	if err := db.SelectContext(ctx, &dest, q, args...); err != nil {
		return nil, err
	}
	return dest, nil
}

// Fetch 実行：1行（見つからない場合は sql.ErrNoRows を返す方針）
//func (b SelectBuilder[S, W]) Fetch(ctx context.Context, db *sqlx.DB) (S, error) {
//	q, args, err := b.build()
//	if err != nil {
//		var zero S
//		return zero, err
//	}
//	q = db.Rebind(q)
//
//	var dest S
//	if err := db.GetContext(ctx, &dest, q, args...); err != nil {
//		return dest, err
//	}
//	return dest, nil
//}

// build クエリ式の作成
func (b SelectBuilder2[S]) build() (string, []any, error) {
	if b.where == nil {
		return "", nil, ErrWhereRequired
	}

	// テーブル名の文字列チェック
	if !safeIdent(b.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	selectCols := ""
	switch {
	case len(b.cols) > 0:
		selectCols = strings.Join(b.cols, ",")
	case len(b.except) > 0:
		cols, ok := registry.Columns(b.table)
		if !ok {
			return "", nil, ErrExceptNeedsSchema
		}
		exSet := map[string]struct{}{}
		for _, c := range b.except {
			exSet[c] = struct{}{}
		}
		var picked []string
		for _, c := range cols {
			if _, ng := exSet[c]; !ng {
				picked = append(picked, c)
			}
		}
		if len(picked) == 0 {
			return "", nil, fmt.Errorf("no columns left after except")
		}
		selectCols = strings.Join(picked, ",")
	default:
		selectCols = "*"
	}

	sb := strings.Builder{}
	sb.WriteString("SELECT ")
	sb.WriteString(selectCols)
	sb.WriteString(" FROM ")
	sb.WriteString(b.table)
	sb.WriteString(" WHERE ")
	sb.WriteString(b.where.GetSQL())

	if b.orderBy != nil {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(b.orderBy.GetSQL())
	}
	if b.limit != 0 {
		sb.WriteString(" LIMIT " + strconv.Itoa(b.limit))
	}
	if b.offset != 0 {
		sb.WriteString(" OFFSET " + strconv.Itoa(b.offset))
	}

	fmt.Println(sb.String())
	fmt.Printf("%+v\n", b.where.GwtArgs())

	//return sb.String(), b.where.GwtArgs(), nil
	return sb.String(), b.where.GwtArgs(), nil
}

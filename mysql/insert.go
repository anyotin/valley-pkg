package mysql

import (
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

var ErrValuesRequired = errors.New("insert requires values")

type InsertBuilder struct {
	table  string
	values *InsertCond
}

// InsertFrom は指定されたテーブル用の InsertBuilder を初期化し、返します。
func InsertFrom(table string) InsertBuilder {
	return InsertBuilder{table: table}
}

// Values 指定された InsertCond 条件を InsertBuilder に追加し、更新された InsertBuilder を返します。
func (b InsertBuilder) Values(conds *InsertCond) InsertBuilder {
	b.values = conds
	return b
}

// Exec 実行
func (b InsertBuilder) Exec(ctx context.Context, db *sqlx.DB) (int64, error) {
	q, args, err := b.build()
	if err != nil {
		return 0, err
	}
	q = db.Rebind(q)

	fmt.Printf("update query: %s\n", q)
	fmt.Printf("update args: %#v\n", args)

	res, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// build は SQL INSERT クエリ文字列を構築し、対応する値を準備し、無効な場合はエラーを返します。
func (b InsertBuilder) build() (string, []any, error) {
	if b.values == nil {
		return "", nil, ErrValuesRequired
	}
	if !safeIdent(b.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	valStrs := make([]string, 0, len(b.values.Arg))
	for range b.values.Arg {
		valStrs = append(valStrs, "?")
	}

	sb := strings.Builder{}
	sb.WriteString("INSERT INTO ")
	sb.WriteString(b.table)
	sb.WriteString(" VALUES ")
	sb.WriteString("(" + strings.Join(valStrs, ", ") + ")")

	return sb.String(), b.values.Arg, nil
}

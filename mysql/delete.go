package mysql

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

type DeleteBuilder[W WhereState] struct {
	table string
	where *WhereCond
}

// DeleteFrom は、指定されたテーブル名で初期化された新しい UpdateBuilder を作成します。
func DeleteFrom(table string) DeleteBuilder[WithoutWhere] {
	return DeleteBuilder[WithoutWhere]{table: table}
}

// Where WHERE条件をDeleteBuilderに追加し、WHERE句を持つ状態に移行します。
func (b DeleteBuilder[WithoutWhere]) Where(c *WhereCond) DeleteBuilder[WithWhere] {
	b.where = c
	return DeleteBuilder[WithWhere](b)
}

// Exec は、指定されたコンテキスト内で提供されたデータベース接続に対して、ビルダーによって定義された DELETE SQL クエリを実行します。
// 実行が成功した場合、影響を受けた行数を返します。失敗した場合はエラーを返します。
func (b DeleteBuilder[WithWhere]) Exec(ctx context.Context, db *sqlx.DB) (int64, error) {
	q, args, err := b.build()
	if err != nil {
		return 0, err
	}
	q = db.Rebind(q)

	fmt.Printf("delete query: %s\n", q)
	fmt.Printf("delete args: %#v\n", args)

	res, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// build は DELETE SQL 文とその関連引数を構築し、前提条件が満たされていない場合にエラーを返します。
func (b DeleteBuilder[W]) build() (string, []any, error) {
	if b.where == nil {
		return "", nil, ErrWhereRequired
	}
	if !safeIdent(b.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	sb := strings.Builder{}
	sb.WriteString("DELETE FROM ")
	sb.WriteString(b.table)
	sb.WriteString(" WHERE ")
	sb.WriteString(b.where.GetSQL())

	return sb.String(), b.where.args, nil
}

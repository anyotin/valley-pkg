package mysql

import (
	"context"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

type deleteBuilder struct {
	table string
	where *WhereCond
}

// withWhere はクエリの WHERE 条件を設定し、更新された deleteBuilder インスタンスを返します。
func (d deleteBuilder) withWhere(where *WhereCond) deleteBuilder {
	d.where = where
	return d
}

// build は DELETE SQL 文とその関連引数を構築し、前提条件が満たされていない場合にエラーを返します。
func (d deleteBuilder) build() (string, []any, error) {
	if d.where == nil {
		return "", nil, ErrWhereRequired
	}
	if !safeIdent(d.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", d.table)
	}

	sb := strings.Builder{}
	sb.WriteString("DELETE FROM ")
	sb.WriteString(d.table)
	sb.WriteString(" WHERE ")
	sb.WriteString(d.where.GetSQL())

	return sb.String(), d.where.args, nil
}

type DeleteWithoutWhere struct{ builder deleteBuilder }
type DeleteWithWhere struct{ builder deleteBuilder }

// DeleteFrom は、指定されたテーブル名で初期化された新しい DeleteWithoutWhere を作成します。
func DeleteFrom(table string) DeleteWithoutWhere {
	return DeleteWithoutWhere{builder: deleteBuilder{table: table}}
}

// Where WHERE条件をDeleteBuilderに追加し、WHERE句を持つ状態に移行します。
func (d DeleteWithoutWhere) Where(c *WhereCond) DeleteWithWhere {
	d.builder = d.builder.withWhere(c)
	return DeleteWithWhere(d)
}

// Exec は、指定されたコンテキスト内で提供されたデータベース接続に対して、ビルダーによって定義された DELETE SQL クエリを実行します。
// 実行が成功した場合、影響を受けた行数を返します。失敗した場合はエラーを返します。
func (d DeleteWithWhere) Exec(ctx context.Context, db *sqlx.DB) (int64, error) {
	q, args, err := d.builder.build()
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

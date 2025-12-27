package mysql

import (
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

var ErrSetRequired = errors.New("update requires set")

type updateBuilder[S any] struct {
	table string
	sets  []UpdateCond
	where *WhereCond
}

// withWhere はクエリの WHERE 条件を設定し、更新された selectBuilder インスタンスを返します。
func (u updateBuilder[S]) withWhere(where *WhereCond) updateBuilder[S] {
	u.where = where
	return u
}

// withSet は、指定された更新条件を現在の条件リストに追加し、更新された updateBuilder を返します
func (u updateBuilder[S]) withSet(cond []UpdateCond) updateBuilder[S] {
	u.sets = append(u.sets, cond...)
	return u
}

// build は SQL UPDATE クエリ文字列を構築し、対応する値を準備し、無効な場合はエラーを返します。
func (b updateBuilder[S]) build() (string, []any, error) {
	if len(b.sets) == 0 {
		return "", nil, ErrSetRequired
	}
	if b.where == nil {
		return "", nil, ErrWhereRequired
	}
	if !safeIdent(b.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	setStrs := make([]string, 0, len(b.sets))
	setArgs := make([]any, 0, len(b.sets))
	for _, s := range b.sets {
		setStrs = append(setStrs, fmt.Sprintf("%s = ?", s.Set))
		setArgs = append(setArgs, s.Arg)
	}

	sb := strings.Builder{}
	sb.WriteString("UPDATE ")
	sb.WriteString(b.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setStrs, ", "))
	sb.WriteString(" WHERE ")
	sb.WriteString(b.where.GetSQL())

	return sb.String(), append(setArgs, b.where.args...), nil
}

// ===== Update =====

type UpdateWithWhere[S any] struct{ builder updateBuilder[S] }
type UpdateWithoutWhere[S any] struct{ builder updateBuilder[S] }

// UpdateFrom は、指定されたテーブル名で初期化された新しい UpdateWithoutWhere[S] を作成します。
func UpdateFrom[S any](table string) UpdateWithoutWhere[S] {
	return UpdateWithoutWhere[S]{builder: updateBuilder[S]{table: table}}
}

// Set は1つ以上のUpdateCond要素をsetsスライスに追加し、更新された UpdateWithoutWhere[S] インスタンスを返します。
func (u UpdateWithoutWhere[S]) Set(conds ...UpdateCond) UpdateWithoutWhere[S] {
	u.builder = u.builder.withSet(conds)
	return u
}

// Where はUpdateBuilderにWHERE条件を設定し、その条件が適用された新しい UpdateBuilder インスタンスを返します。
func (u UpdateWithoutWhere[S]) Where(c *WhereCond) UpdateWithWhere[S] {
	u.builder = u.builder.withWhere(c)
	return UpdateWithWhere[S](u)
}

// Exec は、指定されたデータベース接続とコンテキストを使用して、構築された SQL UPDATE 文を実行します。
// 操作が成功した場合、影響を受けた行数を返します。失敗した場合はエラーを返します。
func (u UpdateWithWhere[S]) Exec(ctx context.Context, db *sqlx.DB) (int64, error) {
	q, args, err := u.builder.build()
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
	return res.RowsAffected()
}

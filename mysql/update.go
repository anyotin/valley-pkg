package mysql

import (
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"strings"
)

var ErrSetRequired = errors.New("update requires set")

type UpdateBuilder[W WhereState] struct {
	table string
	sets  []UpdateCond
	where *WhereCond
}

// UpdateFrom は、指定されたテーブル名で初期化された新しい UpdateBuilder を作成します。
func UpdateFrom(table string) UpdateBuilder[WithoutWhere] {
	return UpdateBuilder[WithoutWhere]{table: table}
}

// Set は1つ以上のUpdateCond要素をsetsスライスに追加し、更新されたUpdateBuilderインスタンスを返します。
func (b UpdateBuilder[W]) Set(conds ...UpdateCond) UpdateBuilder[W] {
	b.sets = append(b.sets, conds...)
	return b
}

// Where はUpdateBuilderにWHERE条件を設定し、その条件が適用された新しいUpdateBuilderインスタンスを返します。
func (b UpdateBuilder[WithoutWhere]) Where(c *WhereCond) UpdateBuilder[WithWhere] {
	b.where = c
	return UpdateBuilder[WithWhere](b)
}

// Exec 実行
func (b UpdateBuilder[WithWhere]) Exec(ctx context.Context, db *sqlx.DB) (int64, error) {
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
	return res.RowsAffected()
}

// build は SQL UPDATE クエリ文字列を構築し、対応する値を準備し、無効な場合はエラーを返します。
func (b UpdateBuilder[W]) build() (string, []any, error) {
	if len(b.sets) == 0 {
		return "", nil, ErrSetRequired
	}
	if b.where == nil {
		return "", nil, ErrWhereRequired
	}
	if !safeIdent(b.table) {
		return "", nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	setStrs := make([]string, len(b.sets))
	setArgs := make([]any, len(b.sets))
	for _, s := range b.sets {
		setStrs = append(setStrs, fmt.Sprintf("%s = ?", s.Set))
		setArgs = append(setArgs, s.Arg)
		//setStrs[i] = fmt.Sprintf("%s = ?", s.Set)
		//setArgs[i] = s.Arg
	}

	fmt.Printf("%+v\n", setArgs)
	fmt.Printf("%+v\n", b.where.args)

	sb := strings.Builder{}
	sb.WriteString("UPDATE ")
	sb.WriteString(b.table)
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setStrs, ", "))
	sb.WriteString(" WHERE ")
	sb.WriteString(b.where.GetSQL())

	return sb.String(), append(setArgs, b.where.args...), nil
}

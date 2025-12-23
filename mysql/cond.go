package mysql

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrAndCondTooFew = errors.New("and() requires at least 2 conditions")
	ErrOrCondTooFew  = errors.New("or() requires at least 2 conditions")
)

// ==== Insert条件 ====

type InsertCond struct {
	Arg []any
}

// ==== Update条件 ====

type UpdateCond struct {
	Set string
	Arg any
}

// ==== OrderBy条件 ====

type OrderbyCond struct {
	Column    string
	Direction DirectionEnum
}

func (c OrderbyCond) GetSQL() string {
	if c.Direction == DirectionDefined {
		c.Direction = DESC
	}
	return fmt.Sprintf("%s %s", c.Column, c.Direction.String())
}

// ==== Where条件 ====

type WhereCond struct {
	sql  string
	args []any
}

func (c WhereCond) GetSQL() string { return c.sql }
func (c WhereCond) GwtArgs() []any { return c.args }
func (c WhereCond) isEmpty() bool  { return strings.TrimSpace(c.sql) == "" }

// Eq 等価条件
func Eq(col string, v any) *WhereCond {
	// col は識別子チェック推奨（前回の safeIdent を流用）
	return &WhereCond{sql: fmt.Sprintf("%s = ?", col), args: []any{v}}
}

// NotEq 非等価条件
func NotEq(col string, v any) *WhereCond {
	return &WhereCond{sql: fmt.Sprintf("%s <> ?", col), args: []any{v}}
}

// And And句
func And(conds ...*WhereCond) *WhereCond {
	var parts []string
	var args []any
	for _, c := range conds {
		if c.isEmpty() {
			continue
		}
		parts = append(parts, "("+c.sql+")")
		args = append(args, c.args...)
	}
	//return &WhereCond{sql: "(" + strings.Join(parts, " AND ") + ")", args: args}
	return &WhereCond{sql: strings.Join(parts, " AND "), args: args}

}

// Or Or句
func Or(conds ...*WhereCond) *WhereCond {
	var parts []string
	var args []any
	for _, c := range conds {
		if c.isEmpty() {
			continue
		}
		parts = append(parts, "("+c.sql+")")
		args = append(args, c.args...)
	}

	return &WhereCond{sql: strings.Join(parts, " OR "), args: args}
}

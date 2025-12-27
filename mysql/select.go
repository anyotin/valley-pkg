package mysql

import (
	"context"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrWhereRequired            = errors.New("where clause is required")
	ErrColumnsNotFound          = errors.New("columns registry not found for table")
	ErrExceptNeedsSchema        = errors.New("except() requires registered columns for the table")
	ErrNoColumnsLeftAfterExcept = errors.New("no columns left after except")
	ErrSNotStruct               = errors.New("S must be struct or *struct")
	ErrNoDBTags                 = errors.New("no db tags found in struct")
	ErrDuplicateDBTag           = errors.New("duplicate db tag in struct")
)

// ---- Builder ----

type selectBuilder[S any] struct {
	table   string
	cols    []string
	except  []string
	where   *WhereCond
	orderBy *OrderbyCond
	limit   int
	offset  int
}

// withColumns は、指定された列を SELECT クエリに追加し、更新された selectBuilder インスタンスを返します。
func (b selectBuilder[S]) withColumns(cols []string) selectBuilder[S] {
	b.cols = append(b.cols, cols...)
	return b
}

// withExcept は、指定された列を「除外」リストに追加し、更新された selectBuilder インスタンスを返します。
func (b selectBuilder[S]) withExcept(except []string) selectBuilder[S] {
	b.except = append(b.except, except...)
	return b
}

// withWhere はクエリの WHERE 条件を設定し、更新された selectBuilder インスタンスを返します。
func (b selectBuilder[S]) withWhere(where *WhereCond) selectBuilder[S] {
	b.where = where
	return b
}

// withOrderBy はクエリの ORDER BY 条件を設定し、更新された selectBuilder インスタンスを返します。
func (b selectBuilder[S]) withOrderBy(cond *OrderbyCond) selectBuilder[S] {
	b.orderBy = cond
	return b
}

// withLimit はクエリで返される行数に制限を設定し、更新された selectBuilder を返します。
func (b selectBuilder[S]) withLimit(limit int) selectBuilder[S] {
	b.limit = limit
	return b
}

// withOffset はクエリ結果のオフセットを設定し、更新された selectBuilder を返します。
func (b selectBuilder[S]) withOffset(offset int) selectBuilder[S] {
	b.offset = offset
	return b
}

// buildWithWhere は WHERE 句を含む SQL SELECT クエリを構築し、クエリ文字列、引数、およびエラーを返します。
// WHERE 条件が指定されていない場合、ErrWhereRequired を返します。
func (b selectBuilder[S]) buildWithWhere() (string, []any, error) {
	if b.where == nil {
		return "", nil, ErrWhereRequired
	}

	sb, err := b.buildHead()
	if err != nil {
		return "", nil, err
	}

	fmt.Printf("sb:  %s\n", sb.String())

	sb.WriteString(" WHERE ")
	fmt.Printf("sb:  %s\n", sb.String())

	sb.WriteString(b.where.GetSQL())

	fmt.Printf("sb:  %s\n", sb.String())

	b.buildTail(sb)
	return sb.String(), b.where.GwtArgs(), nil
}

// buildWithoutWhere は WHERE 句を除外した SQL SELECT クエリを構築し、クエリ文字列と発生したエラーを返します。
func (b selectBuilder[S]) buildWithoutWhere() (string, []any, error) {
	sb, err := b.buildHead()
	if err != nil {
		return "", nil, err
	}

	b.buildTail(sb)
	return sb.String(), nil, nil
}

// buildHead は、SELECT 列と FROM 句を含む SQL SELECT クエリの初期セグメントを構築します。
func (b selectBuilder[S]) buildHead() (*strings.Builder, error) {
	if !safeIdent(b.table) {
		return nil, fmt.Errorf("unsafe table: %s", b.table)
	}

	selectCols, err := b.pickColumns()
	if err != nil {
		return nil, err
	}

	sb := new(strings.Builder)
	sb.WriteString("SELECT ")
	sb.WriteString(selectCols)
	sb.WriteString(" FROM ")
	sb.WriteString(b.table)
	return sb, nil
}

// buildTail は、ビルダーで設定されている場合、指定された SQL クエリに ORDER BY、LIMIT、および OFFSET 句を追加します。
func (b selectBuilder[S]) buildTail(sb *strings.Builder) {
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
}

// pickColumns は、含まれる列または除外される列に基づいて、クエリで選択する列を決定します。
// 除外された列の結果として列が存在しない場合、エラーを返します。それ以外の場合は、カンマ区切りの列の文字列を返します。
func (b selectBuilder[S]) pickColumns() (string, error) {
	selectCols := ""
	switch {
	case len(b.cols) > 0:
		selectCols = strings.Join(b.cols, ",")
		return selectCols, nil
	case len(b.except) > 0:
		cols, err := b.columnsOf()
		if err != nil {
			return "", ErrExceptNeedsSchema
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
			return "", ErrNoColumnsLeftAfterExcept
		}
		selectCols = strings.Join(picked, ",")
		return selectCols, nil
	default:
		selectCols = "*"
		return selectCols, nil
	}
}

// columnsOf は、構造体型のデータベースタグから列名を抽出し、カンマ区切りの文字列として返します。
func (b selectBuilder[S]) columnsOf() ([]string, error) {
	// 型を取り出し
	var zero S
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, ErrSNotStruct
	}

	cols, err := columnsFromDBTags(t)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, ErrNoDBTags
	}

	return cols, nil
}

// columnsFromDBTags は、構造体フィールドから「db」タグを持つ列名を抽出します。一意性を保証し、指定されたフィールドはスキップします。
// 列名のスライスを返します。重複タグが存在する場合やその他の問題が発生した場合はエラーを返します。
func columnsFromDBTags(t reflect.Type) ([]string, error) {
	var cols []string
	seen := map[string]struct{}{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		name := tag
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			name = tag[:idx]
		}
		if name == "" || name == "-" {
			continue
		}
		if _, ok := seen[name]; ok {
			return nil, ErrDuplicateDBTag
		}
		seen[name] = struct{}{}
		cols = append(cols, name)
	}
	return cols, nil
}

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

// ----- Select -----

type SelectWithoutWhere[S any] struct{ builder selectBuilder[S] }
type SelectWithWhere[S any] struct{ builder selectBuilder[S] }

// SelectFrom は指定されたテーブル名で selectBuilder を初期化
func SelectFrom[S any](table string) SelectWithoutWhere[S] {
	return SelectWithoutWhere[S]{builder: selectBuilder[S]{table: table}}
}

// Columns はクエリで選択する列を設定し、更新された SelectWithWhere インスタンスを返します。
func (s SelectWithWhere[S]) Columns(cols ...string) SelectWithWhere[S] {
	s.builder = s.builder.withColumns(cols)
	return s
}

// Columns はクエリで選択する列を設定し、更新された SelectWithoutWhere インスタンスを返します。
func (s SelectWithoutWhere[S]) Columns(cols ...string) SelectWithoutWhere[S] {
	s.builder = s.builder.withColumns(cols)
	return s
}

// Except はクエリの選択から除外する列名を指定し、新しい SelectWithWhere インスタンスを返します。
func (s SelectWithWhere[S]) Except(cols ...string) SelectWithWhere[S] {
	s.builder = s.builder.withExcept(cols)
	return s
}

// Except はクエリの選択から除外する列名を指定し、新しい SelectWithoutWhere インスタンスを返します。
func (s SelectWithoutWhere[S]) Except(cols ...string) SelectWithoutWhere[S] {
	s.builder = s.builder.withExcept(cols)
	return s
}

// Where 指定された条件をクエリに適用し、更新されたビルダーを持つ新しい SelectWithWhere インスタンスを返します。
func (s SelectWithoutWhere[S]) Where(cond *WhereCond) SelectWithWhere[S] {
	s.builder = s.builder.withWhere(cond)
	return SelectWithWhere[S]{builder: s.builder}
}

// OrderBy は、指定された OrderbyCond を使用してクエリの順序付け条件を設定し、更新された SelectWithWhere を返します。
func (s SelectWithWhere[S]) OrderBy(cond *OrderbyCond) SelectWithWhere[S] {
	s.builder = s.builder.withOrderBy(cond)
	return s
}

// OrderBy は、指定された OrderbyCond を使用してクエリの順序付け条件を設定し、更新された SelectWithoutWhere を返します。
func (s SelectWithoutWhere[S]) OrderBy(cond *OrderbyCond) SelectWithoutWhere[S] {
	s.builder = s.builder.withOrderBy(cond)
	return s
}

// Limit は返す行の最大数を設定し、SelectWithWhere インスタンスを更新します。
func (s SelectWithWhere[S]) Limit(limit int) SelectWithWhere[S] {
	s.builder = s.builder.withLimit(limit)
	return s
}

// Limit は返す行の最大数を設定し、SelectWithoutWhere インスタンスを更新します。
func (s SelectWithoutWhere[S]) Limit(limit int) SelectWithoutWhere[S] {
	s.builder = s.builder.withLimit(limit)
	return s
}

// Offset はクエリでスキップする行数を設定し、更新された SelectWithWhere インスタンスを返します。
func (s SelectWithWhere[S]) Offset(offset int) SelectWithWhere[S] {
	s.builder = s.builder.withLimit(offset)
	return s
}

// Offset はクエリでスキップする行数を設定し、更新された SelectWithoutWhere インスタンスを返します。
func (s SelectWithoutWhere[S]) Offset(offset int) SelectWithoutWhere[S] {
	s.builder = s.builder.withLimit(offset)
	return s
}

// FetchAll は、構築されたクエリとバインディングに基づいて SQL SELECT クエリを実行し、一致するすべての行をスライスとして返します。
func (s SelectWithWhere[S]) FetchAll(ctx context.Context, db *sqlx.DB) ([]S, error) {
	q, args, err := s.builder.buildWithWhere()
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

// FetchAll は構築された SQL SELECT クエリを実行し、すべての行を S 型のスライスとして取得します。
func (s SelectWithoutWhere[S]) FetchAll(ctx context.Context, db *sqlx.DB) ([]S, error) {
	q, args, err := s.builder.buildWithoutWhere()
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

// Fetch は SQL SELECT クエリを実行し、構築されたクエリとバインディングに基づいて結果の単一行を取得します。
func (s SelectWithWhere[S]) Fetch(ctx context.Context, db *sqlx.DB) (S, error) {
	q, args, err := s.builder.buildWithWhere()
	if err != nil {
		var zero S
		return zero, err
	}
	q = db.Rebind(q)

	var dest S
	if err := db.GetContext(ctx, &dest, q, args...); err != nil {
		return dest, err
	}
	return dest, nil
}

// Fetch は SQL SELECT クエリを実行し、構築されたクエリとバインディングに基づいて結果の単一行を取得します。
func (s SelectWithoutWhere[S]) Fetch(ctx context.Context, db *sqlx.DB) (S, error) {
	q, args, err := s.builder.buildWithoutWhere()
	if err != nil {
		var zero S
		return zero, err
	}
	q = db.Rebind(q)

	var dest S
	if err := db.GetContext(ctx, &dest, q, args...); err != nil {
		return dest, err
	}
	return dest, nil
}

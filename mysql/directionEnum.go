package mysql

type DirectionEnum int

const (
	DirectionDefined DirectionEnum = iota
	ASC
	DESC
)

func (d DirectionEnum) String() string {
	switch d {
	case ASC:
		return "ASC"
	case DESC:
		return "DESC"
	default:
		return "DESC"
	}
}

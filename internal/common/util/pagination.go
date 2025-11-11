package util

// Pagination describes limit-offset paging parameters.
type Pagination struct {
	Page     int
	PageSize int
}

// Normalize ensures default pagination values are applied.
func (p Pagination) Normalize() Pagination {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 50
	}
	return p
}

// Offset calculates the SQL offset for the pagination settings.
func (p Pagination) Offset() int {
	n := p.Normalize()
	return (n.Page - 1) * n.PageSize
}

// Limit calculates the SQL limit for the pagination settings.
func (p Pagination) Limit() int {
	return p.Normalize().PageSize
}

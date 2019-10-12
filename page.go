package kitty

import (
	"math"
)

// Page 分页
type Page struct {
	Page    uint32 `json:"page"`
	Limit   uint32 `json:"limit"`
	PageMax uint32 `json:"page_max"`
	Total   uint32 `json:"total"`
}

// GetOffset .
func (p *Page) GetOffset() uint32 {
	return (p.Page - 1) * p.Limit
}

// CountPages .
func (p *Page) CountPages(total uint32) {
	if p.Page == 0 {
		p.Page = 1
	}

	if total == 0 {
		p.Page = 1
		p.PageMax = 1
	} else {
		p.PageMax = uint32(math.Ceil(float64(total) / float64(p.Limit)))
		if p.Page > p.PageMax {
			p.Page = p.PageMax
		}
	}

	p.Total = total
}

// MakePage .
func MakePage(page uint32, limit uint32, total uint32) Page {

	var p = Page{
		Page:  page,
		Limit: limit,
	}

	p.CountPages(total)

	return p
}

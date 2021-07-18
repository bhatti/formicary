package controller

import (
	"fmt"
	"plexobject.com/formicary/internal/web"
	"strconv"
)

// PaginatedResult structure
type PaginatedResult struct {
	Records      interface{}
	TotalRecords int64
	Page         int
	PageSize     int
	TotalPages   int64
}

// NewPaginatedResult constructor
func NewPaginatedResult(
	records interface{},
	totalRecords int64,
	page int,
	pageSize int) *PaginatedResult {
	totalPages := int64(0)
	if totalRecords > 0 {
		totalPages = int64(pageSize) / totalRecords
	}
	return &PaginatedResult{
		Records:      records,
		TotalRecords: totalRecords,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
	}
}

// Pagination prepares map that UI uses
func Pagination(page int, pageSize int, total int64, baseURL string) (res map[string]interface{}) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int(total / int64(pageSize))
	}
	res = make(map[string]interface{})
	res["TotalPages"] = totalPages
	res["Page"] = page
	res["DisplayPage"] = page + 1
	res["PageSize"] = pageSize
	res["HasPrevPage"] = page > 0
	res["HasNextPage"] = page+1 < totalPages
	res["PrevPage"] = page - 1
	res["NextPage"] = page + 1
	res["TotalRecords"] = total
	res["BaseURL"] = baseURL
	prevPages := make([]map[string]interface{}, 0)
	start := 0
	if page-4 > start {
		start = page - 3
		res["PrevPagesDot"] = "..."
	}
	for i := start; i < page; i++ {
		prevPages = append(prevPages, map[string]interface{}{"Page": i, "DisplayPage": i + 1, "BaseURL": baseURL})
	}
	res["PrevPages"] = prevPages

	nextPages := make([]map[string]interface{}, 0)
	end := totalPages
	if page+3 < totalPages {
		end = page + 3
		res["NextPagesDot"] = "..."
	}
	for i := page + 1; i <= end; i++ {
		nextPages = append(nextPages, map[string]interface{}{"Page": i, "DisplayPage": i + 1, "BaseURL": baseURL})
	}
	res["NextPages"] = nextPages
	return
}

// ParseParams parses params
func ParseParams(c web.WebContext) (
	params map[string]interface{},
	order []string,
	page int,
	pageSize int,
	q string) {
	page = 0
	pageSize = 200
	params = make(map[string]interface{})
	for name, value := range c.QueryParams() {
		if name == "page" {
			page, _ = strconv.Atoi(value[0])
		} else if name == "pageSize" {
			pageSize, _ = strconv.Atoi(value[0])
		} else if name == "order" {
			order = value
			q += fmt.Sprintf("order=%s&", value[0])
		} else {
			params[name] = value[0]
			q += fmt.Sprintf("%s=%s&", name, value[0])
		}
	}
	q += fmt.Sprintf("pageSize=%d&", pageSize)
	return
}

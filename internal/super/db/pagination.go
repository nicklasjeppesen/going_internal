package db

import (
	"fmt"
	"strconv"
)

type Pagination struct {
	PageStr string
	PerPage int
	Path    string
}

func (pag Pagination) page() int {
	var page int
	var err error
	if pag.PageStr == "" {
		page = 1
	} else {
		page, err = strconv.Atoi(pag.PageStr)
		if err != nil || page < 1 {
			page = 1
		}
	}
	return page
}

func (pag Pagination) AlreadySeen() int {
	return (pag.page() - 1) * pag.PerPage
}

func (pag Pagination) firstPageUrl() string {
	return fmt.Sprintf("%s?page=1", pag.Path)
}

func (pag Pagination) prevPageUrl() string {
	if pag.page() > 1 {
		return fmt.Sprintf("%s?page=%d", pag.Path, pag.page()-1)
	}
	return ""
}

func (pag Pagination) nextPageUrl(numberOfResults int) string {
	var next_page_url string
	if numberOfResults < pag.PerPage {
		next_page_url = ""
	} else {
		next_page_url = fmt.Sprintf("%s?page=%d", pag.Path, pag.page()+1)
	}
	return next_page_url
}

func (pag Pagination) ToMap(results []any) map[string]any {
	return map[string]any{
		"current_page":   pag.page(),
		"data":           results,
		"per_page":       pag.PerPage,
		"path":           pag.Path,
		"first_page_url": pag.firstPageUrl(),
		"next_page_url":  pag.nextPageUrl(len(results)),
		"prev_page_url":  pag.prevPageUrl(),
	}
}

package siteconfig

import (
	"regexp"
	"strings"
)

type ReplaceType string

const (
	ReplaceTypeSimple ReplaceType = ""
	ReplaceTypeRegex  ReplaceType = "regex"
)

type Replacement struct {
	From  string      `json:"from,omitempty"`
	To    string      `json:"to,omitempty"`
	Count int         `json:"count,omitempty"`
	Type  ReplaceType `json:"type,omitempty"`

	cachedRegex *regexp.Regexp
}

func (r *Replacement) Replace(content []byte) []byte {
	if r.Type == ReplaceTypeRegex {
		if r.cachedRegex == nil {
			r.cachedRegex = regexp.MustCompile(r.From)
		}
		return r.cachedRegex.ReplaceAll(content, []byte(r.To))
	}
	count := r.Count
	if count == 0 {
		count = -1
	}
	return []byte(strings.Replace(string(content), r.From, r.To, count))
}

func (r *Replacement) CompileRegex() error {
	if r.Type != ReplaceTypeRegex {
		return nil
	}
	r.cachedRegex = regexp.MustCompile(r.From)
	return nil
}

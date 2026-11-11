// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	bt "github.com/pratikdeoghare/brashtag"
)

// parseBrashtag parses a whole document. The text is wrapped in an anonymous
// bag, as the original barkdown does, because the parser reads exactly one bag.
func parseBrashtag(src string) (bt.Bag, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	body := strings.TrimSpace(src)
	wrapped := "#{" + body + "}"
	tree, err := bt.Parse(wrapped)
	if err != nil {
		lead := len(src) - len(strings.TrimLeftFunc(src, unicode.IsSpace))
		return bt.Bag{}, locate(src, wrapped, lead, err)
	}
	return tree.(bt.Bag), nil
}

// ParseError is a syntax error in a document, with the line it was found on
// (0 when the parser gives no clue).
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return e.Msg
}

func locate(src, wrapped string, lead int, err error) error {
	msg := err.Error()
	head, off := msg, -1
	for i := 0; i+2 < len(msg); i++ {
		if msg[i:i+2] != ": " {
			continue
		}
		if k := strings.Index(wrapped, msg[i+2:]); k >= 0 {
			head, off = msg[:i], k
			break
		}
	}
	var text string
	switch {
	case strings.HasPrefix(head, "%!w(<nil>)"):
		text = "unmatched '}' ends the document early; write it inside backticks"
	case strings.HasPrefix(head, "unexpected {"):
		text = "'{' outside a tag; write it inside backticks"
		if off >= 0 {
			if j := strings.IndexByte(wrapped[off:], '{'); j >= 0 {
				off += j
			}
		}
	case strings.HasPrefix(head, "unopened bag"):
		text = "'#' starts a tag but no '{' follows; write it inside backticks"
	case strings.HasPrefix(head, "code not closed"):
		text = "code is not closed: the backticks that open it never appear again"
	case strings.HasPrefix(head, "bag not closed"):
		text = "a #tag{ is never closed: a '}' is missing"
	default:
		text = head
	}
	line := 0
	if off >= 0 {
		pos := min(max(lead+off-2, 0), len(src))
		line = 1 + strings.Count(src[:pos], "\n")
	}
	return &ParseError{Line: line, Msg: text}
}

// textOf returns the text of a node with all tags removed.
func textOf(n bt.Node) string {
	switch x := n.(type) {
	case bt.Blob:
		return x.Text()
	case bt.Code:
		return x.Text()
	case bt.Bag:
		return textOfNodes(x.Kids())
	}
	return ""
}

func textOfNodes(nodes []bt.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(textOf(n))
	}
	return b.String()
}

// plainText is the readable text of a document body, for search, word
// counts and summaries.
func plainText(nodes []bt.Node) string {
	var b strings.Builder
	var walk func([]bt.Node)
	walk = func(nodes []bt.Node) {
		for _, n := range nodes {
			switch x := n.(type) {
			case bt.Blob:
				b.WriteString(x.Text())
			case bt.Code:
				if isFence(x) {
					b.WriteString(" " + x.Text() + " ")
				} else {
					b.WriteString(x.Text())
				}
			case bt.Bag:
				tag := strings.TrimSpace(x.Tag())
				if tag == "comment" || tag == "html" {
					continue
				}
				h, known := handlers[tag]
				space := !known || h.block
				if space {
					b.WriteString(" ")
				}
				kids := x.Kids()
				if targetTags[tag] {
					_, kids = firstWord(kids)
				}
				if isThought(x) {
					_, kids = firstWord(kids)
					if clock, after := firstWord(kids); clockWord.MatchString(clock) {
						kids = after
					}
				}
				walk(kids)
				if space {
					b.WriteString(" ")
				}
			}
		}
	}
	walk(nodes)
	return strings.Join(strings.Fields(b.String()), " ")
}

var targetTags = map[string]bool{"link": true, "a": true, "ref": true, "img": true, "image": true,
	"figure": true, "video": true, "audio": true, "youtube": true, "abbr": true}

// truncateWords keeps the first n words of s.
func truncateWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) <= n {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:n], " ") + "…"
}

// slugify turns a title into a URL-friendly name: "Hello, World!" -> "hello-world".
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || (unicode.IsMark(r) && b.Len() > 0) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	return b.String()
}

// Latest returns the most recent thought of a post, or nil.
func (d *Doc) Latest() *Thought {
	var latest *Thought
	for _, t := range d.Thoughts {
		if latest == nil || t.Time.After(latest.Time) {
			latest = t
		}
	}
	return latest
}

// Doc is one post or page.
type Doc struct {
	Source   string // path of the file, relative to the site root
	Kind     string // "post" or "page"
	Title    string
	Subtitle string
	Author   string
	Summary  string // from #summary{} or #description{}, as plain text
	Date     time.Time
	Updated  time.Time
	Tags     []string
	Draft    bool
	Slug     string
	Layout   string            // template to use instead of the default
	Weight   int               // order of pages in the menu
	Meta     map[string]string // every metadata bag, by tag, as text

	body   []bt.Node // the document without its metadata
	bundle string    // directory of a page bundle, "" otherwise

	// Filled in by the site while building.
	URL         string
	Content     template.HTML
	Excerpt     template.HTML
	HasMore     bool
	Description string
	Text        string
	Words       int
	ReadingTime int
	Math        bool
	Mermaid     bool
	TagRefs     []*Tag
	Prev, Next  *Doc
	Related     []*Doc

	Thoughts    []*Thought
	LastThought time.Time

	excerptMath, excerptMermaid bool
}

var metaTags = map[string]bool{
	"title": true, "subtitle": true, "author": true, "summary": true,
	"description": true, "date": true, "updated": true, "tags": true,
	"draft": true, "slug": true, "layout": true, "weight": true, "order": true,
}

// fileError prefixes an error with its file and, for a ParseError, its line.
func fileError(source string, err error) error {
	var pe *ParseError
	if errors.As(err, &pe) && pe.Line > 0 {
		return fmt.Errorf("%s:%d: %s", source, pe.Line, pe.Msg)
	}
	return fmt.Errorf("%s: %v", source, err)
}

// ParseDoc parses a document and separates its metadata from its body.
func ParseDoc(source, text string) (*Doc, error) {
	tree, err := parseBrashtag(text)
	if err != nil {
		return nil, fileError(source, err)
	}
	d := &Doc{Source: source, Meta: map[string]string{}}
	for _, kid := range tree.Kids() {
		b, ok := kid.(bt.Bag)
		tag := ""
		if ok {
			tag = strings.TrimSpace(b.Tag())
		}
		switch {
		case tag == "meta":
			for _, m := range b.Kids() {
				if mb, ok := m.(bt.Bag); ok {
					if err := d.setMeta(mb); err != nil {
						return nil, fmt.Errorf("%s: %v", source, err)
					}
				}
			}
		case metaTags[tag]:
			if err := d.setMeta(b); err != nil {
				return nil, fmt.Errorf("%s: %v", source, err)
			}
		default:
			d.body = append(d.body, kid)
		}
	}
	return d, nil
}

func (d *Doc) setMeta(b bt.Bag) error {
	tag := strings.TrimSpace(b.Tag())
	val := strings.Join(strings.Fields(textOf(b)), " ")
	d.Meta[tag] = val
	if val == "" && tag != "draft" {
		return nil
	}
	var err error
	switch tag {
	case "title":
		d.Title = val
	case "subtitle":
		d.Subtitle = val
	case "author":
		d.Author = val
	case "summary", "description":
		d.Summary = val
	case "date":
		d.Date, err = parseDate(val)
	case "updated":
		d.Updated, err = parseDate(val)
	case "tags":
		d.Tags = splitList(val)
	case "draft":
		d.Draft = val != "false" && val != "no"
	case "slug":
		d.Slug = slugify(val)
	case "layout":
		d.Layout = val
	case "weight":
		d.Weight, err = strconv.Atoi(val)
	case "order":
		if val != "oldest" && val != "newest" {
			err = fmt.Errorf("the order of thoughts is oldest or newest")
		}
	}
	if err != nil {
		return fmt.Errorf("#%s{%s}: %v", tag, val, err)
	}
	return nil
}

var dateLayouts = []string{
	"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04",
	time.RFC3339, "2 January 2006", "January 2, 2006", "2 Jan 2006",
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date; use YYYY-MM-DD or YYYY-MM-DD HH:MM")
}

func splitList(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" && !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

var datePrefix = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(.+)$`)

// nameParts derives a slug, and a date if the name starts with one, from a
// file name such as "2026-11-01-hello-world.bt" or a bundle's directory.
func nameParts(path string) (slug string, date time.Time) {
	name := strings.TrimSuffix(filepath.Base(path), ".bt")
	if name == "index" {
		name = filepath.Base(filepath.Dir(path))
	}
	if m := datePrefix.FindStringSubmatch(name); m != nil {
		if t, err := time.ParseInLocation("2006-01-02", m[1], time.Local); err == nil {
			date, name = t, m[2]
		}
	}
	return slugify(name), date
}

// humanize makes a title out of a slug, for documents that lack #title{}.
func humanize(slug string) string {
	s := strings.ReplaceAll(slug, "-", " ")
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	bt "barkdown/brashtag"
)

// Config is the content of site.bt.
type Config struct {
	Title       string
	Subtitle    string
	Description string
	Author      string
	BaseURL     string
	Language    string
	DateFormat  string
	PerPage     int
	Permalink   string
	MathJax     string
	Mermaid     string
	Fonts       string
	Nav         []NavItem
	Footer      template.HTML
	Meta        map[string]string
	footer      []bt.Node
}

type NavItem struct{ Title, URL string }

func defaultConfig() Config {
	return Config{
		Title:      "My barkdown site",
		BaseURL:    "/",
		Language:   "en",
		DateFormat: "2 January 2006",
		PerPage:    10,
		Permalink:  "/posts/:slug/",
		MathJax:    "https://cdn.jsdelivr.net/npm/mathjax@3.2.2/es5/tex-chtml.js",
		Mermaid:    "https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.esm.min.mjs",
		Fonts:      "https://cdn.jsdelivr.net/npm/tufte-css@1.8.0/et-book",
		Meta:       map[string]string{},
	}
}

// LoadConfig reads site.bt; a missing file gives the defaults.
func LoadConfig(path string) (Config, error) {
	c := defaultConfig()
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	tree, err := parseBrashtag(string(src))
	if err != nil {
		return c, fileError(filepath.Base(path), err)
	}
	for _, kid := range tree.Kids() {
		b, ok := kid.(bt.Bag)
		if !ok {
			continue
		}
		tag := strings.TrimSpace(b.Tag())
		val := strings.Join(strings.Fields(textOf(b)), " ")
		c.Meta[tag] = val
		switch tag {
		case "title":
			c.Title = val
		case "subtitle":
			c.Subtitle = val
		case "description":
			c.Description = val
		case "author":
			c.Author = val
		case "baseurl", "url":
			c.BaseURL = val
		case "language", "lang":
			c.Language = val
		case "dateformat":
			c.DateFormat = val
		case "permalink":
			c.Permalink = val
		case "mathjax":
			c.MathJax = val
		case "mermaid":
			c.Mermaid = val
		case "fonts":
			c.Fonts = strings.TrimSuffix(val, "/")
		case "perpage":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return c, fmt.Errorf("site.bt: #perpage{%s}: need a positive number", val)
			}
			c.PerPage = n
		case "nav":
			for _, k := range b.Kids() {
				if l, ok := k.(bt.Bag); ok && (strings.TrimSpace(l.Tag()) == "link" || strings.TrimSpace(l.Tag()) == "a") {
					u, rest := firstWord(l.Kids())
					c.Nav = append(c.Nav, NavItem{Title: words(rest), URL: u})
				}
			}
		case "footer":
			c.footer = b.Kids()
		}
	}
	return c, nil
}

// Options control a build.
type Options struct {
	Src, Out string
	Drafts   bool      // include drafts
	Future   bool      // include posts dated in the future
	BaseURL  string    // overrides #baseurl in site.bt
	Serving  string    // host:port of the development server, if any
	Now      time.Time // the time the build happens
}

// Site is everything known about a site while it is built.
type Site struct {
	Config
	opts       Options
	root       string
	basePath   string // the path of BaseURL, with a slash at both ends
	absBase    string // scheme, host and base path; "" when BaseURL is relative
	Posts      []*Doc
	Pages      []*Doc
	Tags       []*Tag
	Menu       []NavItem
	LiveReload bool
	Built      time.Time
	Warnings   []string
	Skipped    int    // drafts and future posts left out
	Timeline   []*Doc // posts with thoughts, the most recently added to first
	bySlug     map[string]*Doc

	// Used while writing.
	theme     fs.FS
	templates map[string]*template.Template
	assets    map[string]string
	sitemap   []sitemapURL
	pages     int
}

// Tag is a tag and the posts that carry it, newest first.
type Tag struct {
	Name, Slug, URL string
	Posts           []*Doc
}

func (s *Site) warnf(format string, args ...any) {
	s.Warnings = append(s.Warnings, fmt.Sprintf(format, args...))
}

func (s *Site) all() []*Doc {
	return append(append([]*Doc(nil), s.Posts...), s.Pages...)
}

func (s *Site) setBase() error {
	u, err := url.Parse(s.BaseURL)
	if err != nil {
		return fmt.Errorf("site.bt: #baseurl{%s}: %v", s.BaseURL, err)
	}
	p := u.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	s.basePath = p
	if u.Scheme != "" && u.Host != "" {
		s.absBase = u.Scheme + "://" + u.Host + p
	}
	if s.opts.Serving != "" {
		s.absBase = "http://" + s.opts.Serving + p
	}
	return nil
}

// URL turns a site path such as "/posts/x/" into a link with the base path.
func (s *Site) URL(p string) string {
	if isExternal(p) || !strings.HasPrefix(p, "/") {
		return p
	}
	return s.basePath + strings.TrimPrefix(p, "/")
}

// Abs turns a site path into an absolute URL.
func (s *Site) Abs(p string) string {
	if isExternal(p) || s.absBase == "" {
		return s.URL(p)
	}
	return s.absBase + strings.TrimPrefix(p, "/")
}

// Load reads and renders a site without writing anything.
func Load(opts Options) (*Site, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	cfg, err := LoadConfig(filepath.Join(opts.Src, "site.bt"))
	if err != nil {
		return nil, err
	}
	if opts.BaseURL != "" {
		cfg.BaseURL = opts.BaseURL
	}
	s := &Site{Config: cfg, opts: opts, root: opts.Src, bySlug: map[string]*Doc{},
		Built: opts.Now, LiveReload: opts.Serving != ""}
	if err := s.setBase(); err != nil {
		return nil, err
	}
	posts, err := s.loadDir("posts", "post")
	if err != nil {
		return nil, err
	}
	pages, err := s.loadDir("pages", "page")
	if err != nil {
		return nil, err
	}
	for _, d := range append(posts, pages...) {
		switch {
		case d.Draft && !opts.Drafts, d.Kind == "post" && d.Date.After(opts.Now) && !opts.Future:
			s.Skipped++
		case d.Kind == "post":
			s.Posts = append(s.Posts, d)
		default:
			s.Pages = append(s.Pages, d)
		}
	}
	sort.SliceStable(s.Posts, func(i, j int) bool {
		a, b := s.Posts[i], s.Posts[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		return a.Title < b.Title
	})
	sort.SliceStable(s.Pages, func(i, j int) bool {
		a, b := s.Pages[i], s.Pages[j]
		if a.Weight != b.Weight {
			return a.Weight < b.Weight
		}
		return a.Title < b.Title
	})
	reserved := map[string]bool{"tags": true, "archive": true, "search": true, "page": true, "assets": true, "timeline": true}
	urls := map[string]*Doc{}
	for _, d := range s.all() {
		if d.Kind == "post" {
			d.URL = s.permalink(d)
		} else {
			d.URL = "/" + d.Slug + "/"
			if reserved[d.Slug] || d.Slug == "" {
				return nil, fmt.Errorf("%s: a page cannot be called %q; set another #slug{}", d.Source, d.Slug)
			}
		}
		if other := s.bySlug[d.Slug]; other != nil {
			return nil, fmt.Errorf("%s and %s have the same slug %q; set #slug{} in one of them", other.Source, d.Source, d.Slug)
		}
		if other := urls[d.URL]; other != nil {
			return nil, fmt.Errorf("%s and %s would both be published at %s", other.Source, d.Source, d.URL)
		}
		s.bySlug[d.Slug], urls[d.URL] = d, d
	}
	byTag := map[string]*Tag{}
	for _, d := range s.Posts {
		for _, name := range d.Tags {
			slug := slugify(name)
			if slug == "" {
				continue
			}
			t := byTag[slug]
			if t == nil {
				t = &Tag{Name: name, Slug: slug, URL: "/tags/" + slug + "/"}
				byTag[slug] = t
				s.Tags = append(s.Tags, t)
			}
			t.Posts = append(t.Posts, d)
			d.TagRefs = append(d.TagRefs, t)
		}
	}
	sort.Slice(s.Tags, func(i, j int) bool {
		return strings.ToLower(s.Tags[i].Name) < strings.ToLower(s.Tags[j].Name)
	})
	for i, d := range s.Posts {
		if i > 0 {
			d.Next = s.Posts[i-1]
		}
		if i+1 < len(s.Posts) {
			d.Prev = s.Posts[i+1]
		}
		d.Related = s.related(d, 3)
	}
	for _, d := range s.all() {
		s.render(d)
	}
	if s.footer != nil {
		s.Footer = template.HTML(NewRenderer(s).compactFlow(s.footer))
	}
	for _, d := range s.Posts {
		if len(d.Thoughts) > 0 {
			s.Timeline = append(s.Timeline, d)
		}
	}
	sort.SliceStable(s.Timeline, func(i, j int) bool { return s.Timeline[i].LastThought.After(s.Timeline[j].LastThought) })
	if len(s.Nav) > 0 {
		s.Menu = s.Nav
	} else {
		s.Menu = []NavItem{{"Home", "/"}}
		if len(s.Timeline) > 0 {
			s.Menu = append(s.Menu, NavItem{"Timeline", "/timeline/"})
		}
		for _, p := range s.Pages {
			if p.Weight >= 0 {
				s.Menu = append(s.Menu, NavItem{p.Title, p.URL})
			}
		}
		s.Menu = append(s.Menu, NavItem{"Archive", "/archive/"}, NavItem{"Tags", "/tags/"}, NavItem{"Search", "/search/"})
	}
	return s, nil
}

func (s *Site) loadDir(dir, kind string) ([]*Doc, error) {
	root := filepath.Join(s.root, dir)
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var docs []*Doc
	var errs []error
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".bt" {
			return nil
		}
		doc, err := s.loadDoc(path, root, kind)
		if err != nil {
			errs = append(errs, err)
		} else {
			docs = append(docs, doc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, errors.Join(errs...)
}

func (s *Site) loadDoc(path, root, kind string) (*Doc, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(s.root, path)
	d, err := ParseDoc(filepath.ToSlash(rel), string(src))
	if err != nil {
		return nil, err
	}
	d.Kind = kind
	slug, date := nameParts(path)
	if d.Slug == "" {
		d.Slug = slug
	}
	if d.Date.IsZero() {
		d.Date = date
	}
	if filepath.Base(path) == "index.bt" && filepath.Dir(path) != root {
		d.bundle = filepath.Dir(path)
	}
	if d.Title == "" {
		d.Title = humanize(d.Slug)
		s.warnf("%s: no #title{}, using %q", d.Source, d.Title)
	}
	if kind == "post" && d.Date.IsZero() {
		if info, err := os.Stat(path); err == nil {
			d.Date = info.ModTime()
		}
		s.warnf("%s: no #date{} and no date in the file name; using the file's modification time", d.Source)
	}
	return d, nil
}

func (s *Site) permalink(d *Doc) string {
	p := strings.NewReplacer(
		":year", d.Date.Format("2006"), ":month", d.Date.Format("01"),
		":day", d.Date.Format("02"), ":slug", d.Slug,
	).Replace(s.Permalink)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") && path.Ext(p) == "" {
		p += "/"
	}
	return p
}

func (s *Site) related(d *Doc, n int) []*Doc {
	type scored struct {
		doc   *Doc
		score int
	}
	var cands []scored
	for _, o := range s.Posts {
		if o == d {
			continue
		}
		score := 0
		for _, t := range d.TagRefs {
			for _, u := range o.TagRefs {
				if t == u {
					score++
				}
			}
		}
		if score > 0 {
			cands = append(cands, scored{o, score})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	var out []*Doc
	for _, c := range cands[:min(n, len(cands))] {
		out = append(out, c.doc)
	}
	return out
}

const wordsPerMinute = 238

func (s *Site) render(d *Doc) {
	r := NewRenderer(s)
	r.newest = d.Meta["order"] == "newest"
	d.Content = template.HTML(r.Render(d.body))
	d.Math, d.Mermaid = r.math, r.mermaid
	d.Thoughts = r.thoughts
	if t := d.Latest(); t != nil {
		d.LastThought = t.Time
		if t.Time.After(d.Updated) && t.Time.After(d.Date) {
			d.Updated = t.Time
		}
	}
	for _, w := range r.Warnings {
		s.warnf("%s: %s", d.Source, w)
	}
	d.Text = plainText(d.body)
	d.Words = len(strings.Fields(d.Text))
	d.ReadingTime = max(1, int(math.Round(float64(d.Words)/wordsPerMinute)))
	d.Description = d.Summary
	if d.Description == "" {
		d.Description = truncateWords(d.Text, 30)
	}
	if i := moreIndex(d.body); i >= 0 {
		er := NewRenderer(s)
		er.excerpt = true
		d.Excerpt = template.HTML(er.Render(d.body[:i]))
		d.HasMore = true
		d.excerptMath, d.excerptMermaid = er.math, er.mermaid
	} else if d.Summary != "" {
		d.Excerpt = template.HTML("<p>" + html.EscapeString(d.Summary) + "</p>")
	} else {
		d.Excerpt = template.HTML("<p>" + html.EscapeString(truncateWords(d.Text, 40)) + "</p>")
	}
}

func moreIndex(nodes []bt.Node) int {
	for i, n := range nodes {
		if b, ok := n.(bt.Bag); ok && strings.TrimSpace(b.Tag()) == "more" {
			return i
		}
	}
	return -1
}

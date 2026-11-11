// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	bt "barkdown/brashtag"
)

// Renderer turns the body of one document into HTML.
type Renderer struct {
	site     *Site // for links between documents and the base path; may be nil
	excerpt  bool  // rendering an excerpt: no footnotes, no table of contents
	chunks   map[string]bool
	chunk    string // the literate chunk being rendered, if any
	ids      map[string]int
	headings []heading
	notes    int // sidenotes so far
	thoughts []*Thought
	newest   bool // show timelines newest first
	margins  int  // margin notes so far
	math     bool
	mermaid  bool
	Warnings []string
}

type heading struct {
	level    int
	id, text string
}

func NewRenderer(site *Site) *Renderer {
	return &Renderer{site: site, chunks: map[string]bool{}, ids: map[string]int{}}
}

func (r *Renderer) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

func (r *Renderer) uniqueID(base string) string {
	r.ids[base]++
	if n := r.ids[base]; n > 1 {
		return fmt.Sprintf("%s-%d", base, n)
	}
	return base
}

const tocMark = "toc"

// Render renders the body of a document.
func (r *Renderer) Render(nodes []bt.Node) string {
	r.findChunks(nodes)
	out := r.flow(nodes)
	if strings.Contains(out, tocMark) {
		out = strings.ReplaceAll(out, tocMark, r.toc())
	}
	return out
}

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// flow renders nodes in which blank lines separate paragraphs.
func (r *Renderer) flow(nodes []bt.Node) string {
	var out, para strings.Builder
	flush := func() {
		if s := strings.TrimSpace(para.String()); s != "" {
			out.WriteString("<p>" + s + "</p>\n")
		}
		para.Reset()
	}
	for i := 0; i < len(nodes); i++ {
		switch x := nodes[i].(type) {
		case bt.Blob:
			for k, part := range blankLine.Split(x.Text(), -1) {
				if k > 0 {
					flush()
				}
				para.WriteString(html.EscapeString(part))
			}
		case bt.Code:
			if isFence(x) {
				flush()
				out.WriteString(r.fence(x) + "\n")
			} else {
				para.WriteString(r.inlineCode(x))
			}
		case bt.Bag:
			if isThought(x) {
				end := thoughtRun(nodes, i)
				flush()
				out.WriteString(r.timeline(nodes[i:end]) + "\n")
				i = end - 1
				continue
			}
			h, block := r.bag(x)
			if block {
				flush()
				if h != "" {
					out.WriteString(h + "\n")
				}
			} else {
				para.WriteString(h)
			}
		}
	}
	flush()
	return out.String()
}

// inline renders nodes as one run of text, without paragraphs.
func (r *Renderer) inline(nodes []bt.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch x := n.(type) {
		case bt.Blob:
			b.WriteString(html.EscapeString(x.Text()))
		case bt.Code:
			if isFence(x) {
				b.WriteString(r.fence(x))
			} else {
				b.WriteString(r.inlineCode(x))
			}
		case bt.Bag:
			h, _ := r.bag(x)
			b.WriteString(h)
		}
	}
	return strings.TrimSpace(b.String())
}

func (r *Renderer) compactFlow(nodes []bt.Node) string {
	out := strings.TrimSpace(r.flow(nodes))
	if strings.HasPrefix(out, "<p>") && strings.HasSuffix(out, "</p>") && strings.Count(out, "<p>") == 1 {
		return out[3 : len(out)-4]
	}
	return out
}

func (r *Renderer) inlineCode(c bt.Code) string {
	t := c.Text()
	if len(c.Tag()) == 1 && len(t) >= 2 && t[0] == '$' && t[len(t)-1] == '$' {
		r.math = true
		return `<span class="math">\(` + html.EscapeString(t[1:len(t)-1]) + `\)</span>`
	}
	return "<code>" + html.EscapeString(t) + "</code>"
}

type handler struct {
	block bool
	fn    func(r *Renderer, b bt.Bag) string
}

var handlers map[string]handler

// bag renders one bag and says whether the result is a block element.
func (r *Renderer) bag(b bt.Bag) (string, bool) {
	tag := strings.TrimSpace(b.Tag())
	if h, ok := handlers[tag]; ok {
		return h.fn(r, b), h.block
	}
	return r.unknown(b)
}

func init() {
	handlers = map[string]handler{}
	for _, tag := range []string{"b", "i", "em", "strong", "u", "s", "del", "ins", "sup", "sub",
		"small", "mark", "kbd", "q", "cite", "span", "var", "samp", "dfn"} {
		el := tag
		handlers[tag] = handler{false, func(r *Renderer, b bt.Bag) string {
			return "<" + el + ">" + r.inline(b.Kids()) + "</" + el + ">"
		}}
	}
	container := func(tag, el, class string) {
		open := "<" + el + ">"
		if class != "" {
			open = "<" + el + ` class="` + class + `">`
		}
		handlers[tag] = handler{true, func(r *Renderer, b bt.Bag) string {
			return open + "\n" + r.flow(b.Kids()) + "</" + el + ">"
		}}
	}
	container("post", "div", "post")
	container("div", "div", "")
	container("center", "div", "center")
	container("section", "section", "")
	container("article", "article", "")
	container("aside", "aside", "")
	container("header", "header", "")
	container("footer", "footer", "")
	container("blockquote", "blockquote", "")
	container("quote", "blockquote", "")
	container("li", "li", "")
	container("dl", "dl", "")
	container("dd", "dd", "")
	handlers["dt"] = handler{true, func(r *Renderer, b bt.Bag) string { return "<dt>" + r.inline(b.Kids()) + "</dt>" }}
	handlers["p"] = handler{true, func(r *Renderer, b bt.Bag) string { return "<p>" + r.inline(b.Kids()) + "</p>" }}
	handlers["title"] = handler{true, func(r *Renderer, b bt.Bag) string { return "<h1>" + r.inline(b.Kids()) + "</h1>" }}
	handlers["subtitle"] = handler{true, func(r *Renderer, b bt.Bag) string {
		return `<p class="subtitle">` + r.inline(b.Kids()) + "</p>"
	}}
	handlers["by"] = handler{true, func(r *Renderer, b bt.Bag) string {
		return `<footer class="by">— ` + r.inline(b.Kids()) + "</footer>"
	}}
	handlers["br"] = handler{false, func(*Renderer, bt.Bag) string { return "<br>" }}
	handlers["hr"] = handler{true, func(*Renderer, bt.Bag) string { return "<hr>" }}
	handlers["comment"] = handler{false, func(*Renderer, bt.Bag) string { return "" }}
	handlers["more"] = handler{true, func(*Renderer, bt.Bag) string { return "" }}
	for level := 1; level <= 6; level++ {
		handlers[fmt.Sprintf("h%d", level)] = headingHandler(level)
	}
	handlers["toc"] = handler{true, func(r *Renderer, b bt.Bag) string {
		if r.excerpt {
			return ""
		}
		return tocMark
	}}
	handlers["list"] = handler{true, func(r *Renderer, b bt.Bag) string { return r.list(b, "ul", ",") }}
	handlers["ul"] = handler{true, func(r *Renderer, b bt.Bag) string { return r.list(b, "ul", "\n") }}
	handlers["ol"] = handler{true, func(r *Renderer, b bt.Bag) string { return r.list(b, "ol", "\n") }}
	handlers["table"] = handler{true, (*Renderer).table}
	handlers["tr"] = handler{true, func(r *Renderer, b bt.Bag) string { return "<tr>" + r.inline(b.Kids()) + "</tr>" }}
	handlers["td"] = handler{false, func(r *Renderer, b bt.Bag) string { return "<td>" + r.inline(b.Kids()) + "</td>" }}
	handlers["th"] = handler{false, func(r *Renderer, b bt.Bag) string { return "<th>" + r.inline(b.Kids()) + "</th>" }}
	handlers["link"] = handler{false, (*Renderer).link}
	handlers["a"] = handler{false, (*Renderer).link}
	handlers["ref"] = handler{false, (*Renderer).ref}
	handlers["img"] = handler{false, (*Renderer).image}
	handlers["image"] = handler{false, (*Renderer).image}
	handlers["figure"] = handler{true, (*Renderer).figure}
	handlers["video"] = handler{true, media("video")}
	handlers["audio"] = handler{true, media("audio")}
	handlers["youtube"] = handler{true, (*Renderer).youtube}
	handlers["abbr"] = handler{false, (*Renderer).abbr}
	handlers["fn"] = handler{false, (*Renderer).footnote}
	handlers["footnote"] = handler{false, (*Renderer).footnote}
	handlers["sidenote"] = handler{false, (*Renderer).footnote}
	handlers["marginnote"] = handler{false, (*Renderer).marginnote}
	handlers["newthought"] = handler{false, func(r *Renderer, b bt.Bag) string {
		return `<span class="newthought">` + r.inline(b.Kids()) + "</span>"
	}}
	handlers["epigraph"] = handler{true, func(r *Renderer, b bt.Bag) string {
		return "<div class=\"epigraph\"><blockquote>\n" + r.flow(b.Kids()) + "</blockquote></div>"
	}}
	handlers["details"] = handler{true, (*Renderer).details}
	handlers["thought"] = handler{true, func(r *Renderer, b bt.Bag) string { return r.timeline([]bt.Node{b}) }}
	handlers["tweet"] = handlers["thought"]
	handlers["summary"] = handler{true, func(r *Renderer, b bt.Bag) string {
		return "<summary>" + r.inline(b.Kids()) + "</summary>"
	}}
	for _, c := range [][2]string{{"note", "Note"}, {"tip", "Tip"}, {"info", "Info"},
		{"warning", "Warning"}, {"danger", "Danger"}, {"important", "Important"}} {
		handlers[c[0]] = callout(c[0], c[1])
	}
	handlers["code"] = handler{false, func(r *Renderer, b bt.Bag) string {
		return "<code>" + html.EscapeString(textOf(b)) + "</code>"
	}}
	handlers["pre"] = handler{true, func(r *Renderer, b bt.Bag) string {
		return "<pre>" + html.EscapeString(dedent(textOf(b))) + "</pre>"
	}}
	handlers["html"] = handler{true, func(r *Renderer, b bt.Bag) string { return textOf(b) }}
	handlers["math"] = handler{true, func(r *Renderer, b bt.Bag) string {
		r.math = true
		return `<div class="math">\[` + html.EscapeString(strings.TrimSpace(textOf(b))) + `\]</div>`
	}}
	handlers["mermaid"] = handler{true, func(r *Renderer, b bt.Bag) string {
		r.mermaid = true
		return `<pre class="mermaid">` + html.EscapeString(dedent(textOf(b))) + "</pre>"
	}}
}

func headingHandler(level int) handler {
	return handler{true, func(r *Renderer, b bt.Bag) string {
		inner := r.inline(b.Kids())
		text := strings.Join(strings.Fields(textOf(b)), " ")
		base := slugify(text)
		if base == "" {
			base = "section"
		}
		id := r.uniqueID(base)
		if level >= 2 && level <= 4 {
			r.headings = append(r.headings, heading{level, id, html.EscapeString(text)})
		}
		return fmt.Sprintf(`<h%d id="%s">%s <a class="anchor" href="#%s" aria-label="Link to this section">#</a></h%d>`,
			level, id, inner, id, level)
	}}
}

func (r *Renderer) toc() string {
	if len(r.headings) == 0 {
		return ""
	}
	top := 6
	for _, h := range r.headings {
		top = min(top, h.level)
	}
	var b strings.Builder
	id := r.uniqueID("contents")
	fmt.Fprintf(&b, `<nav class="toc" id="%s" aria-label="In this post"><p class="toc-title">In this post</p>`, id)
	open := 0
	for i, h := range r.headings {
		depth := h.level - top + 1
		switch {
		case i == 0 || depth > open:
			for open < depth {
				b.WriteString("<ul><li>")
				open++
			}
		default:
			for open > depth {
				b.WriteString("</li></ul>")
				open--
			}
			b.WriteString("</li><li>")
		}
		fmt.Fprintf(&b, `<a href="#%s">%s</a>`, h.id, h.text)
	}
	for ; open > 0; open-- {
		b.WriteString("</li></ul>")
	}
	b.WriteString("</nav>")
	fmt.Fprintf(&b, `<a class="goto-toc" href="#%s">Back to contents</a>`, id)
	return b.String()
}

var bulletMark = regexp.MustCompile(`^\s*(?:[-*+•]|\d+[.)])\s+`)

func (r *Renderer) list(b bt.Bag, el, sep string) string {
	var items []string
	if hasBag(b.Kids(), "li") {
		for _, k := range b.Kids() {
			if li, ok := k.(bt.Bag); ok && strings.TrimSpace(li.Tag()) == "li" {
				items = append(items, r.compactFlow(li.Kids()))
			}
		}
	} else {
		for _, group := range splitNodes(b.Kids(), sep) {
			group = trimNodes(group)
			if len(group) == 0 {
				continue
			}
			if sep == "\n" {
				if first, ok := group[0].(bt.Blob); ok {
					group[0] = bt.NewBlob(bulletMark.ReplaceAllString(first.Text(), ""))
				}
			}
			if len(items) > 0 && isBlockBag(group) {
				items[len(items)-1] += "\n" + r.inline(group)
				continue
			}
			items = append(items, r.inline(group))
		}
	}
	var out strings.Builder
	out.WriteString("<" + el + ">\n")
	for _, it := range items {
		out.WriteString("<li>" + it + "</li>\n")
	}
	out.WriteString("</" + el + ">")
	return out.String()
}

// splitNodes splits nodes at every sep inside a blob; other nodes stay whole.
func splitNodes(nodes []bt.Node, sep string) [][]bt.Node {
	groups := [][]bt.Node{nil}
	for _, n := range nodes {
		blob, ok := n.(bt.Blob)
		if !ok {
			groups[len(groups)-1] = append(groups[len(groups)-1], n)
			continue
		}
		for i, part := range strings.Split(blob.Text(), sep) {
			if i > 0 {
				groups = append(groups, nil)
			}
			if part != "" {
				groups[len(groups)-1] = append(groups[len(groups)-1], bt.NewBlob(part))
			}
		}
	}
	return groups
}

// trimNodes removes white space at both ends of a node list.
func trimNodes(nodes []bt.Node) []bt.Node {
	blank := func(n bt.Node) bool {
		b, ok := n.(bt.Blob)
		return ok && strings.TrimSpace(b.Text()) == ""
	}
	for len(nodes) > 0 && blank(nodes[0]) {
		nodes = nodes[1:]
	}
	for len(nodes) > 0 && blank(nodes[len(nodes)-1]) {
		nodes = nodes[:len(nodes)-1]
	}
	if len(nodes) == 0 {
		return nil
	}
	out := append([]bt.Node(nil), nodes...)
	if b, ok := out[0].(bt.Blob); ok {
		out[0] = bt.NewBlob(strings.TrimLeftFunc(b.Text(), unicode.IsSpace))
	}
	if b, ok := out[len(out)-1].(bt.Blob); ok {
		out[len(out)-1] = bt.NewBlob(strings.TrimRightFunc(b.Text(), unicode.IsSpace))
	}
	return out
}

func hasBag(nodes []bt.Node, tag string) bool {
	for _, n := range nodes {
		if b, ok := n.(bt.Bag); ok && strings.TrimSpace(b.Tag()) == tag {
			return true
		}
	}
	return false
}

// isBlockBag reports whether nodes is a single bag that renders as a block.
func isBlockBag(nodes []bt.Node) bool {
	if len(nodes) != 1 {
		return false
	}
	b, ok := nodes[0].(bt.Bag)
	return ok && handlers[strings.TrimSpace(b.Tag())].block
}

func words(nodes []bt.Node) string {
	return strings.Join(strings.Fields(textOfNodes(nodes)), " ")
}

var separatorRow = regexp.MustCompile(`^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?$`)

func (r *Renderer) table(b bt.Bag) string {
	var caption string
	var kids []bt.Node
	for _, k := range b.Kids() {
		if c, ok := k.(bt.Bag); ok && strings.TrimSpace(c.Tag()) == "caption" {
			caption = r.inline(c.Kids())
			continue
		}
		kids = append(kids, k)
	}
	var out strings.Builder
	out.WriteString(`<div class="table"><table>`)
	if caption != "" {
		out.WriteString("<caption>" + caption + "</caption>")
	}
	if hasBag(kids, "tr") {
		out.WriteString(r.inline(kids))
	} else {
		rows, header := r.tableRows(kids)
		if header {
			out.WriteString("<thead>" + tableRow(rows[0], "th") + "</thead>")
			rows = rows[1:]
		}
		out.WriteString("<tbody>")
		for _, row := range rows {
			out.WriteString(tableRow(row, "td"))
		}
		out.WriteString("</tbody>")
	}
	out.WriteString("</table></div>")
	return out.String()
}

func (r *Renderer) tableRows(nodes []bt.Node) (rows [][]string, header bool) {
	for _, line := range splitNodes(nodes, "\n") {
		line = trimNodes(line)
		if len(line) == 0 {
			continue
		}
		plain := textOfNodes(line)
		if separatorRow.MatchString(plain) {
			if len(rows) == 1 {
				header = true
			}
			continue
		}
		cells := splitNodes(line, "|")
		if strings.HasPrefix(plain, "|") && len(cells) > 1 {
			cells = cells[1:]
		}
		if strings.HasSuffix(plain, "|") && len(cells) > 1 {
			cells = cells[:len(cells)-1]
		}
		var row []string
		for _, c := range cells {
			row = append(row, r.inline(trimNodes(c)))
		}
		rows = append(rows, row)
	}
	return rows, header
}

func tableRow(cells []string, el string) string {
	var b strings.Builder
	b.WriteString("<tr>")
	for _, c := range cells {
		b.WriteString("<" + el + ">" + c + "</" + el + ">")
	}
	b.WriteString("</tr>\n")
	return b.String()
}

// firstWord splits off the first word of a bag's content.
func firstWord(nodes []bt.Node) (string, []bt.Node) {
	for i, n := range nodes {
		switch x := n.(type) {
		case bt.Blob:
			t := strings.TrimLeftFunc(x.Text(), unicode.IsSpace)
			if t == "" {
				continue
			}
			j := strings.IndexFunc(t, unicode.IsSpace)
			if j < 0 {
				return t, nodes[i+1:]
			}
			return t[:j], append([]bt.Node{bt.NewBlob(t[j:])}, nodes[i+1:]...)
		case bt.Code:
			return x.Text(), nodes[i+1:]
		default:
			return "", nodes[i:]
		}
	}
	return "", nil
}

func isExternal(u string) bool {
	return strings.Contains(u, "://") || strings.HasPrefix(u, "//") ||
		strings.HasPrefix(u, "mailto:") || strings.HasPrefix(u, "tel:")
}

func (r *Renderer) url(p string) string {
	if r.site != nil {
		return r.site.URL(p)
	}
	return p
}

func (r *Renderer) link(b bt.Bag) string {
	href, rest := firstWord(b.Kids())
	label := r.inline(rest)
	if href == "" {
		r.warnf("#link{} without a URL")
		return label
	}
	if label == "" {
		label = html.EscapeString(href)
	}
	attrs := ""
	if isExternal(href) {
		attrs = ` class="external" rel="noopener"`
	}
	return fmt.Sprintf(`<a href="%s"%s>%s</a>`, html.EscapeString(r.url(href)), attrs, label)
}

func (r *Renderer) ref(b bt.Bag) string {
	slug, rest := firstWord(b.Kids())
	label := r.inline(rest)
	frag := ""
	if i := strings.IndexByte(slug, '#'); i >= 0 {
		slug, frag = slug[:i], slug[i:]
	}
	var target *Doc
	if r.site != nil {
		target = r.site.bySlug[slug]
	}
	if target == nil {
		r.warnf("#ref{%s}: no post or page has this slug", slug)
		if label == "" {
			label = html.EscapeString(slug)
		}
		return `<a class="broken">` + label + `</a>`
	}
	if label == "" {
		label = html.EscapeString(target.Title)
	}
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(r.url(target.URL)+frag), label)
}

func (r *Renderer) image(b bt.Bag) string {
	src, rest := firstWord(b.Kids())
	return fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy">`,
		html.EscapeString(r.url(src)), html.EscapeString(words(rest)))
}

func (r *Renderer) figure(b bt.Bag) string {
	src, rest := firstWord(b.Kids())
	caption := r.inline(rest)
	if caption != "" {
		caption = "<figcaption>" + caption + "</figcaption>"
	}
	return fmt.Sprintf(`<figure><img src="%s" alt="%s" loading="lazy">%s</figure>`,
		html.EscapeString(r.url(src)), html.EscapeString(words(rest)), caption)
}

func media(el string) func(*Renderer, bt.Bag) string {
	return func(r *Renderer, b bt.Bag) string {
		src, rest := firstWord(b.Kids())
		out := fmt.Sprintf(`<figure class="media"><%s controls preload="metadata" src="%s"></%s>`,
			el, html.EscapeString(r.url(src)), el)
		if caption := r.inline(rest); caption != "" {
			out += "<figcaption>" + caption + "</figcaption>"
		}
		return out + "</figure>"
	}
}

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{6,}$`)

func (r *Renderer) youtube(b bt.Bag) string {
	id, rest := firstWord(b.Kids())
	if !youtubeID.MatchString(id) {
		r.warnf("#youtube{%s}: not a video id", id)
		return ""
	}
	title := words(rest)
	if title == "" {
		title = "YouTube video"
	}
	return fmt.Sprintf(`<div class="embed"><iframe src="https://www.youtube-nocookie.com/embed/%s" title="%s" `+
		`loading="lazy" allow="encrypted-media; picture-in-picture" allowfullscreen></iframe></div>`,
		id, html.EscapeString(title))
}

func (r *Renderer) abbr(b bt.Bag) string {
	word, rest := firstWord(b.Kids())
	return fmt.Sprintf(`<abbr title="%s">%s</abbr>`, html.EscapeString(words(rest)), html.EscapeString(word))
}

func (r *Renderer) footnote(b bt.Bag) string {
	if r.excerpt {
		return ""
	}
	r.notes++
	n := r.notes
	return fmt.Sprintf(`<label for="sn-%d" class="margin-toggle sidenote-number">%d</label>`+
		`<input type="checkbox" id="sn-%d" class="margin-toggle">`+
		`<span class="sidenote"><span class="sidenote-mark">%d</span> %s</span>`, n, n, n, n, r.inline(b.Kids()))
}

func (r *Renderer) marginnote(b bt.Bag) string {
	if r.excerpt {
		return ""
	}
	r.margins++
	n := r.margins
	return fmt.Sprintf(`<label for="mn-%d" class="margin-toggle" aria-label="Show note">&#8853;</label>`+
		`<input type="checkbox" id="mn-%d" class="margin-toggle">`+
		`<span class="marginnote">%s</span>`, n, n, r.inline(b.Kids()))
}

func callout(kind, label string) handler {
	return handler{true, func(r *Renderer, b bt.Bag) string {
		title := label
		var kids []bt.Node
		for _, k := range b.Kids() {
			if t, ok := k.(bt.Bag); ok && strings.TrimSpace(t.Tag()) == "title" {
				title = r.inline(t.Kids())
				continue
			}
			kids = append(kids, k)
		}
		return fmt.Sprintf(`<aside class="callout callout-%s"><p class="callout-title">%s</p>%s</aside>`,
			kind, title, r.flow(kids))
	}}
}

func (r *Renderer) details(b bt.Bag) string {
	summary := "Details"
	var kids []bt.Node
	for _, k := range b.Kids() {
		if s, ok := k.(bt.Bag); ok && strings.TrimSpace(s.Tag()) == "summary" {
			summary = r.inline(s.Kids())
			continue
		}
		kids = append(kids, k)
	}
	return "<details><summary>" + summary + "</summary>\n" + r.flow(kids) + "</details>"
}

func isFence(c bt.Code) bool { return len(c.Tag()) >= 3 }

var langName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+#.-]*$`)

func splitLang(text string) (lang, code string) {
	if nl := strings.IndexByte(text, '\n'); nl > 0 && langName.MatchString(text[:nl]) {
		return strings.ToLower(text[:nl]), text[nl+1:]
	}
	return "", strings.TrimPrefix(text, "\n")
}

func (r *Renderer) fence(c bt.Code) string {
	lang, code := splitLang(c.Text())
	code = dedent(code)
	switch lang {
	case "math", "latex", "tex":
		r.math = true
		return `<div class="math">\[` + html.EscapeString(code) + `\]</div>`
	case "mermaid":
		r.mermaid = true
		return `<pre class="mermaid">` + html.EscapeString(code) + "</pre>"
	}
	label, head, class := "", "", ""
	if r.chunk != "" {
		label = fmt.Sprintf(`<div class="chunk-label"><a class="chunk-name" href="#%s">⟨%s⟩</a></div>`,
			chunkID(r.chunk), html.EscapeString(r.chunk))
	}
	if lang != "" {
		head = fmt.Sprintf(`<div class="code-head"><span class="lang">%s</span></div>`, html.EscapeString(lang))
		class = fmt.Sprintf(` class="language-%s"`, html.EscapeString(lang))
	}
	return fmt.Sprintf(`<div class="codeblock">%s%s<pre><code%s>%s</code></pre></div>`,
		label, head, class, r.codeHTML(code, lang))
}

// dedent removes the indentation common to all non-blank lines and the
// blank lines around the code.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	prefix, first := "", true
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		ws := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		if first {
			prefix, first = ws, false
			continue
		}
		for !strings.HasPrefix(ws, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
		} else {
			lines[i] = l[len(prefix):]
		}
	}
	return strings.Join(lines, "\n")
}

var refLine = regexp.MustCompile(`^(\s*)<<<(.+?)>>>\s*$`)

func (r *Renderer) codeHTML(code, lang string) string {
	var refs []string
	lines := strings.Split(code, "\n")
	for i, l := range lines {
		if m := refLine.FindStringSubmatch(l); m != nil && r.chunks[m[2]] {
			lines[i] = m[1] + string(rune(0xF0000+len(refs)))
			refs = append(refs, m[2])
		}
	}
	out := Highlight(strings.Join(lines, "\n"), lang)
	for i, name := range refs {
		link := fmt.Sprintf(`<a class="chunkref" href="#%s">⟨%s⟩</a>`,
			chunkID(name), html.EscapeString(name))
		out = strings.Replace(out, string(rune(0xF0000+i)), link, 1)
	}
	return out
}

// Thought is one timestamped entry of a post that grows over time.
type Thought struct {
	Time  time.Time
	Clock bool // the timestamp includes a time of day
	ID    string
	HTML  template.HTML
	Text  string
}

func isThought(n bt.Node) bool {
	b, ok := n.(bt.Bag)
	if !ok {
		return false
	}
	tag := strings.TrimSpace(b.Tag())
	return tag == "thought" || tag == "tweet"
}

// thoughtRun returns the end of the run of thoughts that starts at i. White
// space between two thoughts belongs to the run; anything else ends it.
func thoughtRun(nodes []bt.Node, i int) int {
	end := i
	for j := i; j < len(nodes); j++ {
		if isThought(nodes[j]) {
			end = j + 1
			continue
		}
		if b, ok := nodes[j].(bt.Blob); ok && strings.TrimSpace(b.Text()) == "" {
			continue
		}
		break
	}
	return end
}

var clockWord = regexp.MustCompile(`^\d{1,2}:\d{2}$`)

func (r *Renderer) parseThought(b bt.Bag) (*Thought, []bt.Node) {
	t := &Thought{}
	day, rest := firstWord(b.Kids())
	stamp := day
	if clock, after := firstWord(rest); clockWord.MatchString(clock) {
		stamp, rest, t.Clock = day+" "+clock, after, true
	}
	when, err := parseDate(stamp)
	if err != nil {
		r.warnf("#thought{%s ...}: a thought starts with its date, such as 2026-11-11 or 2026-11-11 09:30", stamp)
		return t, b.Kids()
	}
	t.Time = when
	return t, rest
}

func (r *Renderer) stamp(t *Thought) string {
	layout := "2 January 2006"
	if r.site != nil {
		layout = r.site.DateFormat
	}
	return formatStamp(t, layout)
}

func formatStamp(t *Thought, layout string) string {
	if t.Time.IsZero() {
		return "undated"
	}
	if t.Clock {
		return t.Time.Format(layout + ", 15:04")
	}
	return t.Time.Format(layout)
}

func (r *Renderer) timeline(nodes []bt.Node) string {
	type item struct {
		t    *Thought
		body []bt.Node
	}
	var items []item
	for _, n := range nodes {
		if isThought(n) {
			t, body := r.parseThought(n.(bt.Bag))
			items = append(items, item{t, body})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].t.Time.Before(items[j].t.Time) })
	order := "oldest"
	if r.newest {
		order = "newest"
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<section class=\"timeline\" data-order=\"%s\">\n", order)
	count := "1 thought"
	if len(items) != 1 {
		count = fmt.Sprintf("%d thoughts", len(items))
	}
	fmt.Fprintf(&b, "<p class=\"timeline-head\"><span>%s</span></p>\n", count)
	for _, it := range items {
		t := it.t
		id := "t"
		if !t.Time.IsZero() {
			id = "t-" + t.Time.Format("2006-01-02")
			if t.Clock {
				id += t.Time.Format("-1504")
			}
		}
		t.ID = r.uniqueID(id)
		body := r.flow(it.body)
		t.HTML, t.Text = template.HTML(body), plainText(it.body)
		if !r.excerpt {
			r.thoughts = append(r.thoughts, t)
		}
		fmt.Fprintf(&b, "<article class=\"thought\" id=\"%s\">\n<p class=\"thought-stamp\"><a href=\"#%s\"><time datetime=\"%s\">%s</time></a></p>\n%s</article>\n",
			t.ID, t.ID, t.Time.Format(time.RFC3339), html.EscapeString(r.stamp(t)), body)
	}
	b.WriteString("</section>")
	return b.String()
}

func chunkID(name string) string { return "chunk-" + slugify(name) }

func (r *Renderer) findChunks(nodes []bt.Node) {
	for _, n := range nodes {
		b, ok := n.(bt.Bag)
		if !ok {
			continue
		}
		tag := strings.TrimSpace(b.Tag())
		if _, known := handlers[tag]; !known && tag != "" && hasFence(b.Kids()) {
			r.chunks[tag] = true
		}
		r.findChunks(b.Kids())
	}
}

func (r *Renderer) unknown(b bt.Bag) (string, bool) {
	tag := strings.TrimSpace(b.Tag())
	switch {
	case tag == "":
		return r.flow(b.Kids()), true
	case r.chunks[tag]:
		outer := r.chunk
		r.chunk = tag
		inner := r.flow(b.Kids())
		r.chunk = outer
		return fmt.Sprintf(`<div class="chunk" id="%s">%s</div>`, r.uniqueID(chunkID(tag)), inner), true
	}
	r.warnf("unknown tag #%s{}, rendered with class bt-%s", tag, slugify(tag))
	class := "bt-" + slugify(tag)
	if hasBlock(b.Kids()) {
		return fmt.Sprintf(`<div class="%s">%s</div>`, class, r.flow(b.Kids())), true
	}
	return fmt.Sprintf(`<span class="%s">%s</span>`, class, r.inline(b.Kids())), false
}

func hasFence(nodes []bt.Node) bool {
	for _, n := range nodes {
		if c, ok := n.(bt.Code); ok && isFence(c) {
			return true
		}
	}
	return false
}

// hasBlock reports whether nodes contain a paragraph break or a block.
func hasBlock(nodes []bt.Node) bool {
	for _, n := range nodes {
		switch x := n.(type) {
		case bt.Blob:
			if blankLine.MatchString(x.Text()) {
				return true
			}
		case bt.Code:
			if isFence(x) {
				return true
			}
		case bt.Bag:
			if h, ok := handlers[strings.TrimSpace(x.Tag())]; !ok || h.block {
				return true
			}
		}
	}
	return false
}

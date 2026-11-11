// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func renderBody(t *testing.T, src string) (string, *Renderer) {
	t.Helper()
	d, err := ParseDoc("test.bt", src)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(nil)
	return r.Render(d.body), r
}

func TestRenderExact(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{"paragraphs", "Hello\nthere\n\nWorld", "<p>Hello\nthere</p>\n<p>World</p>\n"},
		{"escaping", "a < b & c > d", "<p>a &lt; b &amp; c &gt; d</p>\n"},
		{"inline tags", "x #b{y} #em{z}", "<p>x <b>y</b> <em>z</em></p>\n"},
		{"explicit paragraph", "#p{hi}", "<p>hi</p>\n"},
		{"comment", "a#comment{gone}b", "<p>ab</p>\n"},
		{"barkdown list", "#list{One, Two,\n Three}", "<ul>\n<li>One</li>\n<li>Two</li>\n<li>Three</li>\n</ul>\n"},
		{"line list", "#ul{\n- a\n- b `c, d`\n}", "<ul>\n<li>a</li>\n<li>b <code>c, d</code></li>\n</ul>\n"},
		{"ordered", "#ol{\n1. x\n2. y\n}", "<ol>\n<li>x</li>\n<li>y</li>\n</ol>\n"},
		{"external link", "#link{https://go.dev Go}",
			`<p><a href="https://go.dev" class="external" rel="noopener">Go</a></p>` + "\n"},
		{"link fragment in code", "#link{`/a/#b` c}", `<p><a href="/a/#b">c</a></p>` + "\n"},
		{"inline math", "`$x^2$`", `<p><span class="math">\(x^2\)</span></p>` + "\n"},
		{"image", "#img{cat.png A cat}", `<p><img src="cat.png" alt="A cat" loading="lazy"></p>` + "\n"},
		{"abbr", "#abbr{BT brash tag}", `<p><abbr title="brash tag">BT</abbr></p>` + "\n"},
		{"block between paragraphs", "a\n#hr{}\nb", "<p>a</p>\n<hr>\n<p>b</p>\n"},
	}
	for _, tt := range tests {
		got, _ := renderBody(t, tt.src)
		if got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestRenderContains(t *testing.T) {
	tests := []struct {
		name, src string
		want      []string
	}{
		{"table", "#table{\na | b\n--- | ---\n1 | `x|y`\n}", []string{
			"<thead><tr><th>a</th><th>b</th></tr>", "<td>1</td><td><code>x|y</code></td>"}},
		{"sidenotes", "x#fn{one} y#fn{two #b{b}}", []string{
			`<label for="sn-1" class="margin-toggle sidenote-number">1</label><input type="checkbox" id="sn-1" class="margin-toggle">`,
			`<span class="sidenote"><span class="sidenote-mark">2</span> two <b>b</b></span>`}},
		{"margin note", "a#marginnote{aside}", []string{`<label for="mn-1" class="margin-toggle"`, `<span class="marginnote">aside</span>`}},
		{"new thought", "#newthought{In the beginning} was", []string{`<p><span class="newthought">In the beginning</span> was</p>`}},
		{"epigraph", "#epigraph{Words.\n#by{Someone}}", []string{`<div class="epigraph"><blockquote>`, `<footer class="by">— Someone</footer>`}},
		{"toc", "#toc{}\n#h2{One}\n#h3{Two}\n#h2{One}", []string{
			`<h2 id="one">`, `<h2 id="one-2">`, `<ul><li><a href="#one">One</a><ul><li><a href="#two">Two</a>`,
			`<nav class="toc" id="contents"`, `</nav><a class="goto-toc" href="#contents">Back to contents</a>`}},
		{"highlighted go", "```go\nfunc f() {} // hi\n```", []string{
			`class="language-go"`, `<span class="tok-k">func</span>`, `<span class="tok-c">// hi</span>`}},
		{"literate chunks", "#main{\n```\n<<<helper>>>\n```\n}\n#helper{\n```\nx\n```\n}", []string{
			`<a class="chunkref" href="#chunk-helper">⟨helper⟩</a>`, `id="chunk-helper"`,
			`<a class="chunk-name" href="#chunk-main">⟨main⟩</a>`}},
		{"nested list", "#ul{\n- a\n#ul{\n- b\n}\n- c\n}", []string{"<li>a\n<ul>\n<li>b</li>\n</ul></li>", "<li>c</li>"}},
		{"dedent", "#div{\n  ```\n    x\n      y\n  ```\n}", []string{"<pre><code>x\n  y</code></pre>"}},
		{"callout", "#warning{#title{Careful}Hot.}", []string{`callout-warning`, `<p class="callout-title">Careful</p>`, "<p>Hot.</p>"}},
		{"details", "#details{#summary{More}Hidden}", []string{"<details><summary>More</summary>", "<p>Hidden</p>"}},
		{"figure", "#figure{a.png The #b{caption}}", []string{`alt="The caption"`, "<figcaption>The <b>caption</b></figcaption>"}},
		{"brashtag code", "```bt\n#b{x} `y`\n```", []string{`<span class="tok-k">#b{</span>x<span class="tok-k">}</span>`}},
	}
	for _, tt := range tests {
		got, _ := renderBody(t, tt.src)
		for _, w := range tt.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: output lacks %q\n%s", tt.name, w, got)
			}
		}
	}
}

func TestRendererState(t *testing.T) {
	_, r := renderBody(t, "#wat{x}")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "#wat") {
		t.Errorf("unknown tag: warnings %q", r.Warnings)
	}
	if _, r := renderBody(t, "#math{`x^2`}"); !r.math {
		t.Error("#math did not ask for MathJax")
	}
	if _, r := renderBody(t, "```mermaid\ngraph LR\n```"); !r.mermaid {
		t.Error("a mermaid block did not ask for Mermaid")
	}
	out, _ := renderBody(t, "#toc{}\n#h2{a}\n#h4{b}\n#h3{c}\n#h2{d}")
	if strings.Count(out, "<ul>") != strings.Count(out, "</ul>") || strings.Count(out, "<li>") != strings.Count(out, "</li>") {
		t.Errorf("unbalanced table of contents: %s", out)
	}
}

func TestThoughts(t *testing.T) {
	src := "#thought{2026-11-02 second}\n\n#tweet{2026-11-01 09:30 first\n\nmore}\n#thought{2026-11-03 third}"
	out, r := renderBody(t, src)
	a, b, c := strings.Index(out, `id="t-2026-11-01-0930"`), strings.Index(out, `id="t-2026-11-02"`), strings.Index(out, `id="t-2026-11-03"`)
	if a < 0 || !(a < b && b < c) || strings.Count(out, `<section class="timeline"`) != 1 || len(r.thoughts) != 3 {
		t.Errorf("thoughts not one sorted timeline:\n%s", out)
	}
	if !strings.Contains(out, `<a href="#t-2026-11-01-0930"><time datetime=`) || !strings.Contains(out, "<p>more</p>") {
		t.Errorf("stamp link or body missing:\n%s", out)
	}
	d, _ := ParseDoc("x.bt", src)
	nr := NewRenderer(nil)
	nr.newest = true
	if out := nr.Render(d.body); strings.Index(out, "t-2026-11-03") > strings.Index(out, "t-2026-11-01-0930") {
		t.Error("newest first did not reverse the timeline")
	}
	if got := plainText(d.body); got != "second first more third" {
		t.Errorf("plain text of thoughts: %q", got)
	}
	if _, r := renderBody(t, "#thought{yesterday hi}"); len(r.Warnings) != 1 {
		t.Errorf("a bad timestamp gave warnings %q", r.Warnings)
	}
}

func TestHighlightLanguages(t *testing.T) {
	tests := []struct{ lang, code, want string }{
		{"python", `"""doc"""` + "\nx = 1", `<span class="tok-s">&#34;&#34;&#34;doc&#34;&#34;&#34;</span>`},
		{"sh", "echo $HOME", `<span class="tok-v">$HOME</span>`},
		{"sql", "SELECT 1", `<span class="tok-k">SELECT</span> <span class="tok-n">1</span>`},
		{"go", "s := `a\nb`", "<span class=\"tok-s\">`a\nb`</span>"},
		{"unknown", "<x>", "&lt;x&gt;"},
	}
	for _, tt := range tests {
		if got := Highlight(tt.code, tt.lang); !strings.Contains(got, tt.want) {
			t.Errorf("%s: %q lacks %q", tt.lang, got, tt.want)
		}
	}
}

func TestMetadata(t *testing.T) {
	d, err := ParseDoc("x.bt", "#title{Hello\n  world}\n#date{2026-11-01 10:30}\n#tags{a, b, a}\n#draft{}\n#meta{#mood{fine}}\n\nBody")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Hello world" || d.Date.Format("2006-01-02 15:04") != "2026-11-01 10:30" || !d.Draft {
		t.Errorf("got title %q, date %v, draft %v", d.Title, d.Date, d.Draft)
	}
	if strings.Join(d.Tags, ",") != "a,b" || d.Meta["mood"] != "fine" {
		t.Errorf("got tags %q, meta %v", d.Tags, d.Meta)
	}
	if got := plainText(d.body); got != "Body" {
		t.Errorf("body %q", got)
	}
	if _, err := ParseDoc("x.bt", "#date{yesterday}"); err == nil {
		t.Error("a bad date was accepted")
	}
}

func TestNamesAndSlugs(t *testing.T) {
	slug, date := nameParts("posts/2026-11-01-hello-world.bt")
	if slug != "hello-world" || date.Format("2006-01-02") != "2026-11-01" {
		t.Errorf("got %q %v", slug, date)
	}
	if slug, _ := nameParts("posts/trip/index.bt"); slug != "trip" {
		t.Errorf("bundle slug %q", slug)
	}
	for in, want := range map[string]string{"Hello, World!": "hello-world", " Ünïcode  ok ": "ünïcode-ok", "---": "", "नमस्ते जग": "नमस्ते-जग"} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if got := quoteBT("a `b` #c"); got != "`` a `b` #c ``" {
		t.Errorf("quoteBT: %q", got)
	}
}

func TestParseErrorLines(t *testing.T) {
	tests := []struct {
		src  string
		line int
		msg  string
	}{
		{"line one\nline two }\nthree", 2, "unmatched '}'"},
		{"a\n\nb { c", 3, "'{' outside a tag"},
		{"a\n`code", 2, "code is not closed"},
		{"\n\nok #oops", 3, "no '{' follows"},
	}
	for _, tt := range tests {
		_, err := ParseDoc("t.bt", tt.src)
		want := fmt.Sprintf("t.bt:%d: ", tt.line)
		if err == nil || !strings.HasPrefix(err.Error(), want) || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%q: got %v, want %s...%s", tt.src, err, want, tt.msg)
		}
	}
}

var testNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local)

func TestBuildExample(t *testing.T) {
	out := t.TempDir()
	s, err := Build(Options{Src: "example", Out: out, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range s.Warnings {
		t.Errorf("warning: %s", w)
	}
	for _, f := range []string{"index.html", "posts/welcome/index.html", "posts/tour/index.html",
		"posts/tour/diagram.svg", "about/index.html", "tags/barkdown/index.html", "tags/index.html",
		"archive/index.html", "search/index.html", "timeline/index.html", "404.html", "feed.xml", "rss.xml", "sitemap.xml",
		"robots.txt", "search.json", "assets/style.css", "assets/site.js", "favicon.svg", ".nojekyll"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "posts/ideas")); err == nil {
		t.Error("the draft was published")
	}
	var feed atomFeed
	if err := xml.Unmarshal(readFile(t, out, "feed.xml"), &feed); err != nil || len(feed.Entries) != len(s.Posts) {
		t.Errorf("atom feed: %v, %d entries for %d posts", err, len(feed.Entries), len(s.Posts))
	}
	var rss rssFeed
	if err := xml.Unmarshal(readFile(t, out, "rss.xml"), &rss); err != nil || len(rss.Channel.Items) != len(s.Posts) {
		t.Errorf("rss feed: %v", err)
	}
	var index []searchEntry
	if err := json.Unmarshal(readFile(t, out, "search.json"), &index); err != nil || len(index) != len(s.Posts)+len(s.Pages) {
		t.Errorf("search index: %v, %d entries", err, len(index))
	}
	var thoughts atomFeed
	if err := xml.Unmarshal(readFile(t, out, "timeline/feed.xml"), &thoughts); err != nil || len(thoughts.Entries) != 4 {
		t.Errorf("thought feed: %v, %d entries", err, len(thoughts.Entries))
	}
	if len(s.Timeline) != 1 || s.Timeline[0].Updated.Format("2006-01-02 15:04") != "2026-11-09 07:45" {
		t.Errorf("timeline posts %v", s.Timeline)
	}
	if !strings.Contains(string(readFile(t, out, "timeline/index.html")), "On a phone there is no margin") {
		t.Error("the timeline page does not show the latest thought")
	}
	if _, err := Build(Options{Src: "example", Out: out, Now: testNow}); err != nil {
		t.Errorf("rebuilding: %v", err)
	}
	foreign := t.TempDir()
	os.WriteFile(filepath.Join(foreign, "precious.txt"), []byte("keep me"), 0o644)
	if _, err := Build(Options{Src: "example", Out: foreign, Now: testNow}); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Errorf("building into a foreign directory: %v", err)
	}
}

func readFile(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBuildUnderSubPath(t *testing.T) {
	out := t.TempDir()
	s, err := Build(Options{Src: "example", Out: out, BaseURL: "https://user.github.io/blog/", Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range s.Warnings {
		t.Errorf("warning: %s", w)
	}
	home := string(readFile(t, out, "index.html"))
	if !strings.Contains(home, `href="/blog/posts/welcome/"`) || strings.Contains(home, `href="/posts/`) {
		t.Error("links on the home page do not carry the base path")
	}
	if feed := string(readFile(t, out, "feed.xml")); !strings.Contains(feed, "https://user.github.io/blog/posts/welcome/") {
		t.Error("the feed lacks absolute URLs")
	}
}

func TestBrokenLinksAreReported(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "posts"), 0o755)
	os.WriteFile(filepath.Join(src, "posts", "2026-01-01-a.bt"), []byte("#title{A}\n#ref{nowhere} #link{/missing/ m} #link{`#nope` n}"), 0o644)
	s, err := Build(Options{Src: src, Out: filepath.Join(src, "public"), Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(s.Warnings, "\n")
	for _, want := range []string{"#ref{nowhere}", "broken link /missing/", "missing anchor #nope"} {
		if !strings.Contains(all, want) {
			t.Errorf("no warning %q in\n%s", want, all)
		}
	}
}

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	steps := [][]string{
		{"init", dir},
		{"new", "post", "-src", dir, "My `odd` #title"},
		{"new", "page", "-src", dir, "Contact"},
		{"build", "-src", dir, "-strict", "-quiet"},
	}
	for _, args := range steps {
		if code := run(args, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("barkdown %s: exit %d\n%s", strings.Join(args, " "), code, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "public", "contact", "index.html")); err != nil {
		t.Error("the new page was not built")
	}
	stdout.Reset()
	if code := run([]string{"render"}, strings.NewReader("#title{T}\n\nHi #b{there}"), &stdout, &stderr); code != 0 ||
		!strings.Contains(stdout.String(), "<h1>T</h1>") || !strings.Contains(stdout.String(), "<p>Hi <b>there</b></p>") {
		t.Errorf("render: exit %d\n%s", code, stdout.String())
	}
	if code := run([]string{"frobnicate"}, nil, &stdout, &stderr); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
}

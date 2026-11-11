// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

// Barkdown is a static site generator for documents written in brashtag.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const version = "1.0.0"

const usage = `barkdown, a static site generator for brashtag

usage:
  barkdown init [dir]                  create a new site with example content
  barkdown new post|page "Title"       start a new post (as a draft) or page
  barkdown build [flags]               build the site into public/
  barkdown serve [flags]               build, serve and rebuild on change
  barkdown render [file.bt]            convert one document to a standalone HTML page
  barkdown version

flags for build and serve:
  -src DIR       the site's directory (default .)
  -out DIR       where to write the site (default SRC/public; serve uses a temporary directory)
  -baseurl URL   override #baseurl{} from site.bt
  -drafts        include drafts (default for serve)
  -future        include posts dated in the future (default for serve)
  -strict        build: fail when there are warnings
  -quiet         build: print only warnings and errors
  -addr ADDR     serve: address to listen on (default localhost:8080)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, args := args[0], args[1:]
	var err error
	switch cmd {
	case "build":
		err = cmdBuild(args, stdout, stderr)
	case "serve":
		err = cmdServe(args, stdout, stderr)
	case "new":
		err = cmdNew(args, stdout, stderr)
	case "init":
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		err = initSite(dir, stdout)
	case "render":
		err = cmdRender(args, stdin, stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, "barkdown", version)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "barkdown: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errUsage):
		return 2
	default:
		fmt.Fprintln(stderr, "barkdown:", err)
		return 1
	}
}

var errUsage = errors.New("usage error")

type siteFlags struct {
	opts          Options
	strict, quiet bool
	addr          string
}

func parseFlags(cmd string, args []string, stderr io.Writer) (*siteFlags, error) {
	f := &siteFlags{}
	set := flag.NewFlagSet("barkdown "+cmd, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&f.opts.Src, "src", ".", "the site's directory")
	set.StringVar(&f.opts.Out, "out", "", "where to write the site")
	set.StringVar(&f.opts.BaseURL, "baseurl", "", "override #baseurl{} from site.bt")
	set.BoolVar(&f.opts.Drafts, "drafts", cmd == "serve", "include drafts")
	set.BoolVar(&f.opts.Future, "future", cmd == "serve", "include posts dated in the future")
	if cmd == "build" {
		set.BoolVar(&f.strict, "strict", false, "fail when there are warnings")
		set.BoolVar(&f.quiet, "quiet", false, "print only warnings and errors")
	} else {
		set.StringVar(&f.addr, "addr", "localhost:8080", "address to listen on")
	}
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, err
		}
		return nil, errUsage
	}
	if set.NArg() > 0 {
		fmt.Fprintf(stderr, "barkdown %s: unexpected argument %q\n", cmd, set.Arg(0))
		return nil, errUsage
	}
	if f.opts.Out == "" && cmd == "build" {
		f.opts.Out = filepath.Join(f.opts.Src, "public")
	}
	return f, nil
}

func cmdBuild(args []string, stdout, stderr io.Writer) error {
	f, err := parseFlags("build", args, stderr)
	if err != nil {
		return err
	}
	start := time.Now()
	s, err := Build(f.opts)
	if err != nil {
		return err
	}
	if f.quiet {
		stdout = io.Discard
	}
	report(stdout, stderr, s, time.Since(start))
	if f.strict && len(s.Warnings) > 0 {
		return fmt.Errorf("%d warning(s), and -strict was given", len(s.Warnings))
	}
	return nil
}

func report(stdout, stderr io.Writer, s *Site, took time.Duration) {
	for _, w := range s.Warnings {
		fmt.Fprintln(stderr, "warning:", w)
	}
	skipped := ""
	if s.Skipped > 0 {
		skipped = fmt.Sprintf(" (%d drafts or future posts left out)", s.Skipped)
	}
	fmt.Fprintf(stdout, "built %s, %s and %s into %s in %v%s: %s\n",
		plural(len(s.Posts), "post"), plural(len(s.Pages), "page"), plural(len(s.Tags), "tag"),
		plural(s.pages, "HTML page"), took.Round(time.Millisecond), skipped, s.opts.Out)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func cmdServe(args []string, stdout, stderr io.Writer) error {
	f, err := parseFlags("serve", args, stderr)
	if err != nil {
		return err
	}
	return serve(f.opts, f.addr, stdout, stderr)
}

func cmdNew(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, `usage: barkdown new post|page [-src DIR] "Title"`)
		return errUsage
	}
	set := flag.NewFlagSet("barkdown new", flag.ContinueOnError)
	set.SetOutput(stderr)
	src := set.String("src", ".", "the site's directory")
	if err := set.Parse(args[1:]); err != nil {
		return errUsage
	}
	title := strings.Join(set.Args(), " ")
	if title == "" {
		fmt.Fprintln(stderr, `usage: barkdown new post|page [-src DIR] "Title"`)
		return errUsage
	}
	return newDoc(*src, args[0], title, time.Now(), stdout)
}

func cmdRender(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	in, name := stdin, "stdin"
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		in, name = f, args[0]
	}
	warnings, err := renderFile(in, name, stdout)
	for _, w := range warnings {
		fmt.Fprintln(stderr, "warning:", w)
	}
	return err
}

//go:embed example
var exampleFS embed.FS

func initSite(dir string, stdout io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, "site.bt")); err == nil {
		return fmt.Errorf("%s already contains a site.bt", dir)
	}
	example, _ := fs.Sub(exampleFS, "example")
	err := fs.WalkDir(example, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(example, p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created a site in %s\n\n  cd %s\n  barkdown serve\n\n", dir, dir)
	return nil
}

func newDoc(src, kind, title string, now time.Time, stdout io.Writer) error {
	slug := slugify(title)
	if slug == "" {
		return fmt.Errorf("the title %q has no letters or digits to make a file name from", title)
	}
	var path, text string
	switch kind {
	case "post":
		path = filepath.Join(src, "posts", now.Format("2006-01-02")+"-"+slug+".bt")
		text = fmt.Sprintf("#title{%s}\n#date{%s}\n#tags{}\n#summary{}\n#draft{}\n\n"+
			"Start writing here. Remove `#draft{}` above when the post is ready.\n",
			quoteBT(title), now.Format("2006-01-02 15:04"))
	case "page":
		path = filepath.Join(src, "pages", slug+".bt")
		text = fmt.Sprintf("#title{%s}\n\nStart writing here.\n", quoteBT(title))
	default:
		return fmt.Errorf("barkdown new: make a post or a page, not %q", kind)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "created", path)
	return nil
}

// quoteBT protects text that contains brashtag's special characters.
func quoteBT(s string) string {
	if !strings.ContainsAny(s, "#{}`") {
		return s
	}
	run, longest := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	ticks := strings.Repeat("`", longest+1)
	return ticks + " " + s + " " + ticks
}

const standalone = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
%s</style>
%s</head>
<body>
<main class="wrap">
<article class="post">
%s<div class="content">
%s</div>
</article>
</main>
</body>
</html>
`

func renderFile(in io.Reader, name string, stdout io.Writer) ([]string, error) {
	src, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}
	d, err := ParseDoc(name, string(src))
	if err != nil {
		return nil, err
	}
	r := NewRenderer(nil)
	body := r.Render(d.body)
	css, _ := fs.ReadFile(themeFS, "theme/style.css")
	cfg := defaultConfig()
	var head strings.Builder
	if r.math {
		fmt.Fprintf(&head, "<script id=\"MathJax-script\" async src=\"%s\"></script>\n", cfg.MathJax)
	}
	if r.mermaid {
		fmt.Fprintf(&head, "<script type=\"module\">import mermaid from \"%s\";mermaid.initialize({startOnLoad:true});</script>\n", cfg.Mermaid)
	}
	title, header := name, ""
	if d.Title != "" {
		title = d.Title
		header = "<header class=\"post-header\"><h1>" + html.EscapeString(d.Title) + "</h1>"
		if d.Subtitle != "" {
			header += "<p class=\"subtitle\">" + html.EscapeString(d.Subtitle) + "</p>"
		}
		header += "</header>\n"
	}
	_, err = fmt.Fprintf(stdout, standalone, html.EscapeString(title), css, head.String(), header, body)
	return r.Warnings, err
}

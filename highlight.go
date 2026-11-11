// Code generated from barkdown.lit by tangling; edit barkdown.lit, not this file.

package main

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

// language describes enough of a language's lexical structure to colour it.
type language struct {
	keywords     map[string]bool
	builtins     map[string]bool
	lineComment  []string
	blockComment [][2]string
	quotes       string // characters that open a string
	rawQuotes    string // of those, the ones whose strings have no escapes
	triple       bool   // """ and ''' strings
	variables    bool   // $name and ${name}
	foldCase     bool   // keywords are case-insensitive
}

func wordSet(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var languages = map[string]*language{}

func init() {
	register := func(l *language, names ...string) {
		for _, n := range names {
			languages[n] = l
		}
	}
	slashes := []string{"//"}
	cblock := [][2]string{{"/*", "*/"}}
	hash := []string{"#"}

	register(&language{
		keywords: wordSet(`break case chan const continue default defer else fallthrough for func go goto
			if import interface map package range return select struct switch type var`),
		builtins: wordSet(`append cap clear close complex copy delete imag len make max min new panic print
			println real recover any bool byte comparable complex64 complex128 error float32 float64 int
			int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr true false nil iota`),
		lineComment: slashes, blockComment: cblock, quotes: "\"'`", rawQuotes: "`",
	}, "go", "golang")

	register(&language{
		keywords: wordSet(`and as assert async await break class continue def del elif else except finally
			for from global if import in is lambda nonlocal not or pass raise return try while with yield
			match case`),
		builtins: wordSet(`True False None self print len range int str float list dict set tuple bool
			open super isinstance enumerate zip map filter sorted sum min max abs any all type object
			Exception ValueError KeyError`),
		lineComment: hash, quotes: `"'`, triple: true,
	}, "python", "py", "python3")

	register(&language{
		keywords: wordSet(`break case catch class const continue debugger default delete do else export
			extends finally for function if import in instanceof let new of return static super switch this
			throw try typeof var void while with yield async await from as interface type enum implements`),
		builtins: wordSet(`true false null undefined NaN Infinity console document window Math JSON Object
			Array String Number Boolean Promise Map Set Error fetch`),
		lineComment: slashes, blockComment: cblock, quotes: "\"'`",
	}, "javascript", "js", "typescript", "ts", "jsx", "tsx", "mjs")

	cfamily := &language{
		keywords: wordSet(`auto break case char const continue default do double else enum extern float for
			goto if inline int long register return short signed sizeof static struct switch typedef union
			unsigned void volatile while class public private protected new delete this namespace using
			template typename virtual override try catch throw bool true false nullptr null import package
			extends implements interface final abstract boolean fn let mut impl trait pub use mod match loop
			where self Self crate move ref as dyn unsafe async await`),
		builtins: wordSet(`printf malloc free size_t uint8_t uint32_t int64_t std string vector cout endl
			String System Vec Option Some None Ok Err Box println`),
		lineComment: slashes, blockComment: cblock, quotes: `"'`,
	}
	register(cfamily, "c", "h", "cpp", "c++", "cc", "hpp", "java", "cs", "csharp", "kotlin", "swift", "zig")
	rust := *cfamily
	rust.quotes = `"`
	register(&rust, "rust", "rs")

	register(&language{
		keywords: wordSet(`if then else elif fi for while until do done case esac in function return exit
			export local readonly set unset shift break continue`),
		builtins: wordSet(`echo cd ls cat grep sed awk printf read test source alias go git make curl sudo
			mkdir rm cp mv chmod`),
		lineComment: hash, quotes: `"'`, rawQuotes: "'", variables: true,
	}, "sh", "bash", "zsh", "shell", "console")

	register(&language{
		keywords: wordSet(`select from where insert into values update set delete create table drop alter
			index join left right inner outer on group by order having limit offset as and or not null is in
			like between distinct union all primary key foreign references default case when then else end
			with returning`),
		builtins:    wordSet(`count sum avg min max coalesce integer text varchar boolean timestamp`),
		lineComment: []string{"--"}, blockComment: cblock, quotes: `'"`, foldCase: true,
	}, "sql")

	register(&language{keywords: wordSet(`true false null`), quotes: `"`}, "json")
	register(&language{
		keywords:    wordSet(`true false null yes no on off`),
		lineComment: hash, quotes: `"'`,
	}, "yaml", "yml", "toml", "ini", "conf")
	register(&language{blockComment: cblock, quotes: `"'`}, "css", "scss")
}

// Highlight returns code as HTML, with its tokens in <span class="tok-X">.
func Highlight(code, lang string) string {
	if lang == "bt" || lang == "brashtag" {
		return highlightBrashtag(code)
	}
	l := languages[lang]
	if l == nil {
		return html.EscapeString(code)
	}
	var b strings.Builder
	for i := 0; i < len(code); {
		n, class := l.token(code[i:])
		if class == "" {
			b.WriteString(html.EscapeString(code[i : i+n]))
		} else {
			span(&b, class, code[i:i+n])
		}
		i += n
	}
	return b.String()
}

func span(b *strings.Builder, class, text string) {
	b.WriteString(`<span class="tok-` + class + `">` + html.EscapeString(text) + "</span>")
}

func (l *language) token(s string) (int, string) {
	for _, p := range l.lineComment {
		if strings.HasPrefix(s, p) {
			if end := strings.IndexByte(s, '\n'); end >= 0 {
				return end, "c"
			}
			return len(s), "c"
		}
	}
	for _, p := range l.blockComment {
		if strings.HasPrefix(s, p[0]) {
			if end := strings.Index(s[len(p[0]):], p[1]); end >= 0 {
				return len(p[0]) + end + len(p[1]), "c"
			}
			return len(s), "c"
		}
	}
	if l.triple && (strings.HasPrefix(s, `"""`) || strings.HasPrefix(s, "'''")) {
		if end := strings.Index(s[3:], s[:3]); end >= 0 {
			return end + 6, "s"
		}
		return len(s), "s"
	}
	c := s[0]
	if strings.IndexByte(l.quotes, c) >= 0 {
		return scanString(s, strings.IndexByte(l.rawQuotes, c) >= 0), "s"
	}
	if l.variables && c == '$' && len(s) > 1 {
		if s[1] == '{' {
			if end := strings.IndexByte(s, '}'); end > 0 {
				return end + 1, "v"
			}
		}
		if n := identLen(s[1:]); n > 0 {
			return n + 1, "v"
		}
	}
	if c >= '0' && c <= '9' {
		n := 1
		for n < len(s) && (s[n] == '_' || s[n] == '.' || unicode.IsLetter(rune(s[n])) || unicode.IsDigit(rune(s[n]))) {
			n++
		}
		return n, "n"
	}
	if n := identLen(s); n > 0 {
		w := s[:n]
		if l.foldCase {
			w = strings.ToLower(w)
		}
		switch {
		case l.keywords[w]:
			return n, "k"
		case l.builtins[w]:
			return n, "b"
		case strings.HasPrefix(strings.TrimLeft(s[n:], " "), "("):
			return n, "f"
		}
		return n, ""
	}
	if n := len(s) - len(strings.TrimLeft(s, " \t\n")); n > 0 {
		return n, ""
	}
	_, size := utf8.DecodeRuneInString(s)
	return size, ""
}

func scanString(s string, raw bool) int {
	q := s[0]
	for j := 1; j < len(s); j++ {
		switch {
		case s[j] == '\\' && !raw:
			j++
		case s[j] == q:
			return j + 1
		case s[j] == '\n' && q != '`':
			return j
		}
	}
	return len(s)
}

func identLen(s string) int {
	n := 0
	for n < len(s) {
		r, size := utf8.DecodeRuneInString(s[n:])
		if r == '_' || unicode.IsLetter(r) || (n > 0 && unicode.IsDigit(r)) {
			n += size
		} else {
			break
		}
	}
	return n
}

func highlightBrashtag(code string) string {
	var b strings.Builder
	for i := 0; i < len(code); {
		switch c := code[i]; {
		case c == '#':
			if j := strings.IndexAny(code[i:], "{}`\n"); j > 0 && code[i+j] == '{' {
				span(&b, "k", code[i:i+j+1])
				i += j + 1
				continue
			}
			b.WriteString("#")
			i++
		case c == '}':
			span(&b, "k", "}")
			i++
		case c == '`':
			n := 0
			for i+n < len(code) && code[i+n] == '`' {
				n++
			}
			end := strings.Index(code[i+n:], code[i:i+n])
			if end < 0 {
				span(&b, "s", code[i:])
				i = len(code)
				continue
			}
			span(&b, "s", code[i:i+2*n+end])
			i += 2*n + end
		default:
			j := strings.IndexAny(code[i:], "#}`")
			if j < 0 {
				j = len(code) - i
			}
			b.WriteString(html.EscapeString(code[i : i+j]))
			i += j
		}
	}
	return b.String()
}

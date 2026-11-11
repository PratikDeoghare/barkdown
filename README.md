# barkdown

**DISCLAIMER: This is VIBE coded.**

A static site generator for [brashtag](https://github.com/PratikDeoghare/brashtag), in Go, with no
dependencies outside the standard library. The source is the literate program `barkdown.lit`; the Go files
here are generated from it.

## Installation
```
go install github.com/pratikdeoghare/barkdown@latest
```

## Quick Start

### Create example site
```
mkdir -p /tmp/mysite
cd /tmp/mysite 
barkdown init 
barkdown serve 
```

### Tour 
There is a tutorial at `http://localhost:8080/posts/tour/`. 

## Build

```
go mod tidy
go build -o barkdown .
go test ./...
```

## Use

```
barkdown init [dir]                  create a new site with example content
barkdown new post|page "Title"       start a new post (as a draft) or page
barkdown build [flags]               build the site into public/
barkdown serve [flags]               build, serve and rebuild on change
barkdown render [file.bt]            convert one document to a standalone HTML page
barkdown version
```

A site looks like this:

```
site.bt                    site configuration, in brashtag
posts/2026-11-01-hello.bt  a post; the date prefix becomes its date
posts/trip/index.bt        a page bundle; other files in posts/trip/ are copied next to it
pages/about.bt             a page, published at /about/
static/                    copied to the output unchanged
templates/                 optional overrides for the built-in templates and assets
public/                    the generated site (created by barkdown build)
```

A post starts with its metadata:

```
#title{Hello, barkdown}
#date{2026-11-01}
#tags{go, brashtag}
#summary{What this blog is about.}
```

A post can grow over time, one timestamped thought after another:

```
#thought{2026-11-11 09:30
  The margin is the best part of Tufte's layout.
}
```

and `site.bt` holds the settings:

```
#title{Notes from the barkdown}
#baseurl{https://example.com/blog/}
#author{Ada}
#perpage{5}
#nav{ #link{/ Home} #link{/about/ About} #link{/archive/ Archive} }
#footer{Written in #link{https://github.com/PratikDeoghare/brashtag brashtag}.}
```

Deploy by copying `public/` to any static host. For GitHub Pages under a repository name, build with
`barkdown build -baseurl https://USER.github.io/REPO/`.

## Regenerating the Go files

```
lit tangle barkdown.lit .     # brashtag-literate tools
```

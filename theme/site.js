(function () {
  "use strict";
  var root = document.documentElement;

  var lamp = document.querySelector(".lamp");
  if (lamp) {
    var dimmed = function () {
      var t = root.getAttribute("data-theme");
      return t ? t === "dark" : matchMedia("(prefers-color-scheme: dark)").matches;
    };
    var label = function () { lamp.textContent = dimmed() ? "Turn the lamp up" : "Dim the lamp"; };
    label();
    lamp.addEventListener("click", function () {
      var next = dimmed() ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("theme", next); } catch (e) {}
      label();
    });
  }

  if (navigator.clipboard) {
    document.querySelectorAll(".codeblock").forEach(function (block) {
      var code = block.querySelector("code");
      var head = block.querySelector(".code-head");
      if (!head) {
        head = document.createElement("div");
        head.className = "code-head";
        block.insertBefore(head, block.querySelector("pre"));
      }
      var button = document.createElement("button");
      button.type = "button";
      button.className = "copy";
      button.textContent = "copy";
      button.addEventListener("click", function () {
        navigator.clipboard.writeText(code.innerText).then(function () {
          button.textContent = "copied";
          setTimeout(function () { button.textContent = "copy"; }, 1500);
        });
      });
      head.appendChild(button);
    });
  }

  document.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest('a[href*="#"]');
    if (!a || a.pathname !== location.pathname) return;
    var target = document.getElementById(decodeURIComponent(a.hash.slice(1)));
    if (target) {
      target.classList.remove("flash");
      void target.offsetWidth;
      target.classList.add("flash");
    }
  });

  var contents = document.querySelector(".toc[id]");
  var back = document.querySelector(".goto-toc");
  if (contents && back && window.IntersectionObserver) {
    new IntersectionObserver(function (entries) {
      var e = entries[0];
      back.classList.toggle("shown", !e.isIntersecting && e.boundingClientRect.bottom < 0);
    }).observe(contents);
  } else if (back) {
    back.classList.add("shown");
  }

  document.querySelectorAll(".timeline").forEach(function (timeline) {
    var head = timeline.querySelector(".timeline-head");
    var order = timeline.getAttribute("data-order");
    var button = document.createElement("button");
    button.type = "button";
    button.className = "timeline-order";
    var label = function () { button.textContent = order === "newest" ? "Show oldest first" : "Show newest first"; };
    var flip = function () {
      var thoughts = Array.prototype.slice.call(timeline.children).filter(function (el) { return el.classList.contains("thought"); });
      thoughts.reverse().forEach(function (el) { timeline.appendChild(el); });
      order = order === "newest" ? "oldest" : "newest";
      timeline.setAttribute("data-order", order);
      label();
    };
    var saved = null;
    try { saved = localStorage.getItem("timeline-order"); } catch (e) {}
    if (saved && saved !== order) flip();
    label();
    button.addEventListener("click", function () {
      flip();
      try { localStorage.setItem("timeline-order", order); } catch (e) {}
    });
    if (head) head.appendChild(button);
  });

  var form = document.getElementById("search-form");
  if (form) {
    var input = document.getElementById("search-input");
    var results = document.getElementById("search-results");
    var status = document.getElementById("search-status");
    var docs = [];
    var esc = function (s) {
      return String(s).replace(/[&<>"']/g, function (c) { return "&#" + c.charCodeAt(0) + ";"; });
    };
    var count = function (hay, needle) {
      var n = 0, i = hay.indexOf(needle);
      while (i >= 0 && n < 20) { n++; i = hay.indexOf(needle, i + needle.length); }
      return n;
    };
    var snippet = function (text, term) {
      var i = text.toLowerCase().indexOf(term);
      if (i < 0) return esc(text.slice(0, 180)) + (text.length > 180 ? "…" : "");
      var start = Math.max(0, i - 70);
      var j = i - start;
      var part = text.slice(start, i + term.length + 110);
      return (start > 0 ? "…" : "") + esc(part.slice(0, j)) + "<mark>" + esc(part.slice(j, j + term.length)) +
        "</mark>" + esc(part.slice(j + term.length)) + (i + term.length + 110 < text.length ? "…" : "");
    };
    var run = function () {
      var q = input.value.trim().toLowerCase();
      try { history.replaceState(null, "", q ? "?q=" + encodeURIComponent(q) : location.pathname); } catch (e) {}
      if (!q) { results.innerHTML = ""; status.textContent = ""; return; }
      var terms = q.split(/\s+/);
      var found = [];
      docs.forEach(function (d) {
        var title = d.title.toLowerCase(), text = d.text.toLowerCase(), tags = (d.tags || []).join(" ").toLowerCase();
        var score = 0;
        for (var k = 0; k < terms.length; k++) {
          var s = 10 * count(title, terms[k]) + 5 * count(tags, terms[k]) + count(text, terms[k]);
          if (!s) return;
          score += s;
        }
        found.push({ doc: d, score: score });
      });
      found.sort(function (a, b) { return b.score - a.score; });
      status.textContent = found.length === 0 ? "Nothing here mentions “" + q + "”." :
        found.length === 1 ? "One post matches." : found.length + " posts match.";
      results.innerHTML = found.slice(0, 50).map(function (f) {
        return '<article class="result"><h2><a href="' + esc(f.doc.url) + '">' + esc(f.doc.title) + "</a></h2>" +
          (f.doc.date ? '<p class="meta">' + esc(f.doc.date) + "</p>" : "") +
          "<p>" + snippet(f.doc.text, terms[0]) + "</p></article>";
      }).join("");
    };
    form.addEventListener("submit", function (e) { e.preventDefault(); run(); });
    input.addEventListener("input", run);
    fetch(form.getAttribute("data-index"))
      .then(function (r) { return r.json(); })
      .then(function (index) {
        docs = index;
        var q = new URLSearchParams(location.search).get("q");
        if (q) input.value = q;
        run();
      })
      .catch(function () { status.textContent = "The search index did not load. Reload the page to try again."; });
  }

  var reload = document.body.getAttribute("data-livereload");
  if (reload && window.EventSource) {
    new EventSource(reload).onmessage = function (e) {
      if (e.data === "reload") location.reload();
    };
  }
})();

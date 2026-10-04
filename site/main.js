(function () {
  "use strict";

  // Base layer: works without anime.js. motion.js (loaded after this file)
  // registers hooks on window.SolomonMotion to take over the visual side.
  document.documentElement.classList.add("js");

  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var M = (window.SolomonMotion = window.SolomonMotion || {});

  // Install tabs and copy
  var install = document.querySelector("[data-install]");
  if (install) {
    var tabs = install.querySelectorAll("[data-cmd]");
    var target = install.querySelector("[data-cmd-target]");
    var copyBtn = install.querySelector("[data-copy]");

    tabs.forEach(function (tab) {
      tab.addEventListener("click", function () {
        tabs.forEach(function (t) { t.setAttribute("aria-selected", "false"); });
        tab.setAttribute("aria-selected", "true");
        target.textContent = tab.getAttribute("data-cmd");
        if (M.onTab) M.onTab(target);
      });
    });

    copyBtn.addEventListener("click", function () {
      var text = target.textContent;
      var done = function () {
        copyBtn.classList.add("done");
        if (M.onCopy) M.onCopy(copyBtn);
        setTimeout(function () { copyBtn.classList.remove("done"); }, 1600);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () {});
      } else {
        var area = document.createElement("textarea");
        area.value = text;
        document.body.appendChild(area);
        area.select();
        try { document.execCommand("copy"); done(); } catch (e) {}
        document.body.removeChild(area);
      }
    });
  }

  var scrollInstall = document.querySelector("[data-scroll-install]");
  if (scrollInstall && install) {
    scrollInstall.addEventListener("click", function (e) {
      e.preventDefault();
      install.scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth", block: "center" });
    });
  }

  // Reveal on scroll. The observer decides *when*; if motion.js registered a
  // hook it decides *how*, otherwise the CSS transition in base.css runs.
  var reveals = Array.prototype.slice.call(document.querySelectorAll(".reveal"));
  var headline = document.querySelector("[data-words]");

  function show(els, group) {
    if (M.onReveal && !reduceMotion) {
      M.onReveal(els, group);
    } else {
      els.forEach(function (el, i) { el.style.setProperty("--i", String(i)); el.classList.add("in"); });
    }
  }

  if ("IntersectionObserver" in window && !reduceMotion) {
    var grouped = [];
    var groups = [];
    document.querySelectorAll("[data-stagger]").forEach(function (group) {
      var kids = Array.prototype.filter.call(group.children, function (c) { return c.classList.contains("reveal"); });
      if (!kids.length) return;
      kids.forEach(function (k) { grouped.push(k); });
      groups.push([group, kids]);
    });
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        io.unobserve(entry.target);
        var hit = groups.filter(function (g) { return g[0] === entry.target; })[0];
        show(hit ? hit[1] : [entry.target], hit ? entry.target : null);
      });
    }, { rootMargin: "0px 0px -8% 0px" });
    groups.forEach(function (g) { io.observe(g[0]); });
    reveals.forEach(function (el) {
      if (grouped.indexOf(el) !== -1 || el === headline) return;
      io.observe(el);
    });
    if (headline) headline.classList.add("in");
  } else {
    reveals.forEach(function (el) { el.classList.add("in"); });
  }

  // Active nav link while scrolling
  var navLinks = document.querySelectorAll("[data-nav] a[href^='#']");
  if (navLinks.length && "IntersectionObserver" in window) {
    var byId = {};
    navLinks.forEach(function (a) { byId[a.getAttribute("href").slice(1)] = a; });
    var nio = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        var link = byId[entry.target.id];
        if (!link || !entry.isIntersecting) return;
        navLinks.forEach(function (a) { a.classList.remove("active"); });
        link.classList.add("active");
      });
    }, { rootMargin: "-40% 0px -55% 0px" });
    Object.keys(byId).forEach(function (id) {
      var sec = document.getElementById(id);
      if (sec) nio.observe(sec);
    });
  }

  // Terminal session
  var term = document.querySelector("[data-terminal]");
  if (!term) return;

  var script = [
    ["type", [["t-gold", "$ "], ["", "solomon ."]]],
    ["line", [["t-dim", "Solomon · agent mode · model: local/qwen2.5-coder · ~/code/api"]]],
    ["line", [["", ""]]],
    ["type", [["t-dim", "[#001] "], ["t-gold", "You: "], ["", "add rate limiting to the /login handler and cover it with tests"]]],
    ["line", [["", ""]]],
    ["line", [["t-sky", "  ◆ searchTools   "], ["t-dim", "read, edit, shell"]]],
    ["line", [["t-sky", "  ◆ orchestrate   "], ["t-dim", "read login.go · grep RateLimit · go test ./internal/http"]]],
    ["line", [["t-sky", "  ✎ editFile      "], ["", "internal/http/login.go  "], ["t-add", "+24 "], ["t-del", "-3"]]],
    ["line", [["t-sky", "  ✎ editFile      "], ["", "internal/http/login_test.go  "], ["t-add", "+41"]]],
    ["line", [["t-sky", "  $ shell         "], ["", "go test ./internal/http  "], ["t-ok", "ok  0.412s"]]],
    ["line", [["", ""]]],
    ["stream", [["", "Added a per-IP token bucket (5 requests/minute) in front of the login handler, with tests for the limit, the reset window and the 429 response."]]],
    ["line", [["", ""]]],
    ["type", [["t-dim", "[#002] "], ["t-gold", "You: "], ["", "/goto 001"]]],
    ["line", [["t-sky", "  ↺ "], ["t-dim", "rewound to checkpoint #001 · files restored"]]],
    ["line", [["", ""]]],
    ["prompt", [["t-dim", "[#001b] "], ["t-gold", "You: "]]]
  ];

  function span(cls, text) {
    var s = document.createElement("span");
    if (cls) s.className = cls;
    s.textContent = text;
    return s;
  }

  var cursor = document.createElement("span");
  cursor.className = "cursor";

  function renderStatic() {
    term.textContent = "";
    script.forEach(function (entry) {
      entry[1].forEach(function (part) { term.appendChild(span(part[0], part[1])); });
      if (entry[0] !== "prompt") term.appendChild(document.createTextNode("\n"));
    });
    term.appendChild(cursor);
  }

  if (reduceMotion || !("IntersectionObserver" in window)) {
    renderStatic();
    return;
  }

  function wait(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }

  function typeInto(el, text, speed) {
    var i = 0;
    return new Promise(function (resolve) {
      (function step() {
        if (i >= text.length) return resolve();
        var chunk = speed < 10 ? 3 : 1;
        el.textContent += text.slice(i, i + chunk);
        i += chunk;
        setTimeout(step, speed + Math.random() * speed);
      })();
    });
  }

  function play() {
    term.textContent = "";
    term.appendChild(cursor);
    var chain = Promise.resolve();
    script.forEach(function (entry) {
      var kind = entry[0];
      var parts = entry[1];
      chain = chain.then(function () {
        var seq = Promise.resolve();
        parts.forEach(function (part, idx) {
          seq = seq.then(function () {
            var el = span(part[0], "");
            term.insertBefore(el, cursor);
            var isInput = kind === "type" && idx === parts.length - 1;
            if (isInput) return wait(350).then(function () { return typeInto(el, part[1], 38); });
            if (kind === "stream") return typeInto(el, part[1], 6);
            el.textContent = part[1];
            if (kind === "line" && part[1] && M.onTermLine) M.onTermLine(el);
          });
        });
        return seq.then(function () {
          if (kind !== "prompt") term.insertBefore(document.createTextNode("\n"), cursor);
          return wait(kind === "type" ? 500 : kind === "line" ? 160 : 300);
        });
      });
    });
  }

  var started = false;
  var tio = new IntersectionObserver(function (entries) {
    if (!started && entries[0].isIntersecting) {
      started = true;
      tio.disconnect();
      play();
    }
  }, { threshold: 0.3 });
  tio.observe(term);
})();

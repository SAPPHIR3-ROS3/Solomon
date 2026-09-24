(function () {
  "use strict";

  document.documentElement.classList.add("js");

  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

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
      });
    });

    copyBtn.addEventListener("click", function () {
      var text = target.textContent;
      var done = function () {
        copyBtn.classList.add("done");
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

  // Reveal on scroll
  var reveals = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window && !reduceMotion) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add("in");
          io.unobserve(entry.target);
        }
      });
    }, { rootMargin: "0px 0px -8% 0px" });
    reveals.forEach(function (el) { io.observe(el); });
  } else {
    reveals.forEach(function (el) { el.classList.add("in"); });
  }

  // Terminal session
  var term = document.querySelector("[data-terminal]");
  if (!term) return;

  // Each line: [kind, text]. "type" lines are typed out, the rest appear at once.
  var script = [
    ["type", [["t-gold", "$ "], ["", "solomon ."]]],
    ["line", [["t-dim", "Solomon · agent mode · provider: Claude Sub · ~/code/api"]]],
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

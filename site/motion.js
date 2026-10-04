(function () {
  "use strict";

  // Motion layer on anime.js v4 (global "anime" from the IIFE build). If the
  // library is missing or the user prefers reduced motion, main.js keeps the
  // page fully usable with its CSS fallbacks.
  var A = window.anime;
  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (!A || typeof A.animate !== "function" || reduceMotion) return;

  var animate = A.animate, stagger = A.stagger, utils = A.utils, createTimeline = A.createTimeline;
  var finePointer = window.matchMedia("(hover: hover) and (pointer: fine)").matches;
  var EASE = "cubicBezier(0.2, 0.7, 0.2, 1)";
  var M = (window.SolomonMotion = window.SolomonMotion || {});
  document.documentElement.classList.add("anime");

  /* ---------- helpers ---------- */

  // Split the text of an element into .ch spans (one per character), keeping
  // <br>, <em> and any other child element intact as a unit.
  function splitChars(el) {
    var chars = [];
    Array.prototype.slice.call(el.childNodes).forEach(function (node) {
      if (node.nodeType === 3) {
        var frag = document.createDocumentFragment();
        node.textContent.split("").forEach(function (c) {
          if (c === "\n") return;
          var s = document.createElement("span");
          s.className = "ch" + (c === " " ? " space" : "");
          s.textContent = c === " " ? "\u00a0" : c;
          chars.push(s);
          frag.appendChild(s);
        });
        el.replaceChild(frag, node);
      } else if (node.nodeName === "EM" && !node.hasAttribute("data-rotate")) {
        chars = chars.concat(splitChars(node));
      } else if (node.nodeType === 1 && node.nodeName !== "BR" && node.nodeName !== "SVG") {
        node.classList.add("wd");
        chars.push(node);
      }
    });
    return chars;
  }

  function splitWords(el) {
    var words = [];
    Array.prototype.slice.call(el.childNodes).forEach(function (node) {
      if (node.nodeType === 3) {
        var frag = document.createDocumentFragment();
        node.textContent.split(/(\s+)/).forEach(function (piece) {
          if (!piece) return;
          if (/^\s+$/.test(piece)) { frag.appendChild(document.createTextNode(piece)); return; }
          var w = document.createElement("span");
          w.className = "wd";
          w.textContent = piece;
          words.push(w);
          frag.appendChild(w);
        });
        el.replaceChild(frag, node);
      } else if (node.nodeType === 1 && node.nodeName !== "BR") {
        node.classList.add("wd");
        words.push(node);
      }
    });
    return words;
  }

  /* ---------- hero: headline + rotating word ---------- */

  var headline = document.querySelector("[data-words]");
  if (headline) {
    var words = splitWords(headline);
    animate(words, {
      opacity: [0, 1],
      y: ["0.5em", "0em"],
      rotate: [2, 0],
      filter: ["blur(8px)", "blur(0px)"],
      duration: 1000,
      delay: stagger(90, { start: 150 }),
      ease: EASE
    });

    var rotator = headline.querySelector("[data-rotate]");
    if (rotator) {
      var phrases = rotator.getAttribute("data-rotate").split("|");
      var idx = 0;
      function setPhrase(text) {
        rotator.textContent = text;
        return splitChars(rotator);
      }
      function cycle() {
        var out = Array.prototype.slice.call(rotator.querySelectorAll(".ch"));
        animate(out, {
          y: ["0em", "-0.7em"],
          opacity: [1, 0],
          duration: 420,
          delay: stagger(22),
          ease: "inQuad",
          onComplete: function () {
            idx = (idx + 1) % phrases.length;
            var inChars = setPhrase(phrases[idx]);
            animate(inChars, {
              y: ["0.7em", "0em"],
              opacity: [0, 1],
              duration: 560,
              delay: stagger(26),
              ease: "outExpo"
            });
          }
        });
      }
      setTimeout(function () {
        setPhrase(phrases[0]);
        setInterval(cycle, 3000);
      }, 2600);
    }
  }

  /* ---------- hero: dot grid ---------- */

  // Quiet dots at rest. Near the pointer they swell a little and link to their
  // neighbours, so a patch of grid appears under the cursor and fades behind it.
  var field = document.querySelector("[data-dotfield]");
  if (field) {
    var canvas = document.createElement("canvas");
    field.appendChild(canvas);
    var ctx = canvas.getContext("2d");
    var CELL = 34, REACH = 150;
    var cols = 0, rows = 0, ox = 0, oy = 0, energy = null, W = 0, H = 0, dpr = 1;
    var pointer = { x: -9999, y: -9999 };
    var presence = { v: 0 };
    var intro = { v: 0 };
    var running = false;

    function resize() {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      W = field.clientWidth; H = field.clientHeight;
      canvas.width = W * dpr; canvas.height = H * dpr;
      canvas.style.width = W + "px"; canvas.style.height = H + "px";
      cols = Math.ceil(W / CELL) + 1; rows = Math.ceil(H / CELL) + 1;
      ox = (W - (cols - 1) * CELL) / 2; oy = (H - (rows - 1) * CELL) / 2;
      energy = new Float32Array(cols * rows);
      draw();
    }

    function draw() {
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, W, H);
      var active = false;
      for (var i = 0; i < energy.length; i++) {
        var x = ox + (i % cols) * CELL, y = oy + Math.floor(i / cols) * CELL;
        var d = Math.hypot(x - pointer.x, y - pointer.y);
        var target = d < REACH ? Math.pow(1 - d / REACH, 1.6) * presence.v : 0;
        energy[i] += (target - energy[i]) * 0.14;
        if (energy[i] < 0.003) energy[i] = 0; else active = true;
      }
      ctx.lineWidth = 1;
      for (var j = 0; j < energy.length; j++) {
        var e = energy[j];
        if (!e) continue;
        var cx = j % cols, x1 = ox + cx * CELL, y1 = oy + Math.floor(j / cols) * CELL;
        if (cx < cols - 1 && energy[j + 1]) line(x1, y1, x1 + CELL, y1, Math.min(e, energy[j + 1]));
        if (j + cols < energy.length && energy[j + cols]) line(x1, y1, x1, y1 + CELL, Math.min(e, energy[j + cols]));
      }
      for (var k = 0; k < energy.length; k++) {
        var en = energy[k];
        var px = ox + (k % cols) * CELL, py = oy + Math.floor(k / cols) * CELL;
        ctx.fillStyle = en > 0.05
          ? "rgba(255, 199, 4, " + (0.25 + en * 0.6).toFixed(3) + ")"
          : "rgba(134, 201, 242, " + (0.2 * intro.v).toFixed(3) + ")";
        ctx.beginPath();
        ctx.arc(px, py, 1.3 + en * 1.4, 0, Math.PI * 2);
        ctx.fill();
      }
      return active;
    }

    function line(x1, y1, x2, y2, a) {
      ctx.strokeStyle = "rgba(134, 201, 242, " + (a * 0.55).toFixed(3) + ")";
      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.lineTo(x2, y2);
      ctx.stroke();
    }

    function loop() {
      var active = draw();
      if (active || presence.v > 0) requestAnimationFrame(loop);
      else running = false;
    }
    function wake() { if (!running) { running = true; requestAnimationFrame(loop); } }

    resize();
    window.addEventListener("resize", resize);
    animate(intro, { v: 1, duration: 1600, delay: 300, ease: "outQuad", onUpdate: draw });

    if (finePointer) {
      var hero = field.parentNode;
      hero.addEventListener("pointermove", function (e) {
        var r = field.getBoundingClientRect();
        pointer.x = e.clientX - r.left;
        pointer.y = e.clientY - r.top;
        wake();
      });
      hero.addEventListener("pointerenter", function () { animate(presence, { v: 1, duration: 500, ease: "outQuad" }); wake(); });
      hero.addEventListener("pointerleave", function () { animate(presence, { v: 0, duration: 700, ease: "inOutQuad" }); wake(); });
    }
  }

  /* ---------- hero: parallax ---------- */

  var heroInner = document.querySelector(".hero-inner");
  var terminal = document.querySelector(".terminal");
  if (heroInner) {
    var ticking = false;
    window.addEventListener("scroll", function () {
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(function () {
        ticking = false;
        var y = window.scrollY;
        if (y > 1200) return;
        heroInner.style.transform = "translate3d(0, " + (y * 0.18).toFixed(1) + "px, 0)";
        heroInner.style.opacity = String(Math.max(0, 1 - y / 700));
        if (terminal) terminal.style.setProperty("--lift", (y * -0.06).toFixed(1) + "px");
      });
    }, { passive: true });
  }

  /* ---------- reveals ---------- */

  // Pre-split section headings so their characters can cascade in.
  var headings = {};
  document.querySelectorAll(".section-head h2, .closing h2").forEach(function (h2, i) {
    var chars = splitChars(h2);
    chars.forEach(function (c) { c.style.opacity = "0"; });
    headings[i] = chars;
    h2.setAttribute("data-h", String(i));
  });

  M.onReveal = function (els, group) {
    els.forEach(function (el) { el.classList.add("in"); });

    animate(els, {
      opacity: [0, 1],
      y: [34, 0],
      duration: 1000,
      delay: group ? stagger(110) : 0,
      ease: EASE
    });

    els.forEach(function (el) {
      var h2 = el.matches("h2[data-h]") ? el : el.querySelector("h2[data-h]");
      if (h2) {
        animate(headings[h2.getAttribute("data-h")], {
          opacity: [0, 1],
          y: ["0.6em", "0em"],
          rotate: [6, 0],
          duration: 900,
          delay: stagger(14, { start: 100 }),
          ease: "outExpo"
        });
      }
      var kicker = el.querySelector(".kicker");
      if (kicker) animate(kicker, { letterSpacing: ["0.5em", "0.1em"], opacity: [0, 1], duration: 1200, ease: "outExpo" });

      var index = el.querySelector(".pillar-index");
      if (index) {
        var n = parseInt(index.textContent, 10);
        var o = { v: 0 };
        animate(o, { v: n, duration: 1200, delay: 200, ease: "outExpo", modifier: utils.round(0), onUpdate: function () { index.textContent = (o.v < 10 ? "0" : "") + o.v; } });
      }

      var tag = el.querySelector(".feature-tag");
      if (tag) animate(tag, { x: [-16, 0], opacity: [0, 1], duration: 800, delay: 150, ease: "outExpo" });

      if (el.classList.contains("feature")) {
        var h3 = el.querySelector("h3");
        if (h3) animate(h3, { y: [20, 0], opacity: [0, 1], duration: 900, delay: 220, ease: EASE });
      }

      if (el.hasAttribute("data-stats")) startStats(el);
      if (el.hasAttribute("data-turns")) startTurns(el);
      if (el.hasAttribute("data-orbit")) startOrbit(el);
      var scribble = el.querySelector(".scribble");
      if (scribble) animate(scribble, { strokeDashoffset: [1, 0], duration: 1300, delay: 700, ease: "inOutCubic" });
    });

    if (group && group.classList.contains("ledger")) {
      animate(group.querySelectorAll("dt"), { x: [-18, 0], duration: 900, delay: stagger(110, { start: 120 }), ease: "outExpo" });
    }
  };

  /* ---------- authorship stats ---------- */

  function startStats(root) {
    root.querySelectorAll("[data-count]").forEach(function (el, i) {
      var to = parseInt(el.getAttribute("data-count"), 10);
      var from = parseInt(el.getAttribute("data-from") || "0", 10);
      var o = { v: from };
      el.textContent = String(from);
      animate(o, {
        v: to,
        duration: from > 100 ? 2600 : 1800,
        delay: 200 + i * 150,
        ease: "outExpo",
        modifier: utils.round(0),
        onUpdate: function () { el.textContent = String(o.v); }
      });
    });
  }

  /* ---------- code mode: round trips ---------- */

  function startTurns(root) {
    var laneA = root.querySelector(".lane:not(.lane-solomon)");
    var laneB = root.querySelector(".lane-solomon");
    var ballA = laneA.querySelector(".ball"), ballB = laneB.querySelector(".ball");
    var countA = root.querySelector("[data-count-a]"), countB = root.querySelector("[data-count-b]");

    function distance(lane) {
      var line = lane.querySelector(".lane-line");
      var ends = lane.querySelectorAll(".lane-end");
      var track = lane.querySelector(".lane-track");
      var from = ends[0].offsetWidth + 10;
      lane.querySelector(".ball").style.setProperty("--from", from - 5 + "px");
      return track.offsetWidth - from - ends[1].offsetWidth - 10;
    }

    function run() {
      var dA = distance(laneA), dB = distance(laneB);
      var trips = 6;
      countA.textContent = "0 turns";
      countB.textContent = "0 turns";
      var tl = createTimeline({ defaults: { ease: "inOutQuad" }, onComplete: function () { setTimeout(run, 1400); } });
      for (var i = 0; i < trips; i++) {
        tl.add(ballA, { x: dA, duration: 420 }, i * 900)
          .add(ballA, { x: 0, duration: 420, onComplete: (function (n) { return function () { countA.textContent = n + (n === 1 ? " turn" : " turns"); }; })(i + 1) }, i * 900 + 460);
      }
      tl.add(ballB, { x: dB, duration: 520, ease: "outExpo" }, 0)
        .add(ballB, { scale: [1, 1.5, 1, 1.5, 1, 1.5, 1], rotate: 360, duration: 1500, ease: "inOutSine" }, 560)
        .add(ballB, { x: 0, duration: 520, ease: "inOutExpo", onComplete: function () { countB.textContent = "1 turn"; } }, 2100)
        .add(countB, { scale: [1, 1.25, 1], duration: 500 }, 2600);
    }
    run();
  }

  /* ---------- surfaces: orbit ---------- */

  function startOrbit(root) {
    var nodes = Array.prototype.slice.call(root.querySelectorAll(".orbit-node"));
    var packets = root.querySelector(".orbit-packets");
    var state = { a: 0 };

    function radius() { return root.clientWidth / 2; }
    function place() {
      var R = radius();
      nodes.forEach(function (n) {
        var ang = (parseFloat(n.getAttribute("data-angle")) + state.a) * Math.PI / 180;
        n.style.transform = "translate(-50%, -50%) translate(" + (Math.cos(ang) * R).toFixed(1) + "px, " + (Math.sin(ang) * R).toFixed(1) + "px)";
      });
    }
    place();
    animate(state, { a: 360, duration: 90000, ease: "linear", loop: true, onUpdate: place });
    animate(root.querySelector(".orbit-core"), { scale: [1, 1.05], duration: 2200, alternate: true, loop: true, ease: "inOutSine" });

    animate(nodes, { opacity: [0, 1], scale: [0.6, 1], duration: 900, delay: stagger(120, { start: 300 }), ease: "outBack" });

    // packets travel from the core to a node (or back) every so often
    function packet() {
      var n = nodes[Math.floor(Math.random() * nodes.length)];
      if (n.classList.contains("soon") && Math.random() < 0.7) return setTimeout(packet, 200);
      var p = document.createElement("i");
      p.className = "packet";
      packets.appendChild(p);
      var out = Math.random() > 0.35;
      var o = { t: out ? 0 : 1 };
      var R = radius() - 28;
      animate(o, {
        t: out ? 1 : 0,
        duration: 900 + Math.random() * 500,
        ease: out ? "inQuad" : "outQuad",
        onUpdate: function () {
          var ang = (parseFloat(n.getAttribute("data-angle")) + state.a) * Math.PI / 180;
          p.style.transform = "translate(" + (Math.cos(ang) * R * o.t).toFixed(1) + "px, " + (Math.sin(ang) * R * o.t).toFixed(1) + "px)";
          p.style.opacity = String(Math.sin(o.t * Math.PI));
        },
        onComplete: function () {
          packets.removeChild(p);
          if (out) animate(n, { scale: [1, 1.12, 1], duration: 500, ease: "outQuad" });
        }
      });
      setTimeout(packet, 500 + Math.random() * 900);
    }
    setTimeout(packet, 1200);
  }

  /* ---------- marquee ---------- */

  var track = document.querySelector("[data-marquee] .marquee-track");
  if (track) {
    Array.prototype.map.call(track.children, function (li) { return li.cloneNode(true); })
      .forEach(function (li) { li.setAttribute("aria-hidden", "true"); track.appendChild(li); });
    track.style.animation = "none";
    var loop = animate(track, { x: ["0%", "-50%"], duration: 38000, ease: "linear", loop: true });
    track.parentNode.addEventListener("pointerenter", function () { loop.pause(); });
    track.parentNode.addEventListener("pointerleave", function () { loop.play(); });
  }

  /* ---------- pointer effects ---------- */

  if (finePointer) {
    document.querySelectorAll("[data-spot]").forEach(function (card) {
      card.addEventListener("pointermove", function (e) {
        var r = card.getBoundingClientRect();
        card.style.setProperty("--mx", (e.clientX - r.left) + "px");
        card.style.setProperty("--my", (e.clientY - r.top) + "px");
      });
    });

    document.querySelectorAll("[data-tilt]").forEach(function (card) {
      card.addEventListener("pointermove", function (e) {
        var r = card.getBoundingClientRect();
        var x = (e.clientX - r.left) / r.width - 0.5;
        var y = (e.clientY - r.top) / r.height - 0.5;
        animate(card, { rotateX: -y * 4, rotateY: x * 4, perspective: 1200, duration: 180, ease: "outQuad" });
      });
      card.addEventListener("pointerleave", function () {
        animate(card, { rotateX: 0, rotateY: 0, perspective: 1200, duration: 800, ease: EASE });
      });
    });

    document.querySelectorAll("[data-magnet]").forEach(function (btn) {
      btn.addEventListener("pointermove", function (e) {
        var r = btn.getBoundingClientRect();
        animate(btn, { x: (e.clientX - r.left - r.width / 2) * 0.2, y: (e.clientY - r.top - r.height / 2) * 0.3, duration: 200, ease: "outQuad" });
      });
      btn.addEventListener("pointerleave", function () {
        animate(btn, { x: 0, y: 0, duration: 700, ease: "outElastic(1, .55)" });
      });
    });

    document.querySelectorAll(".surface, .pillar, .stats div").forEach(function (card) {
      card.addEventListener("pointerenter", function () { animate(card, { y: -5, duration: 350, ease: EASE }); });
      card.addEventListener("pointerleave", function () { animate(card, { y: 0, duration: 500, ease: EASE }); });
    });
  }

  /* ---------- small feedback hooks used by main.js ---------- */

  M.onTab = function (target) { animate(target, { opacity: [0.2, 1], y: [4, 0], duration: 320, ease: "outQuad" }); };
  M.onCopy = function (btn) { animate(btn, { scale: [1, 1.2, 1], duration: 380, ease: "outBack" }); };
  M.onTermLine = function (el) {
    el.style.display = "inline-block";
    animate(el, { opacity: [0, 1], x: [-8, 0], duration: 280, ease: "outQuad" });
  };
})();

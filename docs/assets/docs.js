// RCP For All Camera documentation: shared top bar, sidebar, search and
// light/dark switch. Every page includes this script and declares
// <aside class="sidebar" id="sidebar"></aside>; the menu is built here so
// all pages share one list.
(function () {
  "use strict";

  var icons = {
    home: '<path d="M5 12l-2 0l9 -9l9 9l-2 0"/><path d="M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-7"/>',
    camera: '<path d="M5 7h1a2 2 0 0 0 2 -2a1 1 0 0 1 1 -1h6a1 1 0 0 1 1 1a2 2 0 0 0 2 2h1a2 2 0 0 1 2 2v9a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-9a2 2 0 0 1 2 -2"/><path d="M9 13a3 3 0 1 0 6 0a3 3 0 0 0 -6 0"/>',
    puzzle: '<path d="M4 7h3a1 1 0 0 0 1 -1v-1a2 2 0 0 1 4 0v1a1 1 0 0 0 1 1h3a1 1 0 0 1 1 1v3a1 1 0 0 0 1 1h1a2 2 0 0 1 0 4h-1a1 1 0 0 0 -1 1v3a1 1 0 0 1 -1 1h-3a1 1 0 0 1 -1 -1v-1a2 2 0 0 0 -4 0v1a1 1 0 0 1 -1 1h-3a1 1 0 0 1 -1 -1v-3a1 1 0 0 1 1 -1h1a2 2 0 0 0 0 -4h-1a1 1 0 0 1 -1 -1v-3a1 1 0 0 1 1 -1"/>',
    shield: '<path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"/>',
    layers: '<path d="M12 4l-8 4l8 4l8 -4l-8 -4"/><path d="M4 12l8 4l8 -4"/><path d="M4 16l8 4l8 -4"/>',
    check: '<path d="M5 12l5 5l10 -10"/>',
    book: '<path d="M3 19a9 9 0 0 1 9 0a9 9 0 0 1 9 0"/><path d="M3 6a9 9 0 0 1 9 0a9 9 0 0 1 9 0"/><path d="M3 6l0 13"/><path d="M12 6l0 13"/><path d="M21 6l0 13"/>',
    panel: '<path d="M3 5a1 1 0 0 1 1 -1h16a1 1 0 0 1 1 1v14a1 1 0 0 1 -1 1h-16a1 1 0 0 1 -1 -1z"/><path d="M7 9h2"/><path d="M7 13h2"/><path d="M13 9h4"/><path d="M13 13h4"/>',
    code: '<path d="M7 8l-4 4l4 4"/><path d="M17 8l4 4l-4 4"/><path d="M14 4l-4 16"/>',
    sun: '<path d="M12 12m-4 0a4 4 0 1 0 8 0a4 4 0 1 0 -8 0"/><path d="M3 12h1m8 -9v1m8 8h1m-9 8v1m-6.4 -15.4l.7 .7m12.1 -.7l-.7 .7m0 11.4l.7 .7m-12.1 -.7l-.7 .7"/>',
    moon: '<path d="M12 3c.132 0 .263 0 .393 0a7.5 7.5 0 0 0 7.92 12.446a9 9 0 1 1 -8.313 -12.454z"/>',
    menu: '<path d="M4 6l16 0"/><path d="M4 12l16 0"/><path d="M4 18l16 0"/>',
    search: '<path d="M10 10m-7 0a7 7 0 1 0 14 0a7 7 0 1 0 -14 0"/><path d="M21 21l-6 -6"/>'
  };

  function svg(name) {
    return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + icons[name] + "</svg>";
  }

  // The menu: groups, then pages with their sections.
  var nav = [
    { group: "Get started", pages: [
      { href: "index.html", icon: "home", title: "Overview", subs: [
        ["#what", "What the application does"], ["#quickstart", "Quick start"], ["#concepts", "Key concepts"],
        ["#architecture", "Architecture"], ["#flow", "From the panel to the camera"], ["#panel", "The control panel"],
        ["#window", "The application window"], ["#data", "Data and security"], ["#build", "Build from source"],
        ["#tests", "Tests and quality"]
      ] }
    ] },
    { group: "Cameras", pages: [
      { href: "modules-camera.html", icon: "puzzle", title: "Camera modules", subs: [
        ["#how", "How modules work"], ["#concepts", "Key concepts"], ["#quickstart", "Quick start"],
        ["#contract", "The module contract"], ["#lifecycle", "Camera lifecycle"], ["#demo", "Walkthrough: demo"],
        ["#controls", "Controls reference"], ["#ui", "The UI file"], ["#blocks", "Block gallery"], ["#widgets", "Widget gallery"],
        ["#host", "Host services"], ["#testing", "Testing"], ["#sandbox", "The sandbox"], ["#abi", "Appendix: the ABI"],
        ["#faq", "FAQ"]
      ] }
    ] },
    { group: "Security", pages: [
      { href: "vulnerabilities.html", icon: "shield", title: "Known vulnerabilities", subs: [
        ["#how", "How we check"], ["#dependencies", "Dependencies"], ["#threats", "Application threats"], ["#report", "Report a vulnerability"]
      ] }
    ] },
    { group: "Reference", pages: [
      { href: "go-engineering-rules.md", icon: "book", title: "Go engineering rules", subs: [] },
      { href: "atem-micro-camera-panel-protocol.md", icon: "panel", title: "Panel USB protocol", subs: [] }
    ] }
  ];

  var here = location.pathname.split("/").pop() || "index.html";

  function buildSidebar(el) {
    var html = "";
    nav.forEach(function (g) {
      html += '<div class="group">' + g.group + "</div>";
      g.pages.forEach(function (p) {
        var cur = p.href === here;
        html += '<a class="item' + (cur ? " active" : "") + '" href="' + p.href + '">' + svg(p.icon) + p.title + "</a>";
        p.subs.forEach(function (s) {
          html += '<a class="sub" href="' + (cur ? "" : p.href) + s[0] + '">' + s[1] + "</a>";
        });
      });
    });
    html += '<div class="nohit">No match in the menu.</div>';
    el.innerHTML = html;
  }

  function readTheme() {
    try { return localStorage.getItem("rcp-docs-theme"); } catch (e) { return null; }
  }
  function saveTheme(t) {
    try { localStorage.setItem("rcp-docs-theme", t); } catch (e) { /* private mode */ }
  }
  function effectiveTheme() {
    var t = document.documentElement.getAttribute("data-theme");
    if (t) return t;
    return window.matchMedia && matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  var saved = readTheme();
  if (saved === "light" || saved === "dark") document.documentElement.setAttribute("data-theme", saved);

  document.addEventListener("DOMContentLoaded", function () {
    var bar = document.getElementById("topbar");
    if (bar) {
      bar.innerHTML =
        '<button class="menu-btn" id="menu-btn" aria-label="Menu">' + svg("menu") + "</button>" +
        '<a class="brand" href="index.html"><svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14" fill="#212225"/>' +
        '<circle cx="32" cy="32" r="26" fill="none" stroke="#6B6F78" stroke-width="2"/>' +
        '<path d="M36.993 25.409L51.939 25.409A21 21 0 0 1 51.465 39.881Z M40.191 30.87L50.759 41.438A21 21 0 0 1 40.191 51.337Z M38.591 36.993L38.591 51.939A21 21 0 0 1 24.119 51.465Z M33.13 40.191L22.562 50.759A21 21 0 0 1 12.663 40.191Z M27.007 38.591L12.061 38.591A21 21 0 0 1 12.535 24.119Z M23.809 33.13L13.241 22.562A21 21 0 0 1 23.809 12.663Z M30.87 23.809L41.438 13.241A21 21 0 0 1 51.337 23.809Z" fill="#E8E9EC"/>' +
        '<path d="M25.409 27.007L25.409 12.061A21 21 0 0 1 39.881 12.535Z" fill="#F47B20"/></svg>RCP <span>docs</span></a>' +
        '<div class="search">' + svg("search") + '<input id="search" type="search" placeholder="Search" aria-label="Search the documentation"></div>' +
        '<nav class="toplinks"><a href="index.html">Docs</a><a href="modules-camera.html">Modules</a><a href="vulnerabilities.html">Security</a></nav>' +
        '<button class="theme-btn" id="theme-btn" aria-label="Light or dark theme"></button>';
    }
    var side = document.getElementById("sidebar");
    if (side) buildSidebar(side);

    var themeBtn = document.getElementById("theme-btn");
    function paintThemeBtn() { if (themeBtn) themeBtn.innerHTML = svg(effectiveTheme() === "dark" ? "sun" : "moon"); }
    paintThemeBtn();
    if (themeBtn) themeBtn.addEventListener("click", function () {
      var next = effectiveTheme() === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      saveTheme(next);
      paintThemeBtn();
    });

    var menuBtn = document.getElementById("menu-btn");
    if (menuBtn) menuBtn.addEventListener("click", function () { document.body.classList.toggle("menu-open"); });
    if (side) side.addEventListener("click", function (e) { if (e.target.closest("a")) document.body.classList.remove("menu-open"); });

    // Search: filters the menu as you type; Enter jumps to the first heading
    // or paragraph of this page that contains the text.
    var input = document.getElementById("search");
    if (input && side) {
      input.addEventListener("input", function () {
        var q = input.value.trim().toLowerCase();
        var any = false;
        side.querySelectorAll("a").forEach(function (a) {
          var hit = !q || a.textContent.toLowerCase().indexOf(q) >= 0;
          a.classList.toggle("hidden", !hit);
          any = any || hit;
        });
        side.querySelector(".nohit").style.display = any ? "none" : "block";
      });
      input.addEventListener("keydown", function (e) {
        if (e.key !== "Enter") return;
        var q = input.value.trim().toLowerCase();
        if (!q) return;
        document.querySelectorAll("mark.hit").forEach(function (m) { m.replaceWith(document.createTextNode(m.textContent)); });
        var nodes = document.querySelectorAll("main h2, main h3, main p, main li, main td, main dt");
        for (var i = 0; i < nodes.length; i++) {
          if (nodes[i].textContent.toLowerCase().indexOf(q) >= 0) {
            var m = document.createElement("mark");
            m.className = "hit";
            nodes[i].parentNode.insertBefore(m, nodes[i]);
            m.appendChild(nodes[i]);
            m.scrollIntoView({ block: "center" });
            return;
          }
        }
      });
    }

    // Highlight the section being read in the menu.
    if (side && "IntersectionObserver" in window) {
      var subs = side.querySelectorAll('a.sub[href^="#"]');
      var byId = {};
      subs.forEach(function (a) { byId[a.getAttribute("href").slice(1)] = a; });
      var obs = new IntersectionObserver(function (entries) {
        entries.forEach(function (en) {
          if (en.isIntersecting && byId[en.target.id]) {
            subs.forEach(function (a) { a.classList.remove("active"); });
            byId[en.target.id].classList.add("active");
          }
        });
      }, { rootMargin: "-80px 0px -70% 0px" });
      Object.keys(byId).forEach(function (id) { var s = document.getElementById(id); if (s) obs.observe(s); });
    }
  });
})();

// Renders the Markdown doc named in <body data-doc="..."> with a shared sidebar.
// The .md files are the single source of truth; the .html pages are thin shells.
(function () {
  var NAV = [
    { file: "README.md", title: "Overview", html: "index.html" },
    { file: "architecture.md", title: "Architecture", html: "architecture.html" },
    { file: "features.md", title: "Features", html: "features.html" },
    { file: "admin-console.md", title: "Admin Console (UI)", html: "admin-console.html" },
    { file: "security.md", title: "Security Model", html: "security.html" },
    { file: "observability.md", title: "Observability", html: "observability.html" },
    { file: "solution-comparison.md", title: "Solution Comparison", html: "solution-comparison.html" },
    { file: "data-model.md", title: "Data Model", html: "data-model.html" },
    { file: "local-dev.md", title: "Local Dev & Runbook", html: "local-dev.html" },
    { file: "local-sandbox.md", title: "Local gVisor Sandbox", html: "local-sandbox.html" },
    { file: "sandbox-isolation.md", title: "Sandboxing & Isolation", html: "sandbox-isolation.html" },
    { file: "multi-tenant-keycloak.md", title: "Multi-tenant (Keycloak)", html: "multi-tenant-keycloak.html" },
    { file: "tenant-provisioning.md", title: "Tenant Provisioning", html: "tenant-provisioning.html" },
    { file: "mcp-inspector-rbac.md", title: "MCP Clients & RBAC", html: "mcp-inspector-rbac.html" },
    { file: "isolation-proof.md", title: "Isolation Proof", html: "isolation-proof.html" }
  ];
  var current = document.body.dataset.doc || "README.md";

  function htmlFor(mdHref) {
    var h = mdHref.replace(/\.md($|[#?])/, ".html$1");
    if (h === "README.html" || h.indexOf("README.html") === 0) h = h.replace("README.html", "index.html");
    return h;
  }

  var nav = document.getElementById("nav");
  nav.innerHTML =
    '<div class="brand">MCP Runtime<br><span>documentation</span></div><ul>' +
    NAV.map(function (n) {
      return '<li><a href="' + n.html + '"' + (n.file === current ? ' class="active"' : "") + ">" + n.title + "</a></li>";
    }).join("") +
    '</ul><div class="foot">Multi-Tenant MCP Gateway Runtime</div>';

  var content = document.getElementById("content");

  // Lazy-load Mermaid (only when a doc has ```mermaid blocks) and render them as
  // diagrams. marked renders fenced ```mermaid as <pre><code class=language-mermaid>.
  function loadMermaid(cb) {
    if (window.mermaid) return cb(window.mermaid);
    var s = document.createElement("script");
    s.src = "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js";
    s.onload = function () { cb(window.mermaid); };
    document.head.appendChild(s);
  }
  function renderMermaid() {
    var blocks = content.querySelectorAll("pre > code.language-mermaid");
    if (!blocks.length) return;
    blocks.forEach(function (code) {
      var div = document.createElement("div");
      div.className = "mermaid";
      div.textContent = code.textContent;
      code.parentNode.replaceWith(div);
    });
    var dark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
    loadMermaid(function (m) {
      m.initialize({ startOnLoad: false, theme: dark ? "dark" : "neutral", securityLevel: "loose" });
      m.run({ querySelector: ".mermaid" });
    });
  }

  fetch(current)
    .then(function (r) {
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.text();
    })
    .then(function (md) {
      content.innerHTML = marked.parse(md);
      // Rewrite intra-doc .md links to their .html shells.
      content.querySelectorAll('a[href$=".md"], a[href*=".md#"]').forEach(function (a) {
        a.setAttribute("href", htmlFor(a.getAttribute("href")));
      });
      renderMermaid();
      var meta = NAV.filter(function (n) { return n.file === current; })[0];
      document.title = (meta ? meta.title : "Docs") + " · MCP Runtime";
    })
    .catch(function (e) {
      content.innerHTML = "<h1>Could not load " + current + "</h1><p>" + e +
        "</p><p>If viewing locally, serve this folder over HTTP (e.g. <code>python3 -m http.server</code>) — <code>file://</code> blocks fetch.</p>";
    });
})();

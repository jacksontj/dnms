// dnms web UI.
//
// One page that talks to either the mapper-side or aggregator-side HTTP API,
// renders the network graph via Cytoscape, and colors links by the
// route-correlation suspicion score from /v1/graph/edges/health.
//
// Same-origin: the Go binary serves this HTML from / and the JSON API from
// /v1/..., so no cross-origin requests and no CORS headache.

(function () {
    const $ = (sel) => document.querySelector(sel);

    const state = {
        aggregator: false,           // toggled by header checkbox
        cy: null,                    // cytoscape instance
        eventSource: null,           // SSE for live graph mutations
        healthTimer: null,           // poll timer for edges/health
        graph: null,                 // last-loaded /graph payload, used by the route picker
        route: { start: "", end: "" },
        search: "",                  // node-name / DNS substring filter
        history: new Map(),          // "src;dst" → ring of recent suspicion samples
        focusedEdgeId: null,         // currently-shown edge in the details panel, if any
    };

    // Sparkline history config. Polling cadence (5 s × 60 samples ≈ 5 min)
    // is enough to spot recent fluctuations without unbounded memory growth.
    const HISTORY_LIMIT = 60;

    // --- Bearer token auth -------------------------------------------------
    //
    // The server may run with -httpToken set, in which case every API request
    // needs an Authorization header (or a ?token= query param for SSE, since
    // browser EventSource can't send custom headers). We stash the token in
    // localStorage so it survives reload, prompt on first 401, and clear it
    // if a retry also 401s.

    const TOKEN_KEY = "dnms.token";
    const auth = {
        get: () => localStorage.getItem(TOKEN_KEY) || "",
        set: (t) => localStorage.setItem(TOKEN_KEY, t),
        clear: () => localStorage.removeItem(TOKEN_KEY),
    };

    function basePath() {
        return state.aggregator ? "/v1/aggregator" : "/v1";
    }

    function withTokenQuery(url) {
        const t = auth.get();
        if (!t) return url;
        const sep = url.includes("?") ? "&" : "?";
        return url + sep + "token=" + encodeURIComponent(t);
    }

    function api(path, opts = {}) {
        const t = auth.get();
        const headers = Object.assign({}, opts.headers || {});
        if (t) headers["Authorization"] = "Bearer " + t;
        return fetch(path, Object.assign({}, opts, { headers }));
    }

    function setStatus(msg) { $("#status").textContent = msg; }
    function setError(msg) {
        const el = $("#error");
        if (!msg) { el.classList.add("hidden"); el.textContent = ""; return; }
        el.classList.remove("hidden");
        el.textContent = msg;
    }

    function promptForToken(reason) {
        const current = auth.get();
        const msg = (reason ? reason + "\n\n" : "")
            + "The server is running with -httpToken set. Paste the token "
            + "to authorize this browser. Cancel to clear any stored token.";
        const next = window.prompt(msg, current);
        if (next === null) { auth.clear(); return false; }
        const trimmed = next.trim();
        if (!trimmed) { auth.clear(); return false; }
        auth.set(trimmed);
        return true;
    }

    function updateAuthButton() {
        const btn = $("#auth-btn");
        if (!btn) return;
        btn.textContent = auth.get() ? "sign out" : "sign in";
    }

    async function fetchJSON(path) {
        let r = await api(path);
        if (r.status === 401) {
            // Prompt for token, then retry once.
            const ok = promptForToken("Server returned 401 Unauthorized.");
            updateAuthButton();
            if (!ok) throw new Error(`${path}: HTTP 401 (no token)`);
            r = await api(path);
            if (r.status === 401) {
                // Wrong token; clear it so the next page load reprompts.
                auth.clear();
                updateAuthButton();
                throw new Error(`${path}: HTTP 401 (token rejected)`);
            }
        }
        if (!r.ok) throw new Error(`${path}: HTTP ${r.status}`);
        return r.json();
    }

    function suspicionColor(s) {
        // s is in [-1, 1] in theory; clamp and map to a green→yellow→red ramp.
        if (s == null || !isFinite(s)) return "#6a6f80";
        const v = Math.max(0, Math.min(1, s));
        if (v < 0.05) return "#4cd07a"; // green
        if (v < 0.25) return "#ffb53d"; // amber
        return "#ff5a6b";               // red
    }
    // Same shape as suspicionColor but tuned for raw loss rates (0..1). The
    // route-picker recolors highlighted edges by per-route loss; we want any
    // measurable loss to register, not just suspicion deltas.
    function lossColor(l) {
        if (l == null || !isFinite(l)) return "#6a6f80";
        const v = Math.max(0, Math.min(1, l));
        if (v < 0.01) return "#4cd07a"; // green
        if (v < 0.05) return "#ffb53d"; // amber
        return "#ff5a6b";               // red
    }

    function renderGraph(graph, healthByEdge) {
        const nodes = Object.entries(graph.nodes || {}).map(([id, n]) => ({
            data: { id, label: id, dnsNames: n.dns_names || [] }
        }));
        const edges = Object.entries(graph.edges || {}).map(([id, e]) => {
            const key = `${e.src};${e.dst}`;
            const h = healthByEdge ? healthByEdge[key] : null;
            return {
                data: {
                    id,
                    source: e.src,
                    target: e.dst,
                    suspicion: h ? h.suspicion : null,
                    lossThrough: h ? h.avgLossThrough : null,
                    lossNot: h ? h.avgLossNotThrough : null,
                    routesThrough: h ? h.routesThrough : 0,
                    color: suspicionColor(h && h.suspicion),
                }
            };
        });

        if (state.cy) state.cy.destroy();
        state.cy = cytoscape({
            container: $("#graph"),
            elements: [...nodes, ...edges],
            wheelSensitivity: 0.2,
            style: [
                {
                    selector: "node",
                    style: {
                        "background-color": "#6ea8ff",
                        label: "data(label)",
                        color: "#d5d8e3",
                        "font-size": "10px",
                        "text-valign": "bottom",
                        "text-margin-y": 4,
                        "text-outline-color": "#0f1116",
                        "text-outline-width": 2,
                        width: 14,
                        height: 14,
                    },
                },
                {
                    selector: "node:selected",
                    style: { "background-color": "#fff", "border-width": 2, "border-color": "#fff" },
                },
                {
                    selector: "edge",
                    style: {
                        width: 2,
                        "line-color": "data(color)",
                        "target-arrow-color": "data(color)",
                        "target-arrow-shape": "triangle",
                        "curve-style": "straight",
                        opacity: 0.85,
                    },
                },
                {
                    selector: "edge:selected",
                    style: { width: 4 },
                },
                // Route-picker focus/context styling. .route-match highlights
                // the edges/nodes on a selected (start → end) path; .route-dim
                // fades everything else so the user's eye stays on what
                // matters.
                {
                    selector: ".route-match",
                    style: {
                        "line-color": "data(routeColor)",
                        "target-arrow-color": "data(routeColor)",
                        width: 4,
                        opacity: 1,
                    },
                },
                {
                    selector: "node.route-match",
                    style: { "background-color": "data(routeColor)", "border-color": "#fff", "border-width": 2 },
                },
                {
                    selector: ".route-dim",
                    style: { opacity: 0.15 },
                },
            ],
            layout: { name: "cose", animate: false, fit: true, padding: 30 },
        });

        state.cy.on("tap", "node", (evt) => showNodeDetails(evt.target, graph));
        state.cy.on("tap", "edge", (evt) => showEdgeDetails(evt.target, graph));
        state.cy.on("tap", (evt) => {
            if (evt.target === state.cy) hideDetails();
        });

        applyFilters();
    }

    function showNodeDetails(node, graph) {
        state.focusedEdgeId = null;
        const id = node.id();
        const data = node.data();
        const incidentEdges = node.connectedEdges();
        const html = `
            <h2>Node ${escape(id)}</h2>
            <dl>
                <dt>name</dt><dd>${escape(id)}</dd>
                <dt>DNS</dt><dd>${data.dnsNames.length ? data.dnsNames.map(escape).join("<br>") : "<span style='color:var(--muted)'>(none)</span>"}</dd>
                <dt>incident links</dt><dd>${incidentEdges.length}</dd>
            </dl>
            <h3>Connected links</h3>
            <ul>
                ${incidentEdges.map(e => `<li>${escape(e.data("source"))} → ${escape(e.data("target"))} ${formatSuspicionInline(e.data("suspicion"))}</li>`).join("")}
            </ul>
        `;
        showDetails(html);
    }

    function showEdgeDetails(edge, graph) {
        const d = edge.data();
        const key = `${d.source};${d.target}`;
        state.focusedEdgeId = edge.id();
        const routes = routesThroughLink(graph, d.source, d.target);
        const spark = sparklineSVG(key);
        const buf = state.history.get(key) || [];
        const sparkHTML = spark
            ? `<div class="spark-wrap">${spark}<div class="spark-caption">suspicion · last ${buf.length} sample${buf.length === 1 ? "" : "s"} (5s cadence)</div></div>`
            : `<div class="spark-caption" style="color:var(--muted)">(no samples yet)</div>`;
        const html = `
            <h2>Link ${escape(d.source)} → ${escape(d.target)}</h2>
            <dl>
                <dt>suspicion</dt><dd>${formatNumber(d.suspicion)}</dd>
                <dt>avg loss (routes through)</dt><dd>${formatPercent(d.lossThrough)}</dd>
                <dt>avg loss (routes not through)</dt><dd>${formatPercent(d.lossNot)}</dd>
                <dt>routes through</dt><dd>${d.routesThrough}</dd>
            </dl>
            <h3>Recent suspicion</h3>
            ${sparkHTML}
            <h3>Routes through this link (${routes.length})</h3>
            <ul>
                ${routes.map(r => `<li>${escape(r.summary)} <span style="color:var(--muted)">(loss ${formatPercent(r.lossRate)}, ${r.numPoints} samples)</span></li>`).join("") || "<li><span style='color:var(--muted)'>(none with samples)</span></li>"}
            </ul>
        `;
        showDetails(html);
    }

    function routesThroughLink(graph, src, dst) {
        const out = [];
        for (const [, r] of Object.entries(graph.routes || {})) {
            const path = r.path || [];
            for (let i = 1; i < path.length; i++) {
                if (path[i - 1] === src && path[i] === dst) {
                    out.push({
                        summary: path.join(" → "),
                        lossRate: r.metrics ? r.metrics.lossRate : 0,
                        numPoints: r.metrics ? r.metrics.numPoints : 0,
                    });
                    break;
                }
            }
        }
        return out;
    }

    function showDetails(html) {
        $("#details-body").innerHTML = html;
        $("#details").classList.remove("hidden");
    }
    function hideDetails() {
        $("#details").classList.add("hidden");
        state.focusedEdgeId = null;
    }

    function escape(s) {
        return String(s).replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c]));
    }
    function formatNumber(n) {
        if (n == null || !isFinite(n)) return "–";
        return n.toFixed(3);
    }
    function formatPercent(n) {
        if (n == null || !isFinite(n)) return "–";
        return (n * 100).toFixed(1) + "%";
    }
    function formatSuspicionInline(s) {
        if (s == null) return "";
        const color = suspicionColor(s);
        return `<span style="color:${color}">[s=${s.toFixed(2)}]</span>`;
    }

    async function loadAll() {
        try {
            setStatus("loading…");
            const [graph, health] = await Promise.all([
                fetchJSON(basePath() + "/graph"),
                fetchJSON(basePath() + "/graph/edges/health").catch(() => []),
            ]);
            const healthByEdge = {};
            for (const h of health) healthByEdge[`${h.src};${h.dst}`] = h;
            state.graph = graph;
            updateCounts(graph);
            renderGraph(graph, healthByEdge);
            populateRoutePicker(graph);
            setStatus(`loaded @ ${new Date().toLocaleTimeString()}`);
            setError(null);
        } catch (err) {
            setError(err.message);
            setStatus("error");
        }
    }

    // --- Route picker ------------------------------------------------------
    //
    // Pick a start + end node from the dropdowns and we highlight every
    // route whose path passes through start somewhere before end. Each
    // highlighted edge is colored by the worst (highest-loss) route that
    // traverses it — so the operator sees both "which paths matter for this
    // pair" and "where the loss is concentrated."

    function populateRoutePicker(graph) {
        const ids = Object.keys(graph.nodes || {}).sort();
        for (const sel of ["#route-start", "#route-end"]) {
            const el = $(sel);
            const current = el.value;
            const placeholder = sel === "#route-start" ? "(start)" : "(end)";
            el.innerHTML = `<option value="">${placeholder}</option>`
                + ids.map(id => `<option value="${escape(id)}">${escape(id)}</option>`).join("");
            // Preserve the user's pick across reloads if the node still exists.
            if (current && ids.includes(current)) el.value = current;
        }
    }

    function routesBetween(graph, start, end) {
        const out = [];
        if (!graph || !start || !end || start === end) return out;
        for (const [id, r] of Object.entries(graph.routes || {})) {
            const path = r.path || [];
            const si = path.indexOf(start);
            if (si < 0) continue;
            const ei = path.indexOf(end, si + 1);
            if (ei < 0) continue;
            out.push({
                id,
                path,
                segment: path.slice(si, ei + 1),
                lossRate: r.metrics ? r.metrics.lossRate : 0,
                numPoints: r.metrics ? r.metrics.numPoints : 0,
            });
        }
        return out;
    }

    // searchMatchedNodes returns the set of node ids matching the search
    // query (id or any DNS name substring, case-insensitive), or null if
    // the search filter is empty.
    function searchMatchedNodes() {
        const q = state.search.trim().toLowerCase();
        if (!q) return null;
        const matches = new Set();
        const nodes = state.graph ? state.graph.nodes || {} : {};
        for (const [id, n] of Object.entries(nodes)) {
            if (id.toLowerCase().includes(q)) { matches.add(id); continue; }
            const dns = n.dns_names || [];
            if (dns.some(d => String(d).toLowerCase().includes(q))) matches.add(id);
        }
        return matches;
    }

    // routeAttribution returns {edgeWorst, nodeWorst, count} describing the
    // route-picker selection, or null if the picker is inactive.
    function routeAttribution() {
        const start = state.route.start, end = state.route.end;
        if (!start || !end) return null;
        const routes = routesBetween(state.graph, start, end);
        const edgeWorst = new Map();
        const nodeWorst = new Map();
        for (const r of routes) {
            for (let i = 1; i < r.segment.length; i++) {
                const key = `${r.segment[i - 1]};${r.segment[i]}`;
                const prev = edgeWorst.get(key) ?? -Infinity;
                if (r.lossRate > prev) edgeWorst.set(key, r.lossRate);
            }
            for (const n of r.segment) {
                const prev = nodeWorst.get(n) ?? -Infinity;
                if (r.lossRate > prev) nodeWorst.set(n, r.lossRate);
            }
        }
        return { edgeWorst, nodeWorst, count: routes.length, start, end };
    }

    // applyFilters composes the route-picker highlight and the text-search
    // filter into a single pass over the graph. An element is fully visible
    // only if it passes BOTH active filters; if either filter excludes it,
    // it dims. The route picker also colors matched edges/nodes by per-route
    // loss; the search filter doesn't color, only gates visibility.
    function applyFilters() {
        if (!state.cy) return;
        const cy = state.cy;
        cy.elements().removeClass("route-match route-dim");
        cy.edges().removeData("routeColor");
        cy.nodes().removeData("routeColor");

        const route = routeAttribution();
        const searchSet = searchMatchedNodes();
        if (!route && !searchSet) {
            setStatus(`loaded @ ${new Date().toLocaleTimeString()}`);
            return;
        }

        // For the search filter, edges count as "matched" if either endpoint
        // matches — that way searching for a single host still shows its
        // immediate connections in context.
        const nodePassesSearch = (id) => !searchSet || searchSet.has(id);
        const edgePassesSearch = (src, dst) => !searchSet || searchSet.has(src) || searchSet.has(dst);

        const nodePassesRoute = (id) => !route || route.nodeWorst.has(id);
        const edgePassesRoute = (src, dst) => !route || route.edgeWorst.has(`${src};${dst}`);

        cy.nodes().forEach((n) => {
            const id = n.id();
            if (nodePassesRoute(id) && nodePassesSearch(id)) {
                if (route) {
                    const loss = route.nodeWorst.get(id);
                    if (loss != null) n.data("routeColor", lossColor(loss));
                    n.addClass("route-match");
                }
            } else {
                n.addClass("route-dim");
            }
        });
        cy.edges().forEach((e) => {
            const src = e.data("source"), dst = e.data("target");
            if (edgePassesRoute(src, dst) && edgePassesSearch(src, dst)) {
                if (route && route.edgeWorst.has(`${src};${dst}`)) {
                    e.data("routeColor", lossColor(route.edgeWorst.get(`${src};${dst}`)));
                    e.addClass("route-match");
                }
            } else {
                e.addClass("route-dim");
            }
        });

        // Status line summarizes whichever filters are active.
        const bits = [];
        if (route) bits.push(`${route.count} route${route.count === 1 ? "" : "s"} ${route.start} → ${route.end}`);
        if (searchSet) bits.push(`${searchSet.size} match${searchSet.size === 1 ? "" : "es"} for "${state.search}"`);
        setStatus(bits.join("  ·  "));
    }

    function onRoutePickerChange() {
        state.route.start = $("#route-start").value;
        state.route.end = $("#route-end").value;
        applyFilters();
    }

    function clearRoutePicker() {
        state.route.start = "";
        state.route.end = "";
        $("#route-start").value = "";
        $("#route-end").value = "";
        applyFilters();
    }

    function onSearchChange() {
        state.search = $("#search").value;
        applyFilters();
    }

    async function refreshHealth() {
        if (!state.cy) return;
        try {
            const health = await fetchJSON(basePath() + "/graph/edges/health");
            const byKey = {};
            const sampledAt = Date.now();
            for (const h of health) {
                byKey[`${h.src};${h.dst}`] = h;
                pushHistory(`${h.src};${h.dst}`, sampledAt, h.suspicion);
            }
            state.cy.edges().forEach((e) => {
                const key = `${e.data("source")};${e.data("target")}`;
                const h = byKey[key];
                if (h) {
                    e.data("suspicion", h.suspicion);
                    e.data("lossThrough", h.avgLossThrough);
                    e.data("lossNot", h.avgLossNotThrough);
                    e.data("routesThrough", h.routesThrough);
                    e.data("color", suspicionColor(h.suspicion));
                }
            });
            // If the user is looking at a link's details right now, refresh
            // its panel — the sparkline just got a new sample.
            if (state.focusedEdgeId) {
                const edge = state.cy.getElementById(state.focusedEdgeId);
                if (edge && edge.length) showEdgeDetails(edge, state.graph);
            }
            setStatus(`health refreshed @ ${new Date().toLocaleTimeString()}`);
        } catch (err) {
            setError(err.message);
        }
    }

    function pushHistory(key, t, suspicion) {
        let buf = state.history.get(key);
        if (!buf) {
            buf = [];
            state.history.set(key, buf);
        }
        buf.push({ t, s: suspicion });
        if (buf.length > HISTORY_LIMIT) buf.splice(0, buf.length - HISTORY_LIMIT);
    }

    // sparklineSVG returns an SVG string plotting the last N suspicion
    // samples for the given link key. Returns "" if no samples yet (so the
    // panel can render a friendly placeholder).
    function sparklineSVG(key) {
        const buf = state.history.get(key);
        if (!buf || buf.length === 0) return "";
        const W = 240, H = 44, PAD = 2;
        // Y range: clamp to [-0.2, 1] so most suspicion values land in a
        // stable band. The zero line stays in roughly the same spot.
        const yMin = -0.2, yMax = 1.0;
        const yToPx = (v) => {
            const clamped = Math.max(yMin, Math.min(yMax, v));
            return PAD + (H - 2 * PAD) * (1 - (clamped - yMin) / (yMax - yMin));
        };
        const xToPx = (i) => {
            if (buf.length === 1) return W / 2;
            return PAD + (W - 2 * PAD) * (i / (buf.length - 1));
        };
        const pts = buf.map((p, i) => `${xToPx(i).toFixed(1)},${yToPx(p.s).toFixed(1)}`).join(" ");
        const last = buf[buf.length - 1].s;
        const lineColor = suspicionColor(last);
        const zeroY = yToPx(0).toFixed(1);
        return `
            <svg class="spark" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" xmlns="http://www.w3.org/2000/svg">
                <line x1="0" y1="${zeroY}" x2="${W}" y2="${zeroY}" stroke="rgba(255,255,255,0.08)" stroke-width="1"/>
                <polyline fill="none" stroke="${lineColor}" stroke-width="1.5" points="${pts}"/>
            </svg>
        `;
    }

    function updateCounts(graph) {
        $("#cnt-nodes").textContent = Object.keys(graph.nodes || {}).length;
        $("#cnt-links").textContent = Object.keys(graph.edges || {}).length;
        $("#cnt-routes").textContent = Object.keys(graph.routes || {}).length;
        // peers count isn't in /graph; fetch separately, best effort.
        // Always hits the mapper-side endpoint (the aggregator doesn't expose
        // its own peers list — that's a node-local concept).
        api("/v1/mapper/peers")
            .then(r => r.ok ? r.json() : [])
            .then(peers => { $("#cnt-peers").textContent = peers.length; })
            .catch(() => { $("#cnt-peers").textContent = "–"; });
    }

    function startSSE() {
        if (state.eventSource) { state.eventSource.close(); state.eventSource = null; }
        if (!window.EventSource) return;
        // EventSource can't send headers, so we pass the token (if any) as a
        // query parameter — the server's HTTPSecurity middleware accepts both.
        const url = withTokenQuery(basePath() + "/events/graph");
        const es = new EventSource(url);
        // Live mutation events: just trigger a full reload. Cheap, keeps the
        // graph in sync without us reimplementing the diff logic per-event.
        const reload = () => loadAll();
        ["addNodeEvent","removeNodeEvent","updateNodeEvent","addLinkEvent","removeLinkEvent","addRouteEvent","removeRouteEvent","updateRouteEvent"]
            .forEach(name => es.addEventListener(name, reload));
        es.onerror = () => { /* EventSource auto-reconnects */ };
        state.eventSource = es;
    }

    function startHealthPolling() {
        if (state.healthTimer) clearInterval(state.healthTimer);
        // Attribution is query-time on the server, no event stream, so poll.
        state.healthTimer = setInterval(refreshHealth, 5000);
    }

    function switchView() {
        state.aggregator = $("#view-aggregator").checked;
        // History was accumulated against the previous data source; throw it
        // out so the sparklines don't mix mapper and aggregator measurements.
        state.history.clear();
        hideDetails();
        loadAll();
        startSSE();
    }

    function onAuthClick() {
        if (auth.get()) {
            auth.clear();
            updateAuthButton();
            setStatus("token cleared; subsequent requests will be unauthenticated");
            // Force a reload so any cached EventSource reconnects without the token.
            startSSE();
            loadAll();
            return;
        }
        if (promptForToken()) {
            updateAuthButton();
            startSSE();
            loadAll();
        }
    }

    document.addEventListener("DOMContentLoaded", () => {
        $("#view-aggregator").addEventListener("change", switchView);
        $("#refresh").addEventListener("click", loadAll);
        $("#details-close").addEventListener("click", hideDetails);
        $("#auth-btn").addEventListener("click", onAuthClick);
        $("#route-start").addEventListener("change", onRoutePickerChange);
        $("#route-end").addEventListener("change", onRoutePickerChange);
        $("#route-clear").addEventListener("click", clearRoutePicker);
        $("#search").addEventListener("input", onSearchChange);
        updateAuthButton();
        loadAll();
        startSSE();
        startHealthPolling();
    });
})();

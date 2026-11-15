(function () {
  var qlx = window.qlx = window.qlx || {};
  var DEBOUNCE_MS = 80;
  var LIMIT = 20;

  var dialog = null;
  var input = null;
  var list = null;
  var status = null;
  var results = [];
  var activeIndex = -1;
  var timer = null;
  var controller = null;
  var opener = null;

  function isTyping() {
    var el = document.activeElement;
    if (!el) return false;
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || !!el.isContentEditable;
  }

  function t(key) { return qlx.t ? qlx.t(key) : key; }

  /** Navigate like an hx-get link into #content; the global hook syncs the URL. */
  function go(id) {
    var url = "/containers/" + encodeURIComponent(id);
    if (window.htmx) {
      window.htmx.ajax("GET", url, { target: "#content" });
    } else {
      window.location.assign(url);
    }
  }

  // ── Dialog ────────────────────────────────────────────────────────────
  function ensure() {
    if (dialog) return true;
    dialog = document.getElementById("quick-switcher");
    if (!dialog) return false;
    input = document.getElementById("qs-input");
    list = document.getElementById("qs-list");
    status = document.getElementById("qs-status");

    input.addEventListener("input", function () {
      clearTimeout(timer);
      timer = setTimeout(function () { load(input.value); }, DEBOUNCE_MS);
    });
    input.addEventListener("keydown", onInputKey);
    list.addEventListener("click", function (e) {
      var li = e.target instanceof Element ? e.target.closest("[role=option]") : null;
      if (!li) return;
      choose(parseInt(li.getAttribute("data-index"), 10));
    });
    dialog.addEventListener("click", function (e) {
      if (e.target === dialog) dialog.close();
    });
    dialog.addEventListener("close", onClose);
    return true;
  }

  function open() {
    if (!ensure() || dialog.open) return;
    opener = document.activeElement;
    input.value = "";
    render([]);
    dialog.showModal();
    input.setAttribute("aria-expanded", "true");
    input.focus();
    load("");
  }

  function onClose() {
    clearTimeout(timer);
    if (controller) controller.abort();
    controller = null;
    input.setAttribute("aria-expanded", "false");
    input.setAttribute("aria-activedescendant", "");
    var target = opener;
    opener = null;
    if (target && target.isConnected && typeof target.focus === "function") target.focus();
  }

  function onInputKey(e) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (results.length === 0) return;
      var n = results.length;
      setActive((activeIndex + (e.key === "ArrowDown" ? 1 : -1) + n) % n);
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(activeIndex);
    } else if (e.key === "Tab") {
      // Focus trap: the input is the only tab stop in the dialog.
      e.preventDefault();
    }
  }

  function choose(index) {
    var r = results[index];
    if (!r) return;
    opener = null; // the swapped content replaces the opener; do not refocus it
    dialog.close();
    go(r.id);
  }

  // ── Data ──────────────────────────────────────────────────────────────
  function load(q) {
    if (controller) controller.abort();
    controller = new AbortController();
    var ctl = controller;
    fetch("/api/quick-switch?limit=" + LIMIT + "&q=" + encodeURIComponent(q), {
      signal: ctl.signal,
      headers: { Accept: "application/json" }
    })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      })
      .then(function (data) {
        if (ctl !== controller) return;
        render(Array.isArray(data) ? data : [], q);
      })
      .catch(function (err) {
        if (err && err.name === "AbortError") return;
        console.error("quick switch failed:", err);
        if (qlx.showToast) qlx.showToast(t("error.connection"), true);
      });
  }

  // ── Rendering ─────────────────────────────────────────────────────────
  function render(data, q) {
    results = data;
    while (list.firstChild) list.removeChild(list.firstChild);
    for (var i = 0; i < data.length; i++) list.appendChild(buildOption(data[i], i));
    activeIndex = data.length ? 0 : -1;
    syncActive();
    var msg = "";
    if (q !== undefined && data.length === 0) msg = t(q ? "switcher.no_results" : "switcher.empty");
    status.textContent = msg;
    status.hidden = !msg;
  }

  function buildOption(r, i) {
    var li = document.createElement("li");
    li.id = "qs-opt-" + i;
    li.className = "qs-option";
    li.setAttribute("role", "option");
    li.setAttribute("data-index", String(i));
    li.setAttribute("aria-selected", "false");

    var main = document.createElement("span");
    main.className = "qs-main";
    var name = document.createElement("span");
    name.className = "qs-name";
    name.textContent = r.name;
    main.appendChild(name);
    if (r.pinned) {
      var pin = document.createElement("span");
      pin.className = "qs-pin";
      pin.textContent = "★";
      pin.setAttribute("role", "img");
      pin.setAttribute("aria-label", t("switcher.pinned"));
      main.appendChild(pin);
    }
    li.appendChild(main);

    var parent = parentPath(r);
    if (parent) {
      var path = document.createElement("span");
      path.className = "qs-path";
      path.textContent = parent;
      li.appendChild(path);
    }

    var count = document.createElement("span");
    count.className = "qs-count";
    count.textContent = String(r.item_count || 0);
    count.title = t("rail.items_count");
    li.appendChild(count);
    return li;
  }

  function parentPath(r) {
    var p = r.path || "";
    var suffix = " / " + r.name;
    if (p.length >= suffix.length && p.slice(-suffix.length) === suffix) return p.slice(0, -suffix.length);
    return "";
  }

  function setActive(i) {
    activeIndex = i;
    syncActive();
  }

  function syncActive() {
    var opts = list.children;
    for (var i = 0; i < opts.length; i++) {
      opts[i].setAttribute("aria-selected", i === activeIndex ? "true" : "false");
    }
    if (activeIndex >= 0 && opts[activeIndex]) {
      input.setAttribute("aria-activedescendant", opts[activeIndex].id);
      opts[activeIndex].scrollIntoView({ block: "nearest" });
    } else {
      input.setAttribute("aria-activedescendant", "");
    }
  }

  // ── Global shortcuts ──────────────────────────────────────────────────
  document.addEventListener("keydown", function (e) {
    if ((e.ctrlKey || e.metaKey) && !e.altKey && (e.key === "k" || e.key === "K")) {
      e.preventDefault();
      e.stopImmediatePropagation();
      if (ensure() && dialog.open) dialog.close();
      else open();
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
    if (e.key.length !== 1 || e.key < "1" || e.key > "9") return;
    if (e.defaultPrevented || isTyping() || document.querySelector("dialog[open]")) return;
    e.preventDefault();
    jumpToPin(parseInt(e.key, 10) - 1);
  }, true);

  function jumpToPin(index) {
    fetch("/api/working-set", { headers: { Accept: "application/json" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      })
      .then(function (data) {
        var pin = data && data.pins && data.pins[index];
        if (pin) go(pin.id);
      })
      .catch(function (err) { console.error("pin jump failed:", err); });
  }

  /** Show 1-9 hints next to the first nine pinned rail entries. */
  function markRailDigits() {
    var entries = document.querySelectorAll('#working-rail [data-rail="pins"] .rail-entry');
    for (var i = 0; i < entries.length; i++) {
      if (i < 9) entries[i].setAttribute("data-digit", String(i + 1));
      else entries[i].removeAttribute("data-digit");
    }
  }
  document.body.addEventListener("htmx:afterSettle", function (e) {
    if (e.detail && e.detail.target && e.detail.target.id === "working-rail") markRailDigits();
  });

  qlx.openQuickSwitcher = open;
})();

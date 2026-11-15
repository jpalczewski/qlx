(function () {
  "use strict";

  var qlx = window.qlx = window.qlx || {};

  function t(key) {
    return qlx.t ? qlx.t(key) : key;
  }

  function toast(message, isError) {
    if (qlx.showToast) qlx.showToast(message, isError);
  }

  function containerURL(id, query) {
    return "/containers/" + encodeURIComponent(id) + (query || "");
  }

  /** Split "name x5" into {name, qty}; the token must stand alone as a word. */
  function parseQuickAdd(text) {
    var name = text.replace(/\s+/g, " ").trim();
    var qty = 1;
    var m = name.match(/(?:^|\s)x(\d+)(?=\s|$)/);
    if (m && parseInt(m[1], 10) > 0) {
      qty = parseInt(m[1], 10);
      name = name.replace(m[0], " ").replace(/\s+/g, " ").trim();
    }
    return { name: name, qty: qty };
  }

  // ------------------------------------------------------------------
  // Card quick-add
  // ------------------------------------------------------------------

  /** Item count in words, mirroring PageData.Plural on the server. */
  function countText(n) {
    var form = "many";
    if (n === 1) form = "one";
    else if ((document.documentElement.lang || "") === "pl" &&
             n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14)) form = "few";
    return t("home.count_" + form).replace("{n}", String(n));
  }

  function setExpanded(card, open, focus) {
    var toggle = card.querySelector(".home-card-add-toggle");
    var form = card.querySelector("form.home-card-add");
    if (!toggle || !form) return;
    form.hidden = !open;
    toggle.hidden = open;
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
    if (open) {
      form.querySelector("input").focus();
    } else if (focus) {
      toggle.focus();
    }
  }

  function initCardAdd(root) {
    root.addEventListener("click", function (e) {
      var toggle = e.target.closest ? e.target.closest(".home-card-add-toggle") : null;
      if (toggle) setExpanded(toggle.closest(".home-card"), true, false);
    });
    root.addEventListener("keydown", function (e) {
      if (e.key !== "Escape") return;
      var form = e.target.closest ? e.target.closest("form.home-card-add") : null;
      if (!form) return;
      e.preventDefault();
      setExpanded(form.closest(".home-card"), false, true);
    });
    root.addEventListener("focusout", function (e) {
      var form = e.target.closest ? e.target.closest("form.home-card-add") : null;
      if (!form || form.contains(e.relatedTarget) || form.getAttribute("data-busy") === "true") return;
      if (form.querySelector("input").value === "") setExpanded(form.closest(".home-card"), false, false);
    });
    root.addEventListener("submit", function (e) {
      var form = e.target.closest ? e.target.closest("form.home-card-add") : null;
      if (!form) return;
      e.preventDefault();
      if (form.getAttribute("data-busy") === "true") return;
      var input = form.querySelector("input");
      var parsed = parseQuickAdd(input.value);
      if (!parsed.name) return;
      var card = form.closest(".home-card");
      var id = card.getAttribute("data-container-id");

      form.setAttribute("data-busy", "true");
      fetch("/items", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Accept": "application/json" },
        body: JSON.stringify({ name: parsed.name, container_id: id, quantity: parsed.qty })
      }).then(function (resp) {
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        return resp.json();
      }).then(function () {
        var count = card.querySelector(".home-card-count");
        if (count) {
          var n = (parseInt(count.getAttribute("data-count"), 10) || 0) + 1;
          count.setAttribute("data-count", String(n));
          count.textContent = countText(n);
        }
        input.value = "";
        toast(t("home.card_add_done").replace("{name}", parsed.name), false);
        if (qlx.invalidateContainerCache) qlx.invalidateContainerCache();
        if (window.htmx) window.htmx.trigger(document.body, "workingSetChanged");
      }).catch(function (err) {
        console.error("home quick-add failed:", err);
        // The typed text stays in the field so nothing is lost.
        toast(t("inventory.quick_entry_failed"), true);
      }).then(function () {
        form.removeAttribute("data-busy");
      });
    });
  }

  // ------------------------------------------------------------------
  // Print shortcut and result links: remember what to do on arrival
  // ------------------------------------------------------------------

  function initIntents(root) {
    root.addEventListener("click", function (e) {
      var a = e.target.closest ? e.target.closest("a[data-intent]") : null;
      if (!a) return;
      var intent = { print: a.getAttribute("data-intent") === "print" };
      var item = a.getAttribute("data-item-id");
      if (item) intent.item = item;
      qlx.pendingContainerIntent = intent;
    });
  }

  // ------------------------------------------------------------------
  // Wide search: containers first, then items
  // ------------------------------------------------------------------

  function initSearch(root) {
    var input = root.querySelector("#home-search");
    var results = root.querySelector("#home-search-results");
    var lists = root.querySelector("#home-lists");
    if (!input || !results || !lists) return;
    var timer = null;
    var seq = 0;
    var controller = null;
    var pending = false;
    var openWhenReady = false;

    function clearResults() {
      while (results.firstChild) results.removeChild(results.firstChild);
    }

    function showLists(show) {
      lists.hidden = !show;
      results.hidden = show;
    }

    function groupLabel(text, key) {
      var li = document.createElement("li");
      li.className = "home-results-group";
      li.setAttribute("data-group", key);
      li.setAttribute("role", "presentation");
      li.textContent = text;
      return li;
    }

    function resultLink(href, intent, itemId, nameText, pathText, extraText) {
      var li = document.createElement("li");
      li.className = "home-result";
      var a = document.createElement("a");
      a.href = href;
      a.className = "home-result-link";
      a.setAttribute("hx-get", href);
      a.setAttribute("hx-target", "#content");
      if (intent) a.setAttribute("data-intent", intent);
      if (itemId) a.setAttribute("data-item-id", itemId);
      var name = document.createElement("span");
      name.className = "home-result-name";
      var nameEl = document.createElement("span");
      nameEl.className = "home-card-name-text";
      nameEl.textContent = nameText;
      name.appendChild(nameEl);
      if (extraText) {
        var extra = document.createElement("span");
        extra.className = "home-result-extra";
        extra.textContent = extraText;
        name.appendChild(extra);
      }
      var path = document.createElement("span");
      path.className = "home-card-path";
      path.textContent = pathText || "";
      a.appendChild(name);
      a.appendChild(path);
      li.appendChild(a);
      results.appendChild(li);
      if (window.htmx) window.htmx.process(a);
    }

    function render(containers, items) {
      clearResults();
      if (!containers.length && !items.length) {
        var empty = document.createElement("li");
        empty.className = "home-results-empty";
        empty.textContent = results.getAttribute("data-empty") || "";
        results.appendChild(empty);
        return;
      }
      if (containers.length) {
        results.appendChild(groupLabel(results.getAttribute("data-label-containers") || "", "containers"));
        containers.forEach(function (m) {
          resultLink(containerURL(m.id), null, null, (m.pinned ? "★ " : "") + m.name, m.path, "");
        });
      }
      if (items.length) {
        results.appendChild(groupLabel(results.getAttribute("data-label-items") || "", "items"));
        items.forEach(function (m) {
          var where = m.path || m.container_name || "";
          resultLink(containerURL(m.container_id, "?item=" + encodeURIComponent(m.id)), "item", m.id,
            m.name, where, m.quantity > 1 ? "× " + m.quantity : "");
        });
      }
    }

    function getJSON(url, signal) {
      return fetch(url, { signal: signal, headers: { "Accept": "application/json" } })
        .then(function (r) {
          if (!r.ok) throw new Error("HTTP " + r.status);
          return r.json();
        });
    }

    function search() {
      var q = input.value.trim();
      seq++;
      if (controller) controller.abort();
      controller = null;
      pending = false;
      if (!q) {
        clearResults();
        showLists(true);
        return;
      }
      var mine = seq;
      controller = new AbortController();
      var signal = controller.signal;
      pending = true;
      var enc = encodeURIComponent(q);
      Promise.all([
        getJSON("/api/quick-switch?limit=8&q=" + enc, signal),
        getJSON("/api/search/items?limit=12&q=" + enc, signal)
      ]).then(function (res) {
        if (mine !== seq) return;
        pending = false;
        showLists(false);
        render(res[0] || [], res[1] || []);
        if (openWhenReady) {
          openWhenReady = false;
          var first = links()[0];
          if (first) first.click();
        }
      }).catch(function (err) {
        if (err && err.name === "AbortError") return;
        console.error("home search failed:", err);
        if (mine !== seq) return;
        pending = false;
        openWhenReady = false;
        toast(t("error.connection"), true);
      });
    }

    function links() {
      return Array.prototype.slice.call(results.querySelectorAll(".home-result-link"));
    }

    function fire() {
      timer = null;
      search();
    }

    input.addEventListener("input", function () {
      openWhenReady = false;
      clearTimeout(timer);
      timer = setTimeout(fire, 150);
    });

    input.addEventListener("keydown", function (e) {
      if (e.key === "Enter") {
        e.preventDefault();
        if (timer || pending) {
          // The visible list is stale: search now and open the first hit on arrival.
          openWhenReady = true;
          if (timer) {
            clearTimeout(timer);
            fire();
          }
          return;
        }
        var first = links()[0];
        if (first && !results.hidden) first.click();
      } else if (e.key === "ArrowDown") {
        var link = links()[0] || lists.querySelector(".home-card-link");
        if (link) {
          e.preventDefault();
          link.focus();
        }
      } else if (e.key === "Escape" && input.value) {
        input.value = "";
        openWhenReady = false;
        search();
      }
    });

    results.addEventListener("keydown", function (e) {
      if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
      var all = links();
      var i = all.indexOf(document.activeElement);
      if (i < 0) return;
      e.preventDefault();
      if (e.key === "ArrowDown") {
        if (i + 1 < all.length) all[i + 1].focus();
      } else if (i > 0) {
        all[i - 1].focus();
      } else {
        input.focus();
      }
    });
  }

  qlx.initHome = function (root) {
    if (!root || root.getAttribute("data-home-init") === "true") return;
    root.setAttribute("data-home-init", "true");
    initCardAdd(root);
    initIntents(root);
    initSearch(root);
  };

  var views = document.querySelectorAll(".home-view");
  if (views.length) qlx.initHome(views[views.length - 1]);
})();

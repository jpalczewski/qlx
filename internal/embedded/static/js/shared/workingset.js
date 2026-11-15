(function () {
  var qlx = window.qlx = window.qlx || {};
  var UUID_RE = /^\/containers\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\/?$/i;
  var lastRecorded = null; // container id of the visit already counted

  function currentContainerID() {
    var m = UUID_RE.exec(window.location.pathname);
    return m ? m[1] : null;
  }

  function notifyChanged() {
    if (window.htmx) window.htmx.trigger(document.body, "workingSetChanged");
  }

  /** Mark the rail link of the visible container with aria-current. */
  function syncCurrent() {
    var id = currentContainerID();
    var links = document.querySelectorAll("#working-rail .rail-link");
    for (var i = 0; i < links.length; i++) {
      var entry = links[i].closest(".rail-entry");
      if (id && entry && entry.getAttribute("data-container-id") === id) {
        links[i].setAttribute("aria-current", "page");
      } else {
        links[i].removeAttribute("aria-current");
      }
    }
  }

  /** Count one visit per navigation to a container (not per swap of the same URL). */
  function recordVisit() {
    var id = currentContainerID();
    if (!id) {
      lastRecorded = null;
      return;
    }
    if (id === lastRecorded) return;
    lastRecorded = id;
    fetch("/api/working-set/recents/" + encodeURIComponent(id), { method: "POST" })
      .then(function (r) { if (r.ok) notifyChanged(); })
      .catch(function (err) { console.error("record visit failed:", err); });
  }

  function setToggleState(btn, pinned) {
    var label = btn.getAttribute(pinned ? "data-label-unpin" : "data-label-pin") || "";
    btn.setAttribute("aria-pressed", pinned ? "true" : "false");
    btn.setAttribute("aria-label", label);
    btn.setAttribute("title", label);
    var star = btn.querySelector(".pin-star");
    if (star) star.textContent = pinned ? "★" : "☆";
  }

  function setPinned(id, pinned) {
    return fetch("/api/working-set/pins/" + encodeURIComponent(id), { method: pinned ? "POST" : "DELETE" })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        var toggles = document.querySelectorAll("[data-pin-toggle]");
        for (var i = 0; i < toggles.length; i++) {
          if (toggles[i].getAttribute("data-pin-toggle") === id) setToggleState(toggles[i], pinned);
        }
        notifyChanged();
      })
      .catch(function (err) {
        console.error("pin update failed:", err);
        if (qlx.showToast) qlx.showToast(qlx.t("error.connection"), true);
      });
  }

  document.addEventListener("click", function (event) {
    var target = event.target;
    if (!(target instanceof Element)) return;
    var toggle = target.closest("[data-pin-toggle]");
    if (toggle) {
      var pressed = toggle.getAttribute("aria-pressed") === "true";
      setPinned(toggle.getAttribute("data-pin-toggle"), !pressed);
      return;
    }
    var unpin = target.closest("[data-rail-unpin]");
    if (unpin) setPinned(unpin.getAttribute("data-rail-unpin"), false);
  });

  document.body.addEventListener("htmx:afterSettle", function (event) {
    var t = event.detail && event.detail.target;
    if (!t) return;
    if (t.id === "content") recordVisit();
    if (t.id === "content" || t.id === "working-rail") syncCurrent();
  });
  window.addEventListener("popstate", function () {
    recordVisit();
    syncCurrent();
  });

  // Initial load (full page GET of /containers/{id}).
  recordVisit();
  syncCurrent();
})();

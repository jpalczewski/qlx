(function () {
  "use strict";

  var qlx = window.qlx = window.qlx || {};
  var UUID_RE = /^\/containers\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\/?$/i;

  /** Focus the container's print panel (or the add-printer link when there is no printer). */
  function openPrint() {
    var panel = document.querySelector(".print-panel, .print-section");
    if (!panel) return;
    panel.scrollIntoView({ block: "center" });
    panel.classList.add("print-panel-attention");
    var target = panel.querySelector("[data-panel-printer], .print-tab, a, button");
    if (target) target.focus({ preventScroll: true });
  }

  /** Highlight and reveal one item row of the open container. */
  function highlightItem(id) {
    var rows = document.querySelectorAll("#item-list li[data-id]");
    for (var i = 0; i < rows.length; i++) {
      if (rows[i].getAttribute("data-id") === id) {
        rows[i].classList.add("kb-active");
        rows[i].scrollIntoView({ block: "center" });
        return;
      }
    }
  }

  function stripParams() {
    var params = new URLSearchParams(window.location.search);
    if (!params.has("print") && !params.has("item")) return;
    params.delete("print");
    params.delete("item");
    var qs = params.toString();
    history.replaceState(null, "", window.location.pathname + (qs ? "?" + qs : ""));
  }

  function apply(intent) {
    if (!intent) return;
    if (intent.print) openPrint();
    if (intent.item) highlightItem(intent.item);
  }

  /** Actions requested by a card or search result on arrival at a container page. */
  function consumePending() {
    var intent = qlx.pendingContainerIntent;
    qlx.pendingContainerIntent = null;
    apply(intent);
  }

  // Direct load or reload of /containers/{id}?print=1 / ?item={id}.
  function fromURL() {
    if (!UUID_RE.test(window.location.pathname)) return;
    var params = new URLSearchParams(window.location.search);
    var intent = { print: params.get("print") === "1", item: params.get("item") || "" };
    if (intent.print || intent.item) {
      apply(intent);
      stripParams();
    }
  }

  // Deferred scripts run after parsing, so document.body exists here.
  document.addEventListener("DOMContentLoaded", fromURL);
  document.body.addEventListener("htmx:afterSettle", function (event) {
    if (event.detail && event.detail.target && event.detail.target.id === "content") consumePending();
  });
})();

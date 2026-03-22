
(function () {
  var qlx = window.qlx = window.qlx || {};

  // HTMX autofocus after swap + re-init modules
  document.body.addEventListener("htmx:afterSwap", function (event) {
    if (!event.detail || !event.detail.target) return;
    var target = event.detail.target;
    if (target.id !== "content") return;
    var autofocus = target.querySelector("[autofocus]");
    if (autofocus) autofocus.focus();
  });

  // Push URL to browser history for GET requests that replace #content.
  // This keeps the address bar in sync with the visible page so that
  // back/forward and page-relative logic (e.g. window.location.pathname)
  // works correctly. Search-input requests are excluded to avoid polluting
  // history on every keystroke.
  document.body.addEventListener("htmx:afterRequest", function (event) {
    var detail = event.detail;
    if (!detail || !detail.successful) return;
    var target = detail.target;
    if (!target || target.id !== "content") return;
    var config = detail.requestConfig;
    if (!config || config.verb !== "get") return;
    // Skip requests triggered by text/search inputs (e.g. global search box)
    var elt = detail.elt;
    if (elt && (elt.tagName === "INPUT" || elt.tagName === "TEXTAREA")) return;
    var path = config.path;
    var current = window.location.pathname + (window.location.search || "");
    if (path && path !== current) {
      history.pushState(null, "", path);
    }
  });
})();

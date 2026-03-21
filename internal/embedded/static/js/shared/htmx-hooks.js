
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
})();

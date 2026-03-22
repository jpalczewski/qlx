(function () {
  var qlx = window.qlx = window.qlx || {};

  var picker = qlx.createTreePicker({
    id: "move-picker",
    title: function () { return qlx.t("inventory.move_to_container"); },
    endpoint: "/partials/tree",
    searchEndpoint: "/partials/tree/search",
    searchPlaceholder: function () { return qlx.t("nav.search_placeholder"); },
    confirmLabel: function () { return qlx.t("action.move"); },
    onConfirm: function (targetId) {
      var ids = qlx.selectionEntries();
      fetch("/bulk/move", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids, target_container_id: targetId })
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (data) {
              qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
            });
          }
          qlx.showToast(qlx.t("bulk.moved") + " " + ids.length, false);
          qlx.clearSelection();
          htmx.ajax("GET", window.location.pathname, { target: "#content" });
        })
        .catch(function (err) {
          console.error("bulk move failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
    }
  });

  qlx.openMovePicker = function () { picker.open(); };
})();

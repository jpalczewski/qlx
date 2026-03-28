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
      var pending = picker._pendingMove;
      picker._pendingMove = null;

      if (pending) {
        // Single entity move
        var endpoint = pending.type === "container"
          ? "/containers/" + pending.id + "/move"
          : "/items/" + pending.id + "/move";
        var body = pending.type === "container"
          ? { parent_id: targetId }
          : { container_id: targetId };

        fetch(endpoint, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body)
        })
          .then(function (resp) {
            if (!resp.ok) {
              return resp.json().then(function (data) {
                qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
              });
            }
            var msg = pending.type === "container"
              ? qlx.t("inventory.container_moved")
              : qlx.t("inventory.item_moved");
            qlx.showToast(msg, false);
            htmx.ajax("GET", "/containers/" + targetId, { target: "#content" });
          })
          .catch(function (err) {
            console.error("move failed:", err);
            qlx.showToast(qlx.t("error.connection"), true);
          });
        return;
      }

      // Bulk move (existing behavior)
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

  /**
   * Open the move picker.
   * @param {Object} [opts] - optional single-entity context
   * @param {string} opts.id - entity ID
   * @param {string} opts.type - "item" or "container"
   */
  qlx.openMovePicker = function (opts) {
    picker._pendingMove = opts || null;
    picker.open();
  };
})();

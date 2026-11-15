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

        qlx.bulkRequest(endpoint, {
          method: "PATCH",
          json: body,
          success: pending.type === "container"
            ? qlx.t("inventory.container_moved")
            : qlx.t("inventory.item_moved"),
          reloadUrl: "/containers/" + targetId,
          logLabel: "move failed"
        });
        return;
      }

      // Bulk move (existing behavior)
      var ids = qlx.selectionEntries();
      qlx.bulkRequest("/bulk/move", {
        json: { ids: ids, target_container_id: targetId },
        success: qlx.t("bulk.moved") + " " + ids.length,
        afterSuccess: qlx.clearSelection,
        logLabel: "bulk move failed"
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

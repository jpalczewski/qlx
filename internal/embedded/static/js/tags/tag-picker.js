(function () {
  var qlx = window.qlx = window.qlx || {};

  var picker = qlx.createTreePicker({
    id: "tag-picker",
    title: function () { return qlx.t("tags.add_tag"); },
    endpoint: "/partials/tag-tree",
    searchEndpoint: "/partials/tag-tree/search",
    searchPlaceholder: function () { return qlx.t("tags.search_tags"); },
    confirmLabel: function () { return qlx.t("tags.tag_action"); },
    onConfirm: function (tagId) {
      var ids = qlx.selectionEntries();
      qlx.bulkRequest("/bulk/tags", {
        json: { ids: ids, tag_id: tagId },
        success: qlx.t("bulk.tagged") + " " + ids.length,
        afterSuccess: qlx.clearSelection,
        logLabel: "bulk tag failed"
      });
    }
  });

  qlx.openTagPicker = function () { picker.open(); };
})();

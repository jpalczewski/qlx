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
      fetch("/bulk/tags", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids, tag_id: tagId })
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (data) {
              qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
            });
          }
          qlx.showToast(qlx.t("bulk.tagged") + " " + ids.length, false);
          qlx.clearSelection();
          htmx.ajax("GET", window.location.pathname, { target: "#content" });
        })
        .catch(function (err) {
          console.error("bulk tag failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
    }
  });

  qlx.openTagPicker = function () { picker.open(); };
})();

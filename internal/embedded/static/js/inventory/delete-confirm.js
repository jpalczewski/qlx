
(function () {
  var qlx = window.qlx = window.qlx || {};

  function getOrCreateDeleteDialog() {
    var dlg = document.getElementById("delete-confirm-dialog");
    if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

    dlg = document.createElement("dialog");
    dlg.id = "delete-confirm-dialog";

    var picker = document.createElement("div");
    picker.className = "tree-picker";

    var msg = document.createElement("p");
    msg.id = "delete-confirm-msg";
    picker.appendChild(msg);

    var footer = document.createElement("div");
    footer.className = "tree-picker-footer";

    var cancelBtn = document.createElement("button");
    cancelBtn.className = "btn btn-secondary btn-small";
    cancelBtn.textContent = qlx.t("action.cancel");
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { /** @type {HTMLDialogElement} */ (dlg).close(); });
    footer.appendChild(cancelBtn);

    var confirmBtn = document.createElement("button");
    confirmBtn.className = "btn btn-danger btn-small";
    confirmBtn.textContent = qlx.t("action.delete");
    confirmBtn.type = "button";
    confirmBtn.addEventListener("click", function () {
      /** @type {HTMLDialogElement} */ (dlg).close();
      executeBulkDelete();
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    return /** @type {HTMLDialogElement} */ (dlg);
  }

  /** Open the delete confirmation dialog. */
  qlx.openDeleteConfirm = function openDeleteConfirm() {
    var dlg = getOrCreateDeleteDialog();
    var msg = document.getElementById("delete-confirm-msg");
    if (msg) {
      msg.textContent = qlx.t("bulk.confirm_delete") + " (" + qlx.selectionSize() + ")";
    }
    dlg.showModal();
  };

  /** Execute bulk delete of selected items. */
  function executeBulkDelete() {
    var ids = qlx.selectionEntries();
    fetch("/bulk/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids })
    })
      .then(function (resp) { return resp.json(); })
      .then(function (data) {
        var deleted = data.deleted || [];
        var failed = data.failed || [];
        deleted.forEach(function (id) {
          var el = document.querySelector("[data-id=\"" + id + "\"]");
          var li = el ? el.closest("li") : null;
          if (li) li.remove();
        });
        if (failed.length > 0) {
          qlx.showToast(qlx.t("bulk.delete_failed") + " " + failed.length, true);
        } else {
          qlx.showToast(qlx.t("bulk.deleted") + " " + deleted.length, false);
        }
        qlx.clearSelection();
      })
      .catch(function (err) {
        console.error("bulk delete failed:", err);
        qlx.showToast(qlx.t("error.connection"), true);
      });
  }
})();


(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {string|null} */
  var movePickerTargetId = null;

  // ── Tree helpers (module-private) ───────────────────────────────────────────

  /**
   * Expand or collapse a tree node, fetching children if needed.
   * @param {Element} expandEl
   * @param {HTMLElement} treeContainer
   */
  function handleTreeExpand(expandEl, treeContainer) {
    var li = expandEl.closest(".tree-node");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var ul = li.querySelector("ul.tree-branch");

    if (ul && ul.children.length > 0) {
      ul.style.display = ul.style.display === "none" ? "" : "none";
      expandEl.textContent = ul.style.display === "none" ? "\u25B6" : "\u25BC";
      return;
    }

    var endpoint = "/ui/partials/tree?parent_id=" + encodeURIComponent(id);

    fetch(endpoint)
      .then(function (r) { return r.text(); })
      .then(function (html) {
        if (!ul) {
          ul = document.createElement("ul");
          ul.className = "tree-branch";
          li.appendChild(ul);
        }
        ul.textContent = "";
        var parser = new DOMParser();
        var doc = parser.parseFromString(html, "text/html");
        while (doc.body.firstChild) {
          ul.appendChild(doc.body.firstChild);
        }
        if (window.htmx) htmx.process(ul);
        expandEl.textContent = "\u25BC";
      })
      .catch(function (err) {
        console.error("tree expand failed:", err);
      });
  }

  /**
   * Handle label selection inside a tree picker.
   * @param {Element} labelEl
   * @param {HTMLElement} treeContainer
   * @param {string} confirmBtnId
   */
  function handleTreeLabelSelect(labelEl, treeContainer, confirmBtnId) {
    treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
      el.classList.remove("selected");
    });
    labelEl.classList.add("selected");

    var li = labelEl.closest(".tree-node");
    var id = li ? li.getAttribute("data-id") : null;
    movePickerTargetId = id;

    var confirmBtn = document.getElementById(confirmBtnId);
    if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = !id;
  }

  // ── Dialog ──────────────────────────────────────────────────────────────────

  function getOrCreateMovePickerDialog() {
    var dlg = document.getElementById("move-picker");
    if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

    dlg = document.createElement("dialog");
    dlg.id = "move-picker";

    var picker = document.createElement("div");
    picker.className = "tree-picker";

    var title = document.createElement("h3");
    title.textContent = qlx.t("inventory.move_to_container");
    picker.appendChild(title);

    var searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.className = "tree-search";
    searchInput.placeholder = qlx.t("nav.search_placeholder");
    searchInput.setAttribute("hx-get", "/ui/partials/tree/search");
    searchInput.setAttribute("hx-trigger", "input changed delay:300ms");
    searchInput.setAttribute("hx-target", "#move-tree-container");
    picker.appendChild(searchInput);

    var treeContainer = document.createElement("div");
    treeContainer.id = "move-tree-container";
    treeContainer.style.flex = "1";
    treeContainer.style.overflowY = "auto";
    picker.appendChild(treeContainer);

    var footer = document.createElement("div");
    footer.className = "tree-picker-footer";

    var cancelBtn = document.createElement("button");
    cancelBtn.className = "btn btn-secondary btn-small";
    cancelBtn.textContent = qlx.t("action.cancel");
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { /** @type {HTMLDialogElement} */ (dlg).close(); });
    footer.appendChild(cancelBtn);

    var confirmBtn = document.createElement("button");
    confirmBtn.className = "btn btn-primary btn-small";
    confirmBtn.textContent = qlx.t("action.move");
    confirmBtn.type = "button";
    confirmBtn.disabled = true;
    confirmBtn.id = "move-picker-confirm";
    confirmBtn.addEventListener("click", function () {
      if (movePickerTargetId) {
        executeBulkMove(movePickerTargetId);
        /** @type {HTMLDialogElement} */ (dlg).close();
      }
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    // Delegate click events for tree nodes inside move-picker
    treeContainer.addEventListener("click", function (e) {
      var expandEl = /** @type {HTMLElement} */ (e.target).closest(".tree-expand");
      if (expandEl) {
        handleTreeExpand(expandEl, treeContainer);
        return;
      }
      var labelEl = /** @type {HTMLElement} */ (e.target).closest(".tree-label");
      if (labelEl) {
        handleTreeLabelSelect(labelEl, treeContainer, "move-picker-confirm");
      }
    });

    return /** @type {HTMLDialogElement} */ (dlg);
  }

  /** Open the move-picker dialog. */
  qlx.openMovePicker = function openMovePicker() {
    movePickerTargetId = null;
    var dlg = getOrCreateMovePickerDialog();
    var confirmBtn = document.getElementById("move-picker-confirm");
    if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = true;

    var treeContainer = document.getElementById("move-tree-container");
    if (treeContainer) {
      treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
        el.classList.remove("selected");
      });
      treeContainer.textContent = "";
    }

    // Load root tree
    fetch("/ui/partials/tree?parent_id=")
      .then(function (r) { return r.text(); })
      .then(function (html) {
        if (treeContainer) {
          treeContainer.textContent = "";
          var parser = new DOMParser();
          var doc = parser.parseFromString(html, "text/html");
          while (doc.body.firstChild) {
            treeContainer.appendChild(doc.body.firstChild);
          }
          if (window.htmx) htmx.process(treeContainer);
        }
      })
      .catch(function (err) {
        console.error("tree load failed:", err);
      });

    dlg.showModal();
  };

  /**
   * Execute bulk move of selected items to a target container.
   * @param {string} targetID
   */
  function executeBulkMove(targetID) {
    var ids = qlx.selectionEntries();
    fetch("/ui/actions/bulk/move", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids, target_container_id: targetID })
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
})();

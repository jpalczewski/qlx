
(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {string|null} */
  var tagPickerTargetId = null;

  // ── Tree helpers (module-private, duplicated from move-picker) ─────────────

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

    var endpoint = "/ui/partials/tag-tree?parent_id=" + encodeURIComponent(id);

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
   * Handle label selection inside the tag tree picker.
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
    tagPickerTargetId = id;

    var confirmBtn = document.getElementById(confirmBtnId);
    if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = !id;
  }

  // ── Dialog ──────────────────────────────────────────────────────────────────

  function getOrCreateTagPickerDialog() {
    var dlg = document.getElementById("tag-picker");
    if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

    dlg = document.createElement("dialog");
    dlg.id = "tag-picker";

    var picker = document.createElement("div");
    picker.className = "tree-picker";

    var title = document.createElement("h3");
    title.textContent = qlx.t("tags.add_tag");
    picker.appendChild(title);

    var searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.className = "tree-search";
    searchInput.placeholder = qlx.t("tags.search_tags");
    searchInput.setAttribute("hx-get", "/ui/partials/tag-tree/search");
    searchInput.setAttribute("hx-trigger", "input changed delay:300ms");
    searchInput.setAttribute("hx-target", "#tag-tree-container");
    picker.appendChild(searchInput);

    var treeContainer = document.createElement("div");
    treeContainer.id = "tag-tree-container";
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
    confirmBtn.textContent = qlx.t("tags.tag_action");
    confirmBtn.type = "button";
    confirmBtn.disabled = true;
    confirmBtn.id = "tag-picker-confirm";
    confirmBtn.addEventListener("click", function () {
      if (tagPickerTargetId) {
        executeBulkTag(tagPickerTargetId);
        /** @type {HTMLDialogElement} */ (dlg).close();
      }
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    treeContainer.addEventListener("click", function (e) {
      var expandEl = /** @type {HTMLElement} */ (e.target).closest(".tree-expand");
      if (expandEl) {
        handleTreeExpand(expandEl, treeContainer);
        return;
      }
      var labelEl = /** @type {HTMLElement} */ (e.target).closest(".tree-label");
      if (labelEl) {
        handleTreeLabelSelect(labelEl, treeContainer, "tag-picker-confirm");
      }
    });

    return /** @type {HTMLDialogElement} */ (dlg);
  }

  /** Open the tag picker dialog. */
  qlx.openTagPicker = function openTagPicker() {
    tagPickerTargetId = null;
    var dlg = getOrCreateTagPickerDialog();
    var confirmBtn = document.getElementById("tag-picker-confirm");
    if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = true;

    var treeContainer = document.getElementById("tag-tree-container");
    if (treeContainer) {
      treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
        el.classList.remove("selected");
      });
      treeContainer.textContent = "";
    }

    fetch("/ui/partials/tag-tree?parent_id=")
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
        console.error("tag tree load failed:", err);
      });

    dlg.showModal();
  };

  /**
   * Execute bulk tagging of selected items.
   * @param {string} tagID
   */
  function executeBulkTag(tagID) {
    var ids = qlx.selectionEntries();
    fetch("/ui/actions/bulk/tags", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids, tag_id: tagID })
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
})();

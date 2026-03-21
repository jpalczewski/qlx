
(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {Map<string, string>} id -> type */
  var selection = new Map();

  /**
   * Build [{id, type}] array from selection Map for bulk API calls.
   * @returns {Array<{id: string, type: string}>}
   */
  qlx.selectionEntries = function selectionEntries() {
    var entries = [];
    selection.forEach(function (type, id) {
      entries.push({ id: id, type: type });
    });
    return entries;
  };

  /**
   * Get current selection size.
   * @returns {number}
   */
  qlx.selectionSize = function selectionSize() {
    return selection.size;
  };

  /**
   * Check if an id is in the selection.
   * @param {string} id
   * @returns {boolean}
   */
  qlx.selectionHas = function selectionHas(id) {
    return selection.has(id);
  };

  /** Initialize bulk-select checkboxes and toggle button. */
  qlx.initBulkSelect = function initBulkSelect() {
    document.querySelectorAll(".bulk-select").forEach(function (cb) {
      cb.removeEventListener("change", onBulkCheckChange);
      cb.addEventListener("change", onBulkCheckChange);
    });

    var toggleBtn = document.getElementById("select-toggle-btn");
    if (toggleBtn) {
      toggleBtn.removeEventListener("click", onSelectToggle);
      toggleBtn.addEventListener("click", onSelectToggle);
    }
  };

  /** @param {Event} e */
  function onBulkCheckChange(e) {
    var cb = /** @type {HTMLInputElement} */ (e.target);
    var li = cb.closest("[data-id]");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var type = li.getAttribute("data-type") || "item";
    if (cb.checked) {
      selection.set(id, type);
    } else {
      selection.delete(id);
    }
    updateActionBar();
  }

  function onSelectToggle() {
    var content = document.getElementById("content");
    if (!content) return;
    content.classList.toggle("selection-mode");
    if (!content.classList.contains("selection-mode")) {
      qlx.clearSelection();
    }
  }

  /** Clear all selections and hide the action bar. */
  qlx.clearSelection = function clearSelection() {
    selection.clear();
    document.querySelectorAll(".bulk-select").forEach(function (cb) {
      /** @type {HTMLInputElement} */ (cb).checked = false;
    });
    var bar = document.getElementById("action-bar");
    if (bar) bar.style.display = "none";
  };

  // Backward compatibility
  window.clearSelection = qlx.clearSelection;
  window.initBulkSelect = qlx.initBulkSelect;

  // ── Action Bar ──────────────────────────────────────────────────────────────
  function getOrCreateActionBar() {
    var bar = document.getElementById("action-bar");
    if (bar) return bar;

    bar = document.createElement("div");
    bar.id = "action-bar";
    bar.className = "action-bar";
    bar.style.display = "none";

    var countSpan = document.createElement("span");
    countSpan.className = "action-count";
    bar.appendChild(countSpan);

    var moveBtn = document.createElement("button");
    moveBtn.className = "btn btn-secondary btn-small";
    moveBtn.textContent = qlx.t("action.move_to");
    moveBtn.addEventListener("click", function () {
      if (qlx.openMovePicker) qlx.openMovePicker();
    });
    bar.appendChild(moveBtn);

    var tagBtn = document.createElement("button");
    tagBtn.className = "btn btn-secondary btn-small";
    tagBtn.textContent = qlx.t("tags.tag_action") + "...";
    tagBtn.addEventListener("click", function () {
      if (qlx.openTagPicker) qlx.openTagPicker();
    });
    bar.appendChild(tagBtn);

    var deleteBtn = document.createElement("button");
    deleteBtn.className = "btn btn-danger btn-small";
    deleteBtn.textContent = qlx.t("bulk.delete_selected");
    deleteBtn.addEventListener("click", function () {
      if (qlx.openDeleteConfirm) qlx.openDeleteConfirm();
    });
    bar.appendChild(deleteBtn);

    var clearBtn = document.createElement("button");
    clearBtn.className = "btn btn-secondary btn-small";
    clearBtn.textContent = qlx.t("bulk.deselect");
    clearBtn.addEventListener("click", qlx.clearSelection);
    bar.appendChild(clearBtn);

    document.body.appendChild(bar);
    return bar;
  }

  function updateActionBar() {
    var bar = getOrCreateActionBar();
    if (selection.size > 0) {
      bar.style.display = "flex";
      var countSpan = bar.querySelector(".action-count");
      if (countSpan) {
        countSpan.textContent = qlx.t("bulk.selected_count") + ": " + selection.size;
      }
    } else {
      bar.style.display = "none";
    }
  }

  // Init on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      qlx.initBulkSelect();
    });
  } else {
    qlx.initBulkSelect();
  }

  // Re-init after HTMX swaps
  document.body.addEventListener("htmx:afterSwap", function (event) {
    if (!event.detail || !event.detail.target) return;
    if (event.detail.target.id !== "content") return;
    qlx.initBulkSelect();
  });
})();

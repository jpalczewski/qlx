
(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {{multi?: boolean, ids?: Array<{id: string, type: string}>, id?: string, type?: string}|null} */
  var dragData = null;

  /** Initialize drag & drop on all draggable elements and drop targets. */
  function initDragDrop() {
    var draggables = document.querySelectorAll("[draggable=true]");
    draggables.forEach(function (el) {
      el.addEventListener("dragstart", onDragStart);
      el.addEventListener("dragend", onDragEnd);
    });

    var dropTargets = document.querySelectorAll("[data-drop-id]");
    dropTargets.forEach(function (el) {
      el.addEventListener("dragover", onDragOver);
      el.addEventListener("dragleave", onDragLeave);
      el.addEventListener("drop", onDrop);
    });
  }

  /** @param {DragEvent} e */
  function onDragStart(e) {
    var li = /** @type {HTMLElement} */ (e.target).closest("[draggable]");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var type = li.getAttribute("data-type");

    // Multi-drag: if dragged item is in selection, drag all selected
    if (id && qlx.selectionHas(id) && qlx.selectionSize() > 1) {
      dragData = {
        multi: true,
        ids: qlx.selectionEntries(),
        type: type
      };
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", JSON.stringify(dragData.ids));

      // Composite drag image with count badge
      var ghost = document.createElement("div");
      ghost.style.position = "absolute";
      ghost.style.top = "-1000px";
      ghost.style.background = "var(--bg-card)";
      ghost.style.border = "1px solid var(--border)";
      ghost.style.borderRadius = "4px";
      ghost.style.padding = "0.5rem 1rem";
      ghost.style.color = "var(--text)";
      ghost.textContent = qlx.selectionSize() + " " + qlx.t("bulk.items_count");
      document.body.appendChild(ghost);
      e.dataTransfer.setDragImage(ghost, 0, 0);
      setTimeout(function () { ghost.remove(); }, 0);
    } else {
      // Single drag
      dragData = { id: id, type: type };
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", id);
    }
    li.classList.add("dragging");
  }

  /** @param {DragEvent} e */
  function onDragEnd(e) {
    var li = /** @type {HTMLElement} */ (e.target).closest("[draggable]");
    if (li) li.classList.remove("dragging");
    document.querySelectorAll(".drag-over").forEach(function (el) {
      el.classList.remove("drag-over");
    });
    dragData = null;
  }

  /** @param {DragEvent} e */
  function onDragOver(e) {
    if (!dragData) return;
    var dropEl = /** @type {HTMLElement} */ (e.target).closest("[data-drop-id]");
    if (!dropEl) return;

    var dropId = dropEl.getAttribute("data-drop-id");

    if (!dragData.multi && dragData.id === dropId) return;
    if (dragData.multi && dragData.ids.indexOf(dropId) !== -1) return;

    if (dragData.type === "item" && dropEl.getAttribute("data-drop-type") !== "container") return;

    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    dropEl.classList.add("drag-over");
  }

  /** @param {DragEvent} e */
  function onDragLeave(e) {
    var dropEl = /** @type {HTMLElement} */ (e.target).closest("[data-drop-id]");
    if (dropEl) dropEl.classList.remove("drag-over");
  }

  /** @param {DragEvent} e */
  function onDrop(e) {
    e.preventDefault();
    var dropEl = /** @type {HTMLElement} */ (e.target).closest("[data-drop-id]");
    if (!dropEl || !dragData) return;
    dropEl.classList.remove("drag-over");

    var targetId = dropEl.getAttribute("data-drop-id");

    // Multi-drop: use bulk move endpoint
    if (dragData.multi) {
      var hasTarget = dragData.ids.some(function (entry) { return entry.id === targetId; });
      if (hasTarget) return;
      var ids = dragData.ids;
      dragData = null;
      fetch("/ui/actions/bulk/move", {
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
          console.error("bulk drop move failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
      return;
    }

    // Single drop
    if (dragData.id === targetId) return;

    var movedType = dragData.type;
    var url, body;
    if (movedType === "container") {
      url = "/ui/actions/containers/" + encodeURIComponent(dragData.id) + "/move";
      body = "parent_id=" + encodeURIComponent(targetId);
    } else {
      url = "/ui/actions/items/" + encodeURIComponent(dragData.id) + "/move";
      body = "container_id=" + encodeURIComponent(targetId);
    }

    dragData = null;

    fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body
    })
      .then(function (resp) {
        if (!resp.ok) {
          return resp.json().then(function (data) {
            qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
          });
        }
        qlx.showToast(movedType === "container" ? qlx.t("inventory.container_moved") : qlx.t("inventory.item_moved"), false);
        htmx.ajax("GET", window.location.pathname, { target: "#content" });
      })
      .catch(function (err) {
        console.error("move failed:", err);
        qlx.showToast(qlx.t("error.connection"), true);
      });
  }

  // Init on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      initDragDrop();
    });
  } else {
    initDragDrop();
  }

  // Re-init after HTMX swaps
  document.body.addEventListener("htmx:afterSwap", function (event) {
    if (!event.detail || !event.detail.target) return;
    if (event.detail.target.id !== "content") return;
    initDragDrop();
  });
})();

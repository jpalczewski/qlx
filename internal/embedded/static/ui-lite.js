(function () {
  // HTMX autofocus after swap
  document.body.addEventListener("htmx:afterSwap", function (event) {
    if (!event.detail || !event.detail.target) return;
    var target = event.detail.target;
    if (target.id !== "content") return;
    var autofocus = target.querySelector("[autofocus]");
    if (autofocus) autofocus.focus();
    initDragDrop();
    initBulkSelect();
  });

  // Toast notifications (exposed globally for template scripts)
  window.showToast = showToast;
  function showToast(message, isError) {
    var container = document.getElementById("toast-container");
    if (!container) {
      container = document.createElement("div");
      container.id = "toast-container";
      document.body.appendChild(container);
    }
    var toast = document.createElement("div");
    toast.className = "toast" + (isError ? " toast-error" : " toast-success");
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(function () {
      toast.classList.add("toast-fade");
      setTimeout(function () { toast.remove(); }, 300);
    }, 3000);
  }

  // ── Selection Module ────────────────────────────────────────────────────────
  var selection = new Map(); // id → type

  function initBulkSelect() {
    document.querySelectorAll(".bulk-select").forEach(function (cb) {
      // Avoid double-binding
      cb.removeEventListener("change", onBulkCheckChange);
      cb.addEventListener("change", onBulkCheckChange);
    });

    // "Zaznacz" toggle button
    var toggleBtn = document.getElementById("select-toggle-btn");
    if (toggleBtn) {
      toggleBtn.removeEventListener("click", onSelectToggle);
      toggleBtn.addEventListener("click", onSelectToggle);
    }
  }

  function onBulkCheckChange(e) {
    var cb = e.target;
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
      clearSelection();
    }
  }

  window.clearSelection = clearSelection;
  function clearSelection() {
    selection.clear();
    document.querySelectorAll(".bulk-select").forEach(function (cb) {
      cb.checked = false;
    });
    var bar = document.getElementById("action-bar");
    if (bar) bar.style.display = "none";
  }

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
    moveBtn.textContent = "Przenieś do...";
    moveBtn.addEventListener("click", openMovePicker);
    bar.appendChild(moveBtn);

    var tagBtn = document.createElement("button");
    tagBtn.className = "btn btn-secondary btn-small";
    tagBtn.textContent = "Taguj...";
    tagBtn.addEventListener("click", openTagPicker);
    bar.appendChild(tagBtn);

    var deleteBtn = document.createElement("button");
    deleteBtn.className = "btn btn-danger btn-small";
    deleteBtn.textContent = "Usuń zaznaczone";
    deleteBtn.addEventListener("click", openDeleteConfirm);
    bar.appendChild(deleteBtn);

    var clearBtn = document.createElement("button");
    clearBtn.className = "btn btn-secondary btn-small";
    clearBtn.textContent = "Odznacz";
    clearBtn.addEventListener("click", clearSelection);
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
        countSpan.textContent = "Zaznaczono: " + selection.size;
      }
    } else {
      bar.style.display = "none";
    }
  }

  // ── Move Picker Dialog ──────────────────────────────────────────────────────
  var movePickerTargetId = null;

  function getOrCreateMovePickerDialog() {
    var dlg = document.getElementById("move-picker");
    if (dlg) return dlg;

    dlg = document.createElement("dialog");
    dlg.id = "move-picker";

    var picker = document.createElement("div");
    picker.className = "tree-picker";

    var title = document.createElement("h3");
    title.textContent = "Przenieś do kontenera";
    picker.appendChild(title);

    var searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.className = "tree-search";
    searchInput.placeholder = "Szukaj...";
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
    cancelBtn.textContent = "Anuluj";
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { dlg.close(); });
    footer.appendChild(cancelBtn);

    var confirmBtn = document.createElement("button");
    confirmBtn.className = "btn btn-primary btn-small";
    confirmBtn.textContent = "Przenieś";
    confirmBtn.type = "button";
    confirmBtn.disabled = true;
    confirmBtn.id = "move-picker-confirm";
    confirmBtn.addEventListener("click", function () {
      if (movePickerTargetId) {
        executeBulkMove(movePickerTargetId);
        dlg.close();
      }
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    // Delegate click events for tree nodes inside move-picker
    treeContainer.addEventListener("click", function (e) {
      var expandEl = e.target.closest(".tree-expand");
      if (expandEl) {
        handleTreeExpand(expandEl, treeContainer);
        return;
      }
      var labelEl = e.target.closest(".tree-label");
      if (labelEl) {
        handleTreeLabelSelect(labelEl, treeContainer, "move-picker-confirm");
      }
    });

    return dlg;
  }

  function openMovePicker() {
    movePickerTargetId = null;
    var dlg = getOrCreateMovePickerDialog();
    var confirmBtn = document.getElementById("move-picker-confirm");
    if (confirmBtn) confirmBtn.disabled = true;

    // Clear previous selection highlight
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
          // Safe parse via DOMParser
          var parser = new DOMParser();
          var doc = parser.parseFromString(html, "text/html");
          while (doc.body.firstChild) {
            treeContainer.appendChild(doc.body.firstChild);
          }
          // Process HTMX on newly inserted content
          if (window.htmx) htmx.process(treeContainer);
        }
      })
      .catch(function (err) {
        console.error("tree load failed:", err);
      });

    dlg.showModal();
  }

  function executeBulkMove(targetID) {
    var ids = Array.from(selection.keys());
    fetch("/ui/actions/bulk/move", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids, target_container_id: targetID })
    })
      .then(function (resp) {
        if (!resp.ok) {
          return resp.json().then(function (data) {
            showToast(data.error || "Błąd " + resp.status, true);
          });
        }
        showToast("Przeniesiono " + ids.length + " elementów", false);
        clearSelection();
        htmx.ajax("GET", window.location.pathname, { target: "#content" });
      })
      .catch(function (err) {
        console.error("bulk move failed:", err);
        showToast("Błąd połączenia", true);
      });
  }

  // ── Tag Picker Dialog ───────────────────────────────────────────────────────
  var tagPickerTargetId = null;

  function getOrCreateTagPickerDialog() {
    var dlg = document.getElementById("tag-picker");
    if (dlg) return dlg;

    dlg = document.createElement("dialog");
    dlg.id = "tag-picker";

    var picker = document.createElement("div");
    picker.className = "tree-picker";

    var title = document.createElement("h3");
    title.textContent = "Dodaj tag";
    picker.appendChild(title);

    var searchInput = document.createElement("input");
    searchInput.type = "text";
    searchInput.className = "tree-search";
    searchInput.placeholder = "Szukaj tagów...";
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
    cancelBtn.textContent = "Anuluj";
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { dlg.close(); });
    footer.appendChild(cancelBtn);

    var confirmBtn = document.createElement("button");
    confirmBtn.className = "btn btn-primary btn-small";
    confirmBtn.textContent = "Taguj";
    confirmBtn.type = "button";
    confirmBtn.disabled = true;
    confirmBtn.id = "tag-picker-confirm";
    confirmBtn.addEventListener("click", function () {
      if (tagPickerTargetId) {
        executeBulkTag(tagPickerTargetId);
        dlg.close();
      }
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    treeContainer.addEventListener("click", function (e) {
      var expandEl = e.target.closest(".tree-expand");
      if (expandEl) {
        handleTreeExpand(expandEl, treeContainer);
        return;
      }
      var labelEl = e.target.closest(".tree-label");
      if (labelEl) {
        handleTreeLabelSelect(labelEl, treeContainer, "tag-picker-confirm");
      }
    });

    return dlg;
  }

  function openTagPicker() {
    tagPickerTargetId = null;
    var dlg = getOrCreateTagPickerDialog();
    var confirmBtn = document.getElementById("tag-picker-confirm");
    if (confirmBtn) confirmBtn.disabled = true;

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
  }

  function executeBulkTag(tagID) {
    var ids = Array.from(selection.keys());
    fetch("/ui/actions/bulk/tags", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids: ids, tag_id: tagID })
    })
      .then(function (resp) {
        if (!resp.ok) {
          return resp.json().then(function (data) {
            showToast(data.error || "Błąd " + resp.status, true);
          });
        }
        showToast("Otagowano " + ids.length + " elementów", false);
        clearSelection();
        htmx.ajax("GET", window.location.pathname, { target: "#content" });
      })
      .catch(function (err) {
        console.error("bulk tag failed:", err);
        showToast("Błąd połączenia", true);
      });
  }

  // ── Bulk Delete ─────────────────────────────────────────────────────────────
  function getOrCreateDeleteDialog() {
    var dlg = document.getElementById("delete-confirm-dialog");
    if (dlg) return dlg;

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
    cancelBtn.textContent = "Anuluj";
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { dlg.close(); });
    footer.appendChild(cancelBtn);

    var confirmBtn = document.createElement("button");
    confirmBtn.className = "btn btn-danger btn-small";
    confirmBtn.textContent = "Usuń";
    confirmBtn.type = "button";
    confirmBtn.addEventListener("click", function () {
      dlg.close();
      executeBulkDelete();
    });
    footer.appendChild(confirmBtn);

    picker.appendChild(footer);
    dlg.appendChild(picker);
    document.body.appendChild(dlg);

    return dlg;
  }

  function openDeleteConfirm() {
    var dlg = getOrCreateDeleteDialog();
    var msg = document.getElementById("delete-confirm-msg");
    if (msg) {
      msg.textContent = "Usunąć " + selection.size + " elementów? Tej operacji nie można cofnąć.";
    }
    dlg.showModal();
  }

  function executeBulkDelete() {
    var ids = Array.from(selection.keys());
    fetch("/ui/actions/bulk/delete", {
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
          showToast("Nie udało się usunąć " + failed.length + " elementów", true);
        } else {
          showToast("Usunięto " + deleted.length + " elementów", false);
        }
        clearSelection();
      })
      .catch(function (err) {
        console.error("bulk delete failed:", err);
        showToast("Błąd połączenia", true);
      });
  }

  // ── Tree helpers for dialogs ────────────────────────────────────────────────
  function handleTreeExpand(expandEl, treeContainer) {
    var li = expandEl.closest(".tree-node");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var ul = li.querySelector("ul.tree-branch");

    if (ul && ul.children.length > 0) {
      // Toggle visibility
      ul.style.display = ul.style.display === "none" ? "" : "none";
      expandEl.textContent = ul.style.display === "none" ? "▶" : "▼";
      return;
    }

    // Fetch children
    var endpoint = treeContainer.id === "tag-tree-container"
      ? "/ui/partials/tag-tree?parent_id=" + encodeURIComponent(id)
      : "/ui/partials/tree?parent_id=" + encodeURIComponent(id);

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
        expandEl.textContent = "▼";
      })
      .catch(function (err) {
        console.error("tree expand failed:", err);
      });
  }

  function handleTreeLabelSelect(labelEl, treeContainer, confirmBtnId) {
    // Clear previous selection in this tree
    treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
      el.classList.remove("selected");
    });
    labelEl.classList.add("selected");

    var li = labelEl.closest(".tree-node");
    var id = li ? li.getAttribute("data-id") : null;

    if (treeContainer.id === "tag-tree-container") {
      tagPickerTargetId = id;
    } else {
      movePickerTargetId = id;
    }

    var confirmBtn = document.getElementById(confirmBtnId);
    if (confirmBtn) confirmBtn.disabled = !id;
  }

  // ── Drag & Drop ─────────────────────────────────────────────────────────────
  var dragData = null;

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

  function onDragStart(e) {
    var li = e.target.closest("[draggable]");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var type = li.getAttribute("data-type");

    // Multi-drag: if dragged item is in selection, drag all selected
    if (id && selection.has(id) && selection.size > 1) {
      dragData = {
        multi: true,
        ids: Array.from(selection.keys()),
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
      ghost.textContent = selection.size + " elementów";
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

  function onDragEnd(e) {
    var li = e.target.closest("[draggable]");
    if (li) li.classList.remove("dragging");
    document.querySelectorAll(".drag-over").forEach(function (el) {
      el.classList.remove("drag-over");
    });
    dragData = null;
  }

  function onDragOver(e) {
    if (!dragData) return;
    var dropEl = e.target.closest("[data-drop-id]");
    if (!dropEl) return;

    var dropId = dropEl.getAttribute("data-drop-id");

    if (!dragData.multi && dragData.id === dropId) return;
    if (dragData.multi && dragData.ids.indexOf(dropId) !== -1) return;

    if (dragData.type === "item" && dropEl.getAttribute("data-drop-type") !== "container") return;

    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    dropEl.classList.add("drag-over");
  }

  function onDragLeave(e) {
    var dropEl = e.target.closest("[data-drop-id]");
    if (dropEl) dropEl.classList.remove("drag-over");
  }

  function onDrop(e) {
    e.preventDefault();
    var dropEl = e.target.closest("[data-drop-id]");
    if (!dropEl || !dragData) return;
    dropEl.classList.remove("drag-over");

    var targetId = dropEl.getAttribute("data-drop-id");

    // Multi-drop: use bulk move endpoint
    if (dragData.multi) {
      if (dragData.ids.indexOf(targetId) !== -1) return;
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
              showToast(data.error || "Błąd " + resp.status, true);
            });
          }
          showToast("Przeniesiono " + ids.length + " elementów", false);
          clearSelection();
          htmx.ajax("GET", window.location.pathname, { target: "#content" });
        })
        .catch(function (err) {
          console.error("bulk drop move failed:", err);
          showToast("Błąd połączenia", true);
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
            showToast(data.error || "Błąd " + resp.status, true);
          });
        }
        showToast(movedType === "container" ? "Kontener przeniesiony" : "Przedmiot przeniesiony", false);
        htmx.ajax("GET", window.location.pathname, { target: "#content" });
      })
      .catch(function (err) {
        console.error("move failed:", err);
        showToast("Błąd połączenia", true);
      });
  }

  // Filter templates by selected printer model
  window.filterTemplates = function(printerSelectId, templateSelectId) {
    var printerSel = document.getElementById(printerSelectId);
    var templateSel = document.getElementById(templateSelectId);
    if (!printerSel || !templateSel) return;

    var selected = printerSel.options[printerSel.selectedIndex];
    var model = selected ? selected.getAttribute("data-model") : "";

    var firstVisible = null;
    var currentHidden = false;

    Array.from(templateSel.options).forEach(function(opt) {
      var target = opt.getAttribute("data-target");
      // Legacy options (no data-target) and universal are always visible
      if (!target || target === "universal" || target === "printer:" + model) {
        opt.hidden = false;
        opt.disabled = false;
        if (!firstVisible) firstVisible = opt;
      } else {
        opt.hidden = true;
        opt.disabled = true;
        if (opt.selected) currentHidden = true;
      }
    });

    // If current selection is now hidden, select first visible
    if (currentHidden && firstVisible) {
      firstVisible.selected = true;
    }
  };

  // Run filter on HTMX swap (when page loads via HTMX)
  document.body.addEventListener("htmx:afterSwap", function() {
    if (document.getElementById("print-printer")) {
      window.filterTemplates("print-printer", "print-template");
    }
    if (document.getElementById("container-print-printer")) {
      window.filterTemplates("container-print-printer", "container-print-template");
    }
  });

  // Init on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      initDragDrop();
      initBulkSelect();
    });
  } else {
    initDragDrop();
    initBulkSelect();
  }

  // Expose bulk-select init globally (for use from HTML)
  window.initBulkSelect = initBulkSelect;

  // SSE for live printer status
  var evtSource = null;

  function initSSE() {
    if (evtSource) return;
    evtSource = new EventSource('/api/printers/events');
    evtSource.onmessage = function(e) {
      try {
        var evt = JSON.parse(e.data);
        updatePrinterCard(evt.printer_id, evt.status);
        updateNavbarPrinter(evt.status);
      } catch(err) {
        console.error('SSE parse error:', err);
      }
    };
    evtSource.onerror = function() {
      // Will auto-reconnect
    };
  }

  function updatePrinterCard(printerId, status) {
    var el = document.getElementById('printer-status-' + printerId);
    if (!el) return;

    // Clear and rebuild using safe DOM methods
    el.textContent = '';

    if (!status.connected) {
      var offline = document.createElement('span');
      offline.className = 'status-error';
      offline.textContent = 'Offline';
      if (status.last_error) {
        offline.textContent += ': ' + status.last_error;
      }
      el.appendChild(offline);
      return;
    }

    var parts = [];
    if (status.battery >= 0) parts.push('Battery: ' + status.battery + '%');
    if (status.label_width_mm > 0 && status.label_height_mm > 0) {
      parts.push('Size: ' + status.label_width_mm + 'x' + status.label_height_mm + 'mm');
    } else if (status.print_width_mm > 0) {
      parts.push(status.print_width_mm + 'mm @ ' + status.dpi + 'dpi');
    }
    if (status.label_type) parts.push('Label: ' + status.label_type);
    if (status.total_labels >= 0) parts.push('Labels: ' + status.used_labels + '/' + status.total_labels);
    parts.push(status.lid_closed ? 'Lid: closed' : 'Lid: OPEN');
    parts.push(status.paper_loaded ? 'Paper: OK' : 'Paper: NONE');

    parts.forEach(function(text, i) {
      var span = document.createElement('span');
      span.textContent = text;
      el.appendChild(span);
      if (i < parts.length - 1) {
        el.appendChild(document.createTextNode(' | '));
      }
    });
  }

  function updateNavbarPrinter(status) {
    var el = document.getElementById('printer-status');
    if (!el) return;
    el.textContent = '';

    if (!status.connected) {
      el.textContent = 'Offline';
      el.className = 'status-error';
      return;
    }

    el.className = 'status-ok';
    var text = '';
    if (status.battery >= 0) text += status.battery + '% ';
    if (!status.lid_closed) text += 'LID! ';
    if (!status.paper_loaded) text += 'NO PAPER ';
    if (!text) text = 'Ready';
    el.textContent = text.trim();
  }

  // Fetch initial printer statuses (SSE only sends updates, not initial state)
  function fetchInitialStatuses() {
    fetch('/api/printers/status')
      .then(function(r) { return r.json(); })
      .then(function(statuses) {
        if (statuses && typeof statuses === 'object') {
          Object.keys(statuses).forEach(function(id) {
            updatePrinterCard(id, statuses[id]);
            updateNavbarPrinter(statuses[id]);
          });
        }
      })
      .catch(function() {});
  }

  // Filter templates on initial page load
  if (document.getElementById("print-printer")) {
    window.filterTemplates("print-printer", "print-template");
  }
  if (document.getElementById("container-print-printer")) {
    window.filterTemplates("container-print-printer", "container-print-template");
  }

  // Start SSE + fetch initial state on load
  initSSE();
  fetchInitialStatuses();

  // Re-fetch after HTMX swaps (navigating to printers page)
  document.body.addEventListener("htmx:afterSwap", function() {
    fetchInitialStatuses();
  });
})();

(function () {
  // HTMX autofocus after swap
  document.body.addEventListener("htmx:afterSwap", function (event) {
    if (!event.detail || !event.detail.target) return;
    var target = event.detail.target;
    if (target.id !== "content") return;
    var autofocus = target.querySelector("[autofocus]");
    if (autofocus) autofocus.focus();
    initDragDrop();
  });

  // Toast notifications
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

  // Drag & Drop
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
    dragData = {
      id: li.getAttribute("data-id"),
      type: li.getAttribute("data-type")
    };
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", dragData.id);
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

    if (dragData.id === dropId) return;

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

  // Init on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initDragDrop);
  } else {
    initDragDrop();
  }
})();

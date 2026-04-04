
(function () {
  var qlx = window.qlx = window.qlx || {};

  // ── Shortcut definitions ────────────────────────────────────────────────
  var shortcuts = [
    { key: "/",      handler: focusSearch,         label: "keyboard.focus_search",    group: "nav",     global: true },
    { key: "k",      ctrl: true, handler: focusSearch, label: "keyboard.focus_search", group: "nav",    global: true },
    { key: "g",      handler: openContainerNav,    label: "keyboard.go_to_container", group: "nav",     global: true },
    { key: "m",      handler: handleMove,          label: "keyboard.move_to_container", group: "action" },
    { key: "i",      handler: focusItemEntry,      label: "keyboard.new_item",        group: "action",  context: "container" },
    { key: "c",      handler: focusContainerEntry, label: "keyboard.new_container",   group: "action",  context: "container" },
    { key: "s",      handler: toggleSelection,     label: "keyboard.selection_mode",  group: "action",  context: "container" },
    { key: "a",      handler: toggleSelectAll,     label: "keyboard.select_all",      group: "action",  context: "selection" },
    { key: "?",      handler: showHelp,            label: "keyboard.help",            group: "general", global: true },
    { key: "Escape", handler: handleEscape,        label: "keyboard.close",           group: "general", global: true, allowInInput: true }
  ];

  // ── Context detection ───────────────────────────────────────────────────
  function getContext() {
    var content = document.getElementById("content");
    if (!content) return "";
    if (qlx.isSelectionMode && qlx.isSelectionMode()) return "selection";
    if (content.querySelector(".container-view")) return "container";
    return "";
  }

  function isInputFocused() {
    var el = document.activeElement;
    if (!el) return false;
    var tag = el.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
    if (el.getAttribute("contenteditable") === "true") return true;
    return false;
  }

  function isDialogOpen() {
    return !!document.querySelector("dialog[open]");
  }

  // ── Dispatcher ──────────────────────────────────────────────────────────
  document.addEventListener("keydown", function (e) {
    var inInput = isInputFocused();
    var dialogOpen = isDialogOpen();

    for (var i = 0; i < shortcuts.length; i++) {
      var s = shortcuts[i];
      if (e.key !== s.key) continue;
      if (s.ctrl && !(e.ctrlKey || e.metaKey)) continue;
      if (!s.ctrl && (e.ctrlKey || e.metaKey) && s.key !== "Escape") continue;

      if (inInput && !s.allowInInput && !s.ctrl) continue;
      if (dialogOpen && s.key !== "Escape") continue;

      if (s.context) {
        var ctx = getContext();
        if (s.context === "container" && ctx !== "container" && ctx !== "selection") continue;
        if (s.context === "selection" && ctx !== "selection") continue;
      }

      e.preventDefault();
      s.handler(e);
      return;
    }

    // List navigation (arrow keys + Enter)
    if (!inInput && !dialogOpen) {
      var listCtx = getContext();
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        if (listCtx === "container" || listCtx === "selection") {
          e.preventDefault();
          navigateList(e.key === "ArrowDown" ? 1 : -1);
        }
      } else if (e.key === "Enter") {
        var active = document.querySelector(".kb-active");
        if (active) {
          e.preventDefault();
          openActiveItem();
        }
      }
    }
  });

  // ── Handlers ────────────────────────────────────────────────────────────
  function focusSearch() {
    var search = document.getElementById("global-search");
    if (search) search.focus();
  }

  function focusItemEntry() {
    var input = document.querySelector(".qe-input");
    if (input) /** @type {HTMLElement} */ (input).focus();
  }

  function focusContainerEntry() {
    var input = document.querySelector(".qe-input");
    if (input) /** @type {HTMLElement} */ (input).focus();
  }

  function toggleSelection() {
    if (qlx.toggleSelectionMode) qlx.toggleSelectionMode();
  }

  function toggleSelectAll() {
    if (qlx.selectAll) qlx.selectAll();
  }

  function handleMove() {
    if (!qlx.openMovePicker) return;

    // Item detail page
    var itemView = document.querySelector(".item-view");
    if (itemView) {
      var editLink = itemView.querySelector("a[href*='/items/'][href*='/edit']");
      if (editLink) {
        var itemId = editLink.getAttribute("href").replace("/items/", "").replace("/edit", "");
        qlx.openMovePicker({ id: itemId, type: "item" });
      }
      return;
    }

    // Container detail page — check for container-detail element (specific container, not root list)
    var containerDetail = document.querySelector(".container-view .container-detail");
    if (containerDetail) {
      // Selection mode with items selected — bulk move
      if (qlx.isSelectionMode && qlx.isSelectionMode() && qlx.selectionSize && qlx.selectionSize() > 0) {
        qlx.openMovePicker();
        return;
      }

      // kb-active element — single move of that element
      var active = document.querySelector(".kb-active");
      if (active) {
        var id = active.getAttribute("data-id");
        var type = active.getAttribute("data-type") || "item";
        if (id) {
          qlx.openMovePicker({ id: id, type: type });
          return;
        }
      }

      // No selection, no kb-active — move the current container itself
      var containerEditLink = containerDetail.querySelector("a[href*='/containers/'][href*='/edit']");
      if (containerEditLink) {
        var containerId = containerEditLink.getAttribute("href").replace("/containers/", "").replace("/edit", "");
        qlx.openMovePicker({ id: containerId, type: "container" });
        return;
      }
    }
  }

  function handleEscape() {
    var openDialog = document.querySelector("dialog[open]");
    if (openDialog) {
      /** @type {HTMLDialogElement} */ (openDialog).close();
      return;
    }
    if (isInputFocused()) {
      /** @type {HTMLElement} */ (document.activeElement).blur();
      return;
    }
    if (qlx.isSelectionMode && qlx.isSelectionMode()) {
      if (qlx.toggleSelectionMode) qlx.toggleSelectionMode();
      return;
    }
    clearListHighlight();
  }

  // ── List Navigation ─────────────────────────────────────────────────────
  var activeListIndex = -1;

  function getNavigableItems() {
    var items = [];
    var containerItems = document.querySelectorAll("#container-list > li:not(.empty-state)");
    var itemItems = document.querySelectorAll("#item-list > li:not(.empty-state)");
    for (var i = 0; i < containerItems.length; i++) items.push(containerItems[i]);
    for (var j = 0; j < itemItems.length; j++) items.push(itemItems[j]);
    return items;
  }

  function navigateList(direction) {
    var items = getNavigableItems();
    if (items.length === 0) return;

    for (var i = 0; i < items.length; i++) items[i].classList.remove("kb-active");

    activeListIndex += direction;
    if (activeListIndex < 0) activeListIndex = 0;
    if (activeListIndex >= items.length) activeListIndex = items.length - 1;

    items[activeListIndex].classList.add("kb-active");
    items[activeListIndex].scrollIntoView({ block: "nearest" });
  }

  function openActiveItem() {
    var active = document.querySelector(".kb-active");
    if (!active) return;
    var link = active.querySelector("a");
    if (link) link.click();
  }

  function clearListHighlight() {
    activeListIndex = -1;
    var highlighted = document.querySelectorAll(".kb-active");
    for (var i = 0; i < highlighted.length; i++) highlighted[i].classList.remove("kb-active");
  }

  document.body.addEventListener("htmx:afterSwap", function (e) {
    if (!e.detail || !e.detail.target) return;
    if (e.detail.target.id !== "content") return;
    clearListHighlight();
  });

  // ── Container Navigator ─────────────────────────────────────────────────
  var navPicker = null;

  function openContainerNav() {
    if (!navPicker && qlx.createTreePicker) {
      navPicker = qlx.createTreePicker({
        id: "container-nav-picker",
        title: function () { return qlx.t("keyboard.go_to_container"); },
        endpoint: "/partials/tree",
        searchEndpoint: "/partials/tree/search",
        searchPlaceholder: function () { return qlx.t("nav.search_placeholder"); },
        confirmLabel: function () { return qlx.t("keyboard.open"); },
        onConfirm: function (targetId) {
          htmx.ajax("GET", "/containers/" + targetId, { target: "#content" });
        }
      });
    }
    if (navPicker) navPicker.open();
  }

  // ── Help Overlay ────────────────────────────────────────────────────────
  var helpDialog = null;

  var helpLayout = [
    {
      group: "keyboard.nav_group",
      items: [
        { keys: ["/", "Ctrl+K"], label: "keyboard.focus_search" },
        { keys: ["g"],           label: "keyboard.go_to_container" },
        { keys: ["\u2191 \u2193"], label: "keyboard.navigate" },
        { keys: ["Enter"],       label: "keyboard.open_selected" }
      ]
    },
    {
      group: "keyboard.action_group",
      items: [
        { keys: ["i"], label: "keyboard.new_item" },
        { keys: ["c"], label: "keyboard.new_container" },
        { keys: ["s"], label: "keyboard.selection_mode" },
        { keys: ["a"], label: "keyboard.select_all" },
        { keys: ["m"], label: "keyboard.move_to_container" }
      ]
    },
    {
      group: "keyboard.general_group",
      items: [
        { keys: ["?"],   label: "keyboard.help" },
        { keys: ["Esc"], label: "keyboard.close" }
      ]
    }
  ];

  function showHelp() {
    if (!helpDialog) {
      helpDialog = document.createElement("dialog");
      helpDialog.id = "keyboard-help";

      var title = document.createElement("h3");
      title.textContent = qlx.t("keyboard.help");
      helpDialog.appendChild(title);

      for (var g = 0; g < helpLayout.length; g++) {
        var group = helpLayout[g];
        var section = document.createElement("div");
        section.className = "kb-help-group";

        var groupTitle = document.createElement("div");
        groupTitle.className = "kb-help-group-title";
        groupTitle.textContent = qlx.t(group.group);
        section.appendChild(groupTitle);

        for (var r = 0; r < group.items.length; r++) {
          var item = group.items[r];
          var row = document.createElement("div");
          row.className = "kb-help-row";

          for (var k = 0; k < item.keys.length; k++) {
            var kbd = document.createElement("kbd");
            kbd.textContent = item.keys[k];
            row.appendChild(kbd);
          }

          var desc = document.createElement("span");
          desc.textContent = qlx.t(item.label);
          row.appendChild(desc);

          section.appendChild(row);
        }

        helpDialog.appendChild(section);
      }

      helpDialog.addEventListener("click", function (e) {
        if (e.target === helpDialog) helpDialog.close();
      });

      document.body.appendChild(helpDialog);
    }
    helpDialog.showModal();
  }
})();

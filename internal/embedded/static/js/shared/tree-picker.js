(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Expand or collapse a tree node, fetching children from the given endpoint.
   * @param {Element} expandEl - the clicked expand toggle
   * @param {HTMLElement} treeContainer - the tree root container
   * @param {string} endpoint - base URL for fetching children (e.g. "/partials/tree")
   */
  function handleTreeExpand(expandEl, treeContainer, endpoint) {
    var li = expandEl.closest(".tree-node");
    if (!li) return;
    var id = li.getAttribute("data-id");
    var ul = li.querySelector("ul.tree-branch");

    if (ul && ul.children.length > 0) {
      var collapsed = ul.style.display === "none";
      ul.style.display = collapsed ? "" : "none";
      if (collapsed) {
        li.classList.add("expanded");
      } else {
        li.classList.remove("expanded");
      }
      return;
    }

    var url = endpoint + "?parent_id=" + encodeURIComponent(id);

    fetch(url)
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
        li.classList.add("expanded");
      })
      .catch(function (err) {
        console.error("tree expand failed:", err);
      });
  }

  /**
   * Create a tree picker dialog.
   * @param {Object} config
   * @param {string} config.id - dialog element id
   * @param {string|function(): string} config.title - dialog heading (string or getter fn for deferred i18n)
   * @param {string} config.endpoint - tree data endpoint
   * @param {string} config.searchEndpoint - search endpoint for hx-get
   * @param {string|function(): string} config.searchPlaceholder - placeholder text (string or getter fn)
   * @param {string|function(): string} config.confirmLabel - confirm button text (string or getter fn)
   * @param {function(string): void} config.onConfirm - called with selected node data-id
   * @returns {{ open: function(): void }}
   */
  qlx.createTreePicker = function createTreePicker(config) {
    /** @type {string|null} */
    var selectedId = null;
    var treeContainerId = config.id + "-tree-container";
    var confirmBtnId = config.id + "-confirm";

    /** Resolve a config value that may be a string or a getter function. */
    function resolve(val) {
      return typeof val === "function" ? val() : val;
    }

    function getOrCreateDialog() {
      var dlg = document.getElementById(config.id);
      if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

      dlg = document.createElement("dialog");
      dlg.id = config.id;

      var picker = document.createElement("div");
      picker.className = "tree-picker";

      var title = document.createElement("h3");
      title.textContent = resolve(config.title);
      picker.appendChild(title);

      var searchInput = document.createElement("input");
      searchInput.type = "text";
      searchInput.className = "tree-search";
      searchInput.placeholder = resolve(config.searchPlaceholder);
      searchInput.setAttribute("name", "q");
      searchInput.setAttribute("hx-get", config.searchEndpoint);
      searchInput.setAttribute("hx-trigger", "input[this.value.length>0] delay:300ms");
      searchInput.setAttribute("hx-target", "#" + treeContainerId);
      picker.appendChild(searchInput);

      searchInput.addEventListener("input", function () {
        if (searchInput.value !== "") return;
        var tc = document.getElementById(treeContainerId);
        if (!tc) return;
        tc.textContent = "";
        fetch(config.endpoint + "?parent_id=")
          .then(function (r) { return r.text(); })
          .then(function (html) {
            var parser = new DOMParser();
            var doc = parser.parseFromString(html, "text/html");
            while (doc.body.firstChild) tc.appendChild(doc.body.firstChild);
            if (window.htmx) htmx.process(tc);
          })
          .catch(function (err) { console.error("tree reload failed:", err); });
      });

      var treeContainer = document.createElement("div");
      treeContainer.id = treeContainerId;
      treeContainer.style.flex = "1";
      treeContainer.style.overflowY = "auto";
      picker.appendChild(treeContainer);

      var footer = document.createElement("div");
      footer.className = "tree-picker-footer";

      var cancelBtn = document.createElement("button");
      cancelBtn.className = "btn btn-secondary btn-small";
      cancelBtn.textContent = qlx.t("action.cancel");
      cancelBtn.type = "button";
      cancelBtn.addEventListener("click", function () {
        /** @type {HTMLDialogElement} */ (dlg).close();
      });
      footer.appendChild(cancelBtn);

      var confirmBtn = document.createElement("button");
      confirmBtn.className = "btn btn-primary btn-small";
      confirmBtn.textContent = resolve(config.confirmLabel);
      confirmBtn.type = "button";
      confirmBtn.disabled = true;
      confirmBtn.id = confirmBtnId;
      confirmBtn.addEventListener("click", function () {
        if (selectedId) {
          config.onConfirm(selectedId);
          /** @type {HTMLDialogElement} */ (dlg).close();
        }
      });
      footer.appendChild(confirmBtn);

      picker.appendChild(footer);
      dlg.appendChild(picker);
      document.body.appendChild(dlg);
      if (window.htmx) htmx.process(searchInput);

      // Delegate click events for tree nodes
      treeContainer.addEventListener("click", function (e) {
        var expandEl = /** @type {HTMLElement} */ (e.target).closest(".tree-expand");
        if (expandEl) {
          handleTreeExpand(expandEl, treeContainer, config.endpoint);
          return;
        }
        var labelEl = /** @type {HTMLElement} */ (e.target).closest(".tree-label");
        if (labelEl) {
          treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
            el.classList.remove("selected");
          });
          labelEl.classList.add("selected");

          var li = labelEl.closest(".tree-node");
          selectedId = li ? li.getAttribute("data-id") : null;

          var btn = document.getElementById(confirmBtnId);
          if (btn) /** @type {HTMLButtonElement} */ (btn).disabled = !selectedId;
        }
      });

      return /** @type {HTMLDialogElement} */ (dlg);
    }

    return {
      open: function () {
        selectedId = null;
        var dlg = getOrCreateDialog();
        var searchInput = dlg.querySelector(".tree-search");
        if (searchInput) searchInput.value = "";
        var confirmBtn = document.getElementById(confirmBtnId);
        if (confirmBtn) /** @type {HTMLButtonElement} */ (confirmBtn).disabled = true;

        var treeContainer = document.getElementById(treeContainerId);
        if (treeContainer) {
          treeContainer.querySelectorAll(".tree-label.selected").forEach(function (el) {
            el.classList.remove("selected");
          });
          treeContainer.textContent = "";
        }

        // Load root tree
        fetch(config.endpoint + "?parent_id=")
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
      }
    };
  };
})();

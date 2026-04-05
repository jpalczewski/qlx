(function () {
  var qlx = window.qlx = window.qlx || {};
  var cache = null;

  function invalidateContainerCache() { cache = null; }

  function fetchContainers() {
    if (cache) return Promise.resolve(cache);
    return fetch("/api/containers/flat", { headers: { "Accept": "application/json" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      })
      .then(function (containers) {
        cache = containers || [];
        return cache;
      })
      .catch(function () {
        return [];
      });
  }

  function filterContainers(containers, query) {
    var q = query.toLowerCase();
    return containers.filter(function (c) {
      return c.name.toLowerCase().indexOf(q) !== -1 ||
        (c.path && c.path.toLowerCase().indexOf(q) !== -1);
    }).slice(0, 8);
  }

  function positionDropdown(dropdown, anchor) {
    var rect = anchor.getBoundingClientRect();
    var spaceBelow = window.innerHeight - rect.bottom;
    dropdown.classList.remove("container-ac-dropdown--above");
    if (spaceBelow < 200 && rect.top > spaceBelow) {
      dropdown.classList.add("container-ac-dropdown--above");
    }
  }

  function buildOption(container, index, onPick) {
    var opt = document.createElement("div");
    opt.className = "container-ac-option";
    opt.setAttribute("role", "option");
    opt.setAttribute("data-index", String(index));
    opt.setAttribute("data-id", container.id);
    opt.id = "container-ac-opt-" + container.id;

    var iconEl = document.createElement("span");
    iconEl.className = "container-ac-icon";
    if (container.icon) {
      var i = document.createElement("i");
      i.className = "ph ph-" + container.icon;
      iconEl.appendChild(i);
    } else {
      iconEl.textContent = "\uD83D\uDCE6";
    }
    opt.appendChild(iconEl);

    var nameEl = document.createElement("span");
    nameEl.className = "container-ac-name";
    nameEl.textContent = container.name;
    opt.appendChild(nameEl);

    if (container.path) {
      var pathEl = document.createElement("span");
      pathEl.className = "container-ac-path";
      pathEl.textContent = container.path;
      opt.appendChild(pathEl);
    }

    opt.addEventListener("mousedown", function (e) {
      e.preventDefault();
      onPick(container);
    });
    return opt;
  }

  function renderDropdown(containers, onPick) {
    var div = document.createElement("div");
    div.className = "container-ac-dropdown";
    div.setAttribute("role", "listbox");

    containers.forEach(function (container, i) {
      div.appendChild(buildOption(container, i, onPick));
    });

    return div;
  }

  // ContainerAutocomplete is designed for EXTERNAL driving:
  // the caller manages an input element and calls update(query) on input events
  // and onKeydown(e) on keydown events. No internal event listeners are attached.
  qlx.ContainerAutocomplete = function (opts) {
    var anchor = opts.anchor;
    var onSelect = opts.onSelect || function () {};
    var onCancel = opts.onCancel || function () {};
    var dropdown = null;
    var activeIndex = -1;

    function close() {
      if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
      dropdown = null;
      activeIndex = -1;
    }

    function isOpen() {
      return dropdown !== null;
    }

    function highlightOption(options) {
      options.forEach(function (o, i) {
        if (i === activeIndex) {
          o.classList.add("active");
          o.setAttribute("aria-selected", "true");
        } else {
          o.classList.remove("active");
          o.setAttribute("aria-selected", "false");
        }
      });
    }

    function onKeydown(e) {
      if (!dropdown) return false;
      var options = dropdown.querySelectorAll("[role=option]");
      if (e.key === "ArrowDown") {
        e.preventDefault();
        activeIndex = Math.min(activeIndex + 1, options.length - 1);
        highlightOption(options);
        return true;
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        highlightOption(options);
        return true;
      } else if (e.key === "Enter" && activeIndex >= 0) {
        e.preventDefault();
        options[activeIndex].dispatchEvent(new MouseEvent("mousedown"));
        return true;
      } else if (e.key === "Escape") {
        e.preventDefault();
        close();
        onCancel();
        return true;
      }
      return false;
    }

    function update(query) {
      var currentQuery = (query || "").trim();
      fetchContainers().then(function (containers) {
        var results = filterContainers(containers, currentQuery);
        if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
        activeIndex = -1;

        if (results.length === 0) {
          dropdown = null;
          return;
        }

        dropdown = renderDropdown(results, function (container) {
          close();
          onSelect(container);
        });
        positionDropdown(dropdown, anchor);
        anchor.parentNode.style.position = "relative";
        anchor.parentNode.appendChild(dropdown);
      });
    }

    return { update: update, onKeydown: onKeydown, close: close, isOpen: isOpen };
  };

  qlx.invalidateContainerCache = invalidateContainerCache;
})();

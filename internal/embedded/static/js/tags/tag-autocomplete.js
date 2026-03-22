(function () {
  var qlx = window.qlx = window.qlx || {};
  var cache = null;
  var debounceTimer = null;

  function invalidateCache() { cache = null; tagById = {}; }

  // id → tag lookup, rebuilt whenever cache is refreshed
  var tagById = {};

  function fetchTags() {
    if (cache) return Promise.resolve(cache);
    return fetch("/tags", { headers: { "Accept": "application/json" } })
      .then(function (r) { return r.json(); })
      .then(function (tags) {
        cache = tags || [];
        tagById = {};
        cache.forEach(function (t) { tagById[t.id] = t; });
        return cache;
      });
  }

  function filterTags(tags, query) {
    var q = query.toLowerCase();
    var exact = false;
    var results = tags.filter(function (t) {
      if (t.name.toLowerCase() === q) exact = true;
      return t.name.toLowerCase().indexOf(q) !== -1;
    });
    return { results: results.slice(0, 8), exactMatch: exact };
  }

  function createTag(name) {
    return fetch("/tags", {
      method: "POST",
      headers: { "Content-Type": "application/json", "Accept": "application/json" },
      body: JSON.stringify({ name: name, color: "", icon: "" })
    }).then(function (r) {
      if (!r.ok) return r.json().then(function (d) { throw new Error(d.error || "Error"); });
      invalidateCache();
      return r.json();
    });
  }

  function positionDropdown(dropdown, anchor) {
    var rect = anchor.getBoundingClientRect();
    var spaceBelow = window.innerHeight - rect.bottom;
    dropdown.classList.remove("above", "below");
    if (spaceBelow < 200 && rect.top > spaceBelow) {
      dropdown.classList.add("above");
    } else {
      dropdown.classList.add("below");
    }
  }

  // Map palette names to hex codes (subset from palette.Colors).
  // Kept in sync manually — only used for color dots in autocomplete dropdown.
  // Palette name → hex (from internal/shared/palette/colors.go)
  var paletteHex = {
    red: "#e94560", orange: "#f4845f", amber: "#f5a623", yellow: "#ffc93c",
    green: "#4ecca3", teal: "#2ec4b6", blue: "#4d9de0", indigo: "#7b6cf6",
    purple: "#b07cd8", pink: "#e84393"
  };

  function buildOption(tag, index, onPick) {
    var opt = document.createElement("div");
    opt.className = "tag-ac-option";
    opt.setAttribute("role", "option");
    opt.setAttribute("data-index", String(index));
    opt.setAttribute("data-id", tag.id);
    opt.id = "tag-ac-opt-" + tag.id;

    var dot = document.createElement("span");
    dot.className = "color-dot";
    if (tag.color) dot.style.backgroundColor = paletteHex[tag.color] || tag.color;
    opt.appendChild(dot);

    var nameSpan = document.createElement("span");
    if (tag.parent_id && tagById[tag.parent_id]) {
      var parentSpan = document.createElement("span");
      parentSpan.className = "tag-ac-parent";
      parentSpan.textContent = tagById[tag.parent_id].name + " / ";
      nameSpan.appendChild(parentSpan);
    }
    var tagNameNode = document.createTextNode(tag.name);
    nameSpan.appendChild(tagNameNode);
    opt.appendChild(nameSpan);

    opt.addEventListener("mousedown", function (e) {
      e.preventDefault();
      onPick(tag);
    });
    return opt;
  }

  function buildCreateOption(query, index, onCreate) {
    var opt = document.createElement("div");
    opt.className = "tag-ac-option create";
    opt.setAttribute("role", "option");
    opt.setAttribute("data-index", String(index));
    opt.id = "tag-ac-opt-create";

    var label = document.createElement("span");
    var promptText = qlx.t ? qlx.t("tags.create_tag_prompt") : 'Create "{0}"?';
    label.textContent = promptText.replace("{0}", query);
    opt.appendChild(label);

    opt.addEventListener("mousedown", function (e) {
      e.preventDefault();
      onCreate(query);
    });
    return opt;
  }

  function renderDropdown(tags, query, onPick, onCreate) {
    var div = document.createElement("div");
    div.className = "tag-ac-dropdown below";
    div.setAttribute("role", "listbox");

    tags.forEach(function (tag, i) {
      div.appendChild(buildOption(tag, i, onPick));
    });

    if (onCreate && query.length > 0) {
      div.appendChild(buildCreateOption(query, tags.length, onCreate));
    }

    return div;
  }

  qlx.TagAutocomplete = function (opts) {
    var anchor = opts.anchor;
    var onSelect = opts.onSelect || function () {};
    var onCancel = opts.onCancel || function () {};
    var dropdown = null;
    var activeIndex = -1;
    var input = null;

    function open(inputEl) {
      input = inputEl;
      update(input.value);
      input.addEventListener("input", onInput);
      input.addEventListener("keydown", onKeydown);
      input.addEventListener("blur", onBlur);
    }

    function close() {
      if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
      dropdown = null;
      activeIndex = -1;
      if (input) {
        input.removeEventListener("input", onInput);
        input.removeEventListener("keydown", onKeydown);
        input.removeEventListener("blur", onBlur);
        input = null;
      }
    }

    function onBlur() {
      setTimeout(function () { close(); onCancel(); }, 150);
    }

    function onInput() {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(function () { update(input.value); }, 150);
    }

    function onKeydown(e) {
      if (!dropdown) return;
      var options = dropdown.querySelectorAll("[role=option]");
      if (e.key === "ArrowDown") {
        e.preventDefault();
        activeIndex = Math.min(activeIndex + 1, options.length - 1);
        highlightOption(options);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        highlightOption(options);
      } else if (e.key === "Enter" && activeIndex >= 0) {
        e.preventDefault();
        options[activeIndex].dispatchEvent(new MouseEvent("mousedown"));
      } else if (e.key === "Escape") {
        e.preventDefault();
        close();
        onCancel();
      }
    }

    function highlightOption(options) {
      options.forEach(function (o, i) {
        o.setAttribute("aria-selected", i === activeIndex ? "true" : "false");
      });
      if (input && activeIndex >= 0) {
        input.setAttribute("aria-activedescendant", options[activeIndex].id || "");
      }
    }

    function update(query) {
      var currentQuery = query.trim();
      fetchTags().then(function (tags) {
        var filtered = filterTags(tags, currentQuery);
        if (dropdown && dropdown.parentNode) dropdown.parentNode.removeChild(dropdown);
        activeIndex = -1;
        var showCreate = !filtered.exactMatch && currentQuery.length > 0;
        dropdown = renderDropdown(
          filtered.results,
          currentQuery,
          function (tag) { close(); onSelect(tag); },
          showCreate ? function (name) {
            createTag(name).then(function (tag) {
              close();
              onSelect(tag);
            }).catch(function (err) {
              showError(err.message);
            });
          } : null
        );
        positionDropdown(dropdown, anchor);
        anchor.parentNode.style.position = "relative";
        anchor.parentNode.appendChild(dropdown);
      });
    }

    function showError(msg) {
      if (!dropdown) return;
      var existing = dropdown.querySelector(".tag-ac-error");
      if (existing) existing.parentNode.removeChild(existing);
      var err = document.createElement("div");
      err.className = "tag-ac-error";
      err.textContent = msg;
      dropdown.appendChild(err);
    }

    return { open: open, close: close };
  };

  qlx.invalidateTagCache = invalidateCache;
})();

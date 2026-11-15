(function () {
  var qlx = window.qlx = window.qlx || {};

  // The "+" button is rendered by a partial without access to translations,
  // so its tooltip and accessible name are set lazily on first hover/focus.
  function labelTagAddButton(e) {
    var btn = e.target.closest && e.target.closest(".tag-add");
    if (!btn || btn.title || !qlx.t) return;
    var label = qlx.t("tags.add_tag");
    btn.title = label;
    btn.setAttribute("aria-label", label);
  }

  function initInlineTagAdd() {
    document.addEventListener("mouseover", labelTagAddButton);
    document.addEventListener("focusin", labelTagAddButton);
    document.addEventListener("click", function (e) {
      var btn = e.target.closest(".tag-add");
      if (!btn) return;

      var objectId = btn.getAttribute("data-object-id");
      var objectType = btn.getAttribute("data-object-type");
      var chipsDiv = btn.closest(".tag-chips");

      // Replace button with a positioned wrapper + input so the dropdown
      // is absolutely positioned relative to the input, not the chips div.
      var input = document.createElement("input");
      input.type = "text";
      input.className = "tag-ac-input";
      input.placeholder = qlx.t ? qlx.t("tags.search_tags") : "Tag...";
      var wrap = document.createElement("div");
      wrap.className = "tag-ac-wrap";
      wrap.appendChild(input);
      btn.style.display = "none";
      chipsDiv.appendChild(wrap);
      input.focus();

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          cleanup();
          htmx.ajax("POST", "/" + objectType + "s/" + objectId + "/tags", {
            target: "#tag-chips-" + objectId,
            swap: "outerHTML",
            values: { tag_id: tag.id }
          });
        },
        onCancel: function () {
          cleanup();
        }
      });

      function cleanup() {
        if (wrap.parentNode) wrap.parentNode.removeChild(wrap);
        btn.style.display = "";
      }

      ac.open(input);
    });
  }

  // Init on load — uses event delegation so works for dynamically added chips
  document.addEventListener("DOMContentLoaded", initInlineTagAdd);
})();

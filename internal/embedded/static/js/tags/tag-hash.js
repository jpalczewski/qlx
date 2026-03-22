(function () {
  var qlx = window.qlx = window.qlx || {};
  var pendingTagId = null;
  var activeHashAC = null; // guard against re-entrancy

  function initHashTagging() {
    document.addEventListener("input", function (e) {
      var input = e.target;
      if (!input.matches || !input.matches(".quick-entry input[name=name]")) return;

      var val = input.value;
      var hashIdx = val.indexOf("#");
      if (hashIdx === -1) return;

      var query = val.substring(hashIdx + 1);
      if (query.indexOf(" ") !== -1) return; // stop at space after #
      if (query.length === 0) return;

      // Prevent multiple simultaneous instances
      if (activeHashAC) return;

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          // Strip #query from the name
          var before = input.value.substring(0, hashIdx);
          var afterHash = input.value.substring(hashIdx);
          var spaceIdx = afterHash.indexOf(" ", 1);
          var after = spaceIdx !== -1 ? afterHash.substring(spaceIdx) : "";
          input.value = (before + after).trim();

          activeHashAC = null; // clear guard
          pendingTagId = tag.id;

          // Determine object type from the form
          var form = input.closest(".quick-entry");
          var isItem = !!form.querySelector("input[name=container_id]");
          var objectType = isItem ? "item" : "container";
          var targetSelector = form.getAttribute("hx-target");

          // Register one-shot afterSwap listener
          var target = document.querySelector(targetSelector);
          if (target) {
            var handler = function () {
              target.removeEventListener("htmx:afterSwap", handler);
              if (!pendingTagId) return;

              var allEls = target.querySelectorAll("li[data-id]");
              var newEl = allEls.length > 0 ? allEls[allEls.length - 1] : null;
              if (!newEl) { pendingTagId = null; return; }

              var newId = newEl.getAttribute("data-id");
              fetch("/ui/actions/" + objectType + "s/" + newId + "/tags", {
                method: "POST",
                headers: { "Content-Type": "application/x-www-form-urlencoded" },
                body: "tag_id=" + encodeURIComponent(pendingTagId)
              }).then(function (resp) {
                if (resp.ok) {
                  // Refresh the page to show the tag chip
                  htmx.ajax("GET", window.location.pathname, { target: "#content" });
                }
              });
              pendingTagId = null;
            };
            target.addEventListener("htmx:afterSwap", handler);
          }
        },
        onCancel: function () {
          activeHashAC = null; // clear guard
          // Leave the # text as-is if user cancels
        }
      });

      activeHashAC = ac;
      ac.open(input);
    });
  }

  document.addEventListener("DOMContentLoaded", initHashTagging);
})();

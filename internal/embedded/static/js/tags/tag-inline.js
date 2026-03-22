(function () {
  var qlx = window.qlx = window.qlx || {};

  function initInlineTagAdd() {
    document.addEventListener("click", function (e) {
      var btn = e.target.closest(".tag-add");
      if (!btn) return;

      var objectId = btn.getAttribute("data-object-id");
      var objectType = btn.getAttribute("data-object-type");
      var chipsDiv = btn.closest(".tag-chips");

      // Replace button with input
      var input = document.createElement("input");
      input.type = "text";
      input.className = "tag-ac-input";
      input.placeholder = qlx.t ? qlx.t("tags.search_tags") : "Tag...";
      btn.style.display = "none";
      chipsDiv.appendChild(input);
      input.focus();

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          // POST assign tag — response is the tag-chips partial HTML
          fetch("/ui/actions/" + objectType + "s/" + objectId + "/tags", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "tag_id=" + encodeURIComponent(tag.id)
          }).then(function (resp) {
            if (resp.ok) {
              // Refresh chips via HTMX
              htmx.ajax("GET", window.location.pathname, { target: "#content" });
            }
          });
          cleanup();
        },
        onCancel: function () {
          cleanup();
        }
      });

      function cleanup() {
        if (input.parentNode) input.parentNode.removeChild(input);
        btn.style.display = "";
      }

      ac.open(input);
    });
  }

  // Init on load — uses event delegation so works for dynamically added chips
  document.addEventListener("DOMContentLoaded", initInlineTagAdd);
})();

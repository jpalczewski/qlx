(function () {
  var qlx = window.qlx = window.qlx || {};

  function initFieldTagging() {
    document.addEventListener("focus", function (e) {
      var input = e.target;
      if (!input.matches || !input.matches(".tag-field-input")) return;

      var objectId = input.getAttribute("data-object-id");
      var objectType = input.getAttribute("data-object-type");

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          input.value = "";
          // POST assign tag
          fetch("/ui/actions/" + objectType + "s/" + objectId + "/tags", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "tag_id=" + encodeURIComponent(tag.id)
          }).then(function (resp) {
            if (resp.ok) {
              // Refresh the whole page to update chips
              htmx.ajax("GET", window.location.pathname, { target: "#content" });
            }
          });
        },
        onCancel: function () {
          input.value = "";
        }
      });

      ac.open(input);
    }, true); // capture phase for focus
  }

  document.addEventListener("DOMContentLoaded", initFieldTagging);
})();

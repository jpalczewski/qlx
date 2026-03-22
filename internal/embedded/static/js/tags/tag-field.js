(function () {
  var qlx = window.qlx = window.qlx || {};
  var activeFieldAC = null;

  function initFieldTagging() {
    document.addEventListener("focus", function (e) {
      var input = e.target;
      if (!input.matches || !input.matches(".tag-field-input")) return;

      if (activeFieldAC) return;

      var objectId = input.getAttribute("data-object-id");
      var objectType = input.getAttribute("data-object-type");

      var ac = qlx.TagAutocomplete({
        anchor: input,
        onSelect: function (tag) {
          activeFieldAC = null;
          input.value = "";
          // POST assign tag
          fetch("/" + objectType + "s/" + objectId + "/tags", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: "tag_id=" + encodeURIComponent(tag.id)
          }).then(function (resp) {
            if (resp.ok) {
              var returnUrl = "/" + objectType + "s/" + objectId;
              htmx.ajax("GET", returnUrl, { target: "#content" });
            }
          });
        },
        onCancel: function () {
          activeFieldAC = null;
          input.value = "";
        }
      });

      activeFieldAC = ac;
      ac.open(input);
    }, true); // capture phase for focus
  }

  document.addEventListener("DOMContentLoaded", initFieldTagging);
})();

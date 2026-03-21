
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Filter template options by the selected printer's model.
   * @param {string} printerSelectId
   * @param {string} templateSelectId
   */
  qlx.filterTemplates = function filterTemplates(printerSelectId, templateSelectId) {
    var printerSel = /** @type {HTMLSelectElement|null} */ (document.getElementById(printerSelectId));
    var templateSel = /** @type {HTMLSelectElement|null} */ (document.getElementById(templateSelectId));
    if (!printerSel || !templateSel) return;

    var selected = printerSel.options[printerSel.selectedIndex];
    var model = selected ? selected.getAttribute("data-model") : "";

    /** @type {HTMLOptionElement|null} */
    var firstVisible = null;
    var currentHidden = false;

    Array.from(templateSel.options).forEach(function (opt) {
      var target = opt.getAttribute("data-target");
      // Legacy options (no data-target) and universal are always visible
      if (!target || target === "universal" || target === "printer:" + model) {
        opt.hidden = false;
        opt.disabled = false;
        if (!firstVisible) firstVisible = opt;
      } else {
        opt.hidden = true;
        opt.disabled = true;
        if (opt.selected) currentHidden = true;
      }
    });

    // If current selection is now hidden, select first visible
    if (currentHidden && firstVisible) {
      firstVisible.selected = true;
    }
  };

  // Backward compatibility
  window.filterTemplates = qlx.filterTemplates;

  // Run filter on HTMX swap (when page loads via HTMX)
  document.body.addEventListener("htmx:afterSwap", function () {
    if (document.getElementById("print-printer")) {
      qlx.filterTemplates("print-printer", "print-template");
    }
    if (document.getElementById("container-print-printer")) {
      qlx.filterTemplates("container-print-printer", "container-print-template");
    }
  });

  // Filter templates on initial page load
  if (document.getElementById("print-printer")) {
    qlx.filterTemplates("print-printer", "print-template");
  }
  if (document.getElementById("container-print-printer")) {
    qlx.filterTemplates("container-print-printer", "container-print-template");
  }
})();

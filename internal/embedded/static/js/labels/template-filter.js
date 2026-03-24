
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Filter template options by the selected printer's model.
   * Supports both legacy ID-based selectors and data-attribute selectors.
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

  // Note: Template filtering for data-attribute-based print forms
  // is handled by print-form.js. This file provides the legacy
  // ID-based API for backward compatibility.
})();

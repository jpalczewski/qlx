
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Activate a tab by its tab-id. Finds the nearest .tab-bar ancestor
   * and switches both the .tab-btn and .tab-panel within the same
   * .tab-container parent.
   *
   * @param {HTMLElement} btn  The clicked .tab-btn element.
   */
  function activateTab(btn) {
    var tabID = btn.dataset.tab;
    if (!tabID) return;

    var container = btn.closest(".tab-container");
    if (!container) return;

    // Deactivate all buttons in this container
    var allBtns = container.querySelectorAll(".tab-btn");
    for (var i = 0; i < allBtns.length; i++) {
      allBtns[i].classList.remove("active");
      allBtns[i].setAttribute("aria-selected", "false");
    }

    // Deactivate all panels in this container
    var allPanels = container.querySelectorAll(".tab-panel");
    for (var j = 0; j < allPanels.length; j++) {
      allPanels[j].classList.remove("active");
    }

    // Activate clicked button
    btn.classList.add("active");
    btn.setAttribute("aria-selected", "true");

    // Activate matching panel
    var panel = container.querySelector(".tab-panel[data-tab=\"" + tabID + "\"]");
    if (panel) {
      panel.classList.add("active");
      // Re-initialise HTMX on newly visible content so any hx-* attributes work
      if (typeof htmx !== "undefined") {
        htmx.process(panel);
      }
    }
  }

  // Delegated click handler — works for statically rendered and HTMX-swapped tabs
  document.addEventListener("click", function (e) {
    var btn = e.target.closest(".tab-btn");
    if (!btn) return;
    activateTab(btn);
  });

  // Keyboard navigation (left/right arrow keys within a tab bar)
  document.addEventListener("keydown", function (e) {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    var btn = e.target.closest(".tab-btn");
    if (!btn) return;

    var bar = btn.closest(".tab-bar");
    if (!bar) return;

    var btns = Array.prototype.slice.call(bar.querySelectorAll(".tab-btn"));
    var idx = btns.indexOf(btn);
    if (idx === -1) return;

    var next = e.key === "ArrowRight"
      ? btns[(idx + 1) % btns.length]
      : btns[(idx - 1 + btns.length) % btns.length];

    if (next) {
      next.focus();
      activateTab(next);
    }
    e.preventDefault();
  });

  /** Public helper to programmatically switch to a tab. */
  qlx.activateTab = activateTab;
})();

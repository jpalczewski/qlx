
(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Show a toast notification.
   * @param {string} message
   * @param {boolean} [isError]
   */
  qlx.showToast = function showToast(message, isError) {
    var container = document.getElementById("toast-container");
    if (!container) {
      container = document.createElement("div");
      container.id = "toast-container";
      document.body.appendChild(container);
    }
    var toast = document.createElement("div");
    toast.className = "toast" + (isError ? " toast-error" : " toast-success");
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(function () {
      toast.classList.add("toast-fade");
      setTimeout(function () { toast.remove(); }, 300);
    }, 3000);
  };

  // Keep backward compatibility with existing template scripts
  window.showToast = qlx.showToast;
})();


(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {EventSource|null} */
  var evtSource = null;

  /** Open an SSE connection to receive live printer status updates. */
  function initSSE() {
    if (evtSource) return;
    evtSource = new EventSource("/api/printers/events");
    evtSource.onmessage = function (e) {
      try {
        var evt = JSON.parse(e.data);
        updatePrinterCard(evt.printer_id, evt.status);
        updateNavbarPrinter(evt.status);
      } catch (err) {
        console.error("SSE parse error:", err);
      }
    };
    evtSource.onerror = function () {
      // Will auto-reconnect
    };
  }

  /**
   * Update a printer detail card with the latest status.
   * @param {string} printerId
   * @param {Record<string, any>} status
   */
  function updatePrinterCard(printerId, status) {
    var el = document.getElementById("printer-status-" + printerId);
    if (!el) return;

    el.textContent = "";

    if (!status.connected) {
      var offline = document.createElement("span");
      offline.className = "status-error";
      offline.textContent = "Offline";
      if (status.last_error) {
        offline.textContent += ": " + status.last_error;
      }
      el.appendChild(offline);
      return;
    }

    var parts = [];
    if (status.battery >= 0) parts.push("Battery: " + status.battery + "%");
    if (status.label_width_mm > 0 && status.label_height_mm > 0) {
      parts.push("Size: " + status.label_width_mm + "x" + status.label_height_mm + "mm");
    } else if (status.print_width_mm > 0) {
      parts.push(status.print_width_mm + "mm @ " + status.dpi + "dpi");
    }
    if (status.label_type) parts.push("Label: " + status.label_type);
    if (status.total_labels >= 0) parts.push("Labels: " + status.used_labels + "/" + status.total_labels);
    parts.push(status.lid_closed ? "Lid: closed" : "Lid: OPEN");
    parts.push(status.paper_loaded ? "Paper: OK" : "Paper: NONE");

    parts.forEach(function (text, i) {
      var span = document.createElement("span");
      span.textContent = text;
      el.appendChild(span);
      if (i < parts.length - 1) {
        el.appendChild(document.createTextNode(" | "));
      }
    });
  }

  /**
   * Update the navbar printer status badge.
   * @param {Record<string, any>} status
   */
  function updateNavbarPrinter(status) {
    var el = document.getElementById("printer-status");
    if (!el) return;
    el.textContent = "";

    if (!status.connected) {
      el.textContent = "Offline";
      el.className = "status-error";
      return;
    }

    el.className = "status-ok";
    var text = "";
    if (status.battery >= 0) text += status.battery + "% ";
    if (!status.lid_closed) text += "LID! ";
    if (!status.paper_loaded) text += "NO PAPER ";
    if (!text) text = "Ready";
    el.textContent = text.trim();
  }

  /**
   * Fetch initial printer statuses (SSE only sends updates, not initial state).
   */
  function fetchInitialStatuses() {
    fetch("/api/printers/status")
      .then(function (r) { return r.json(); })
      .then(function (statuses) {
        if (statuses && typeof statuses === "object") {
          Object.keys(statuses).forEach(function (id) {
            updatePrinterCard(id, statuses[id]);
            updateNavbarPrinter(statuses[id]);
          });
        }
      })
      .catch(function () {});
  }

  // Start SSE + fetch initial state on load
  initSSE();
  fetchInitialStatuses();

  // Re-fetch after HTMX swaps (navigating to printers page)
  document.body.addEventListener("htmx:afterSwap", function () {
    fetchInitialStatuses();
  });
})();


(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {EventSource|null} */
  var evtSource = null;

  /** Open an SSE connection to receive live printer status updates. */
  function initSSE() {
    if (evtSource) return;
    evtSource = new EventSource("/printers/events");
    evtSource.onmessage = function (e) {
      try {
        var evt = JSON.parse(e.data);
        if (evt.state !== undefined) {
          updatePrinterConnectionState(evt.printer_id, evt.state, evt.message);
        }
        if (evt.status !== undefined) {
          updatePrinterCard(evt.printer_id, evt.status);
          updateNavbarPrinter(evt.status);
        }
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
   * Update printer connection state with visual indicators.
   * @param {string} printerId
   * @param {string} state - "connecting"|"connected"|"disconnected"|"reconnecting"|"error"|"idle"
   * @param {string} message - Optional error message
   */
  function updatePrinterConnectionState(printerId, state, message) {
    var card = document.getElementById("printer-status-" + printerId);
    if (!card) return;

    var dot = card.querySelector(".conn-dot");
    if (!dot) {
      dot = document.createElement("span");
      dot.className = "conn-dot";
      card.prepend(dot);
    }
    dot.className = "conn-dot";

    var label = card.querySelector(".conn-label");
    if (!label) {
      label = document.createElement("span");
      label.className = "conn-label";
      dot.after(label);
    }

    switch (state) {
      case "idle":
      case "connecting":
      case "reconnecting":
        dot.classList.add("conn-dot--pulse");
        label.textContent = qlx.t("printers.state_connecting");
        break;
      case "connected":
        dot.classList.add("conn-dot--ok");
        label.textContent = qlx.t("printers.state_connected");
        break;
      case "disconnected":
        dot.classList.add("conn-dot--warn");
        label.textContent = qlx.t("printers.state_disconnected");
        break;
      case "error":
        dot.classList.add("conn-dot--error");
        label.textContent = message || qlx.t("printers.state_error");
        addReconnectButton(card, printerId);
        break;
    }

    if (state !== "error") {
      var btn = card.querySelector(".reconnect-btn");
      if (btn) btn.remove();
    }

    updateNavbarConnectionState(state);
  }

  /**
   * Update the navbar connection state class.
   * @param {string} state
   */
  function updateNavbarConnectionState(state) {
    var navEl = document.getElementById("printer-status");
    if (!navEl) return;
    navEl.className = "printer-nav-status printer-nav-status--" + state;
  }

  /**
   * Add a reconnect button to the printer card.
   * @param {HTMLElement} card
   * @param {string} printerId
   */
  function addReconnectButton(card, printerId) {
    if (card.querySelector(".reconnect-btn")) return;
    var btn = document.createElement("button");
    btn.className = "reconnect-btn btn btn-sm";
    btn.textContent = qlx.t("printers.reconnect");
    btn.addEventListener("click", function () {
      fetch("/printers/" + printerId + "/reconnect", { method: "POST" })
        .catch(function () {});
    });
    card.appendChild(btn);
  }

  // Start SSE on load (snapshot delivers initial state)
  initSSE();
})();

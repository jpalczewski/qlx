(function () {
  var qlx = window.qlx = window.qlx || {};

  var storedText = "";
  var storedFilename = "";

  function getOrCreateExportDialog() {
    var dlg = document.getElementById("export-dialog");
    if (dlg) return /** @type {HTMLDialogElement} */ (dlg);

    dlg = document.createElement("dialog");
    dlg.id = "export-dialog";
    dlg.className = "export-dialog";

    // --- Step 1: Options panel ---
    var options = document.createElement("div");
    options.className = "export-options";

    var title = document.createElement("h3");
    title.textContent = qlx.t("export.title");
    options.appendChild(title);

    // Format group
    var formatLabel = document.createElement("strong");
    formatLabel.textContent = qlx.t("export.format");
    options.appendChild(formatLabel);

    var formatGroup = document.createElement("div");
    formatGroup.className = "export-format-group";
    var formats = [
      { value: "csv", label: qlx.t("export.format_csv") },
      { value: "json", label: qlx.t("export.format_json") },
      { value: "md", label: qlx.t("export.format_md") }
    ];
    formats.forEach(function (f) {
      var lbl = document.createElement("label");
      var radio = document.createElement("input");
      radio.type = "radio";
      radio.name = "export-format";
      radio.value = f.value;
      lbl.appendChild(radio);
      var span = document.createElement("span");
      span.textContent = f.label;
      lbl.appendChild(span);
      formatGroup.appendChild(lbl);
    });
    options.appendChild(formatGroup);

    // Markdown style sub-group
    var mdGroup = document.createElement("div");
    mdGroup.className = "export-md-group";
    mdGroup.hidden = true;

    var mdLabel = document.createElement("strong");
    mdLabel.textContent = qlx.t("export.md_style");
    mdGroup.appendChild(mdLabel);

    var mdStyles = [
      { value: "table", label: qlx.t("export.md_table") },
      { value: "document", label: qlx.t("export.md_document") },
      { value: "both", label: qlx.t("export.md_both") }
    ];
    mdStyles.forEach(function (s, i) {
      var lbl = document.createElement("label");
      var radio = document.createElement("input");
      radio.type = "radio";
      radio.name = "export-md-style";
      radio.value = s.value;
      if (i === 0) radio.checked = true;
      lbl.appendChild(radio);
      var span = document.createElement("span");
      span.textContent = s.label;
      lbl.appendChild(span);
      mdGroup.appendChild(lbl);
    });
    options.appendChild(mdGroup);

    // Recursive checkbox (hidden by default, shown when containerId set)
    var recursiveLabel = document.createElement("label");
    recursiveLabel.className = "export-recursive-label";
    recursiveLabel.hidden = true;
    var recursiveCheck = document.createElement("input");
    recursiveCheck.type = "checkbox";
    recursiveCheck.name = "export-recursive";
    recursiveCheck.checked = true;
    recursiveLabel.appendChild(recursiveCheck);
    var recursiveSpan = document.createElement("span");
    recursiveSpan.textContent = qlx.t("export.recursive");
    recursiveLabel.appendChild(recursiveSpan);
    options.appendChild(recursiveLabel);

    // Footer with preview + cancel
    var optFooter = document.createElement("div");
    optFooter.className = "tree-picker-footer";

    var cancelBtn = document.createElement("button");
    cancelBtn.className = "btn btn-secondary btn-small";
    cancelBtn.textContent = qlx.t("action.cancel");
    cancelBtn.type = "button";
    cancelBtn.addEventListener("click", function () { /** @type {HTMLDialogElement} */ (dlg).close(); });
    optFooter.appendChild(cancelBtn);

    var previewBtn = document.createElement("button");
    previewBtn.className = "btn btn-primary btn-small";
    previewBtn.textContent = qlx.t("export.preview");
    previewBtn.type = "button";
    previewBtn.disabled = true;
    optFooter.appendChild(previewBtn);

    options.appendChild(optFooter);
    dlg.appendChild(options);

    // --- Step 2: Preview panel ---
    var preview = document.createElement("div");
    preview.className = "export-preview-panel";
    preview.hidden = true;

    var filenameEl = document.createElement("div");
    filenameEl.className = "export-filename";
    preview.appendChild(filenameEl);

    var pre = document.createElement("pre");
    pre.className = "export-preview";
    preview.appendChild(pre);

    var prevFooter = document.createElement("div");
    prevFooter.className = "tree-picker-footer";

    var prevCancelBtn = document.createElement("button");
    prevCancelBtn.className = "btn btn-secondary btn-small";
    prevCancelBtn.textContent = qlx.t("action.cancel");
    prevCancelBtn.type = "button";
    prevCancelBtn.addEventListener("click", function () { /** @type {HTMLDialogElement} */ (dlg).close(); });
    prevFooter.appendChild(prevCancelBtn);

    var backBtn = document.createElement("button");
    backBtn.className = "btn btn-secondary btn-small";
    backBtn.textContent = qlx.t("export.back");
    backBtn.type = "button";
    backBtn.addEventListener("click", function () {
      preview.hidden = true;
      options.hidden = false;
    });
    prevFooter.appendChild(backBtn);

    var downloadBtn = document.createElement("button");
    downloadBtn.className = "btn btn-secondary btn-small";
    downloadBtn.textContent = qlx.t("export.download");
    downloadBtn.type = "button";
    downloadBtn.addEventListener("click", function () {
      var blob = new Blob([storedText], { type: "text/plain" });
      var url = URL.createObjectURL(blob);
      var a = document.createElement("a");
      a.href = url;
      a.download = storedFilename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    });
    prevFooter.appendChild(downloadBtn);

    var copyBtn = document.createElement("button");
    copyBtn.className = "btn btn-primary btn-small";
    copyBtn.textContent = qlx.t("export.copy");
    copyBtn.type = "button";
    copyBtn.addEventListener("click", function () {
      navigator.clipboard.writeText(storedText).then(function () {
        var orig = copyBtn.textContent;
        copyBtn.textContent = qlx.t("export.copied");
        setTimeout(function () { copyBtn.textContent = orig; }, 2000);
      });
    });
    prevFooter.appendChild(copyBtn);

    preview.appendChild(prevFooter);
    dlg.appendChild(preview);

    // --- Behavior: format radio change ---
    formatGroup.addEventListener("change", function () {
      var sel = dlg.querySelector("input[name='export-format']:checked");
      mdGroup.hidden = !(sel && sel.value === "md");
      previewBtn.disabled = !sel;
    });

    // --- Behavior: preview fetch ---
    previewBtn.addEventListener("click", function () {
      var sel = dlg.querySelector("input[name='export-format']:checked");
      if (!sel) return;
      var format = sel.value;
      var params = "format=" + format;
      var cid = dlg.getAttribute("data-container-id");
      if (cid) params += "&container=" + encodeURIComponent(cid);
      var rec = dlg.querySelector("input[name='export-recursive']");
      if (rec && !rec.parentElement.hidden) params += "&recursive=" + (rec.checked ? "true" : "false");
      if (format === "md") {
        var msSel = dlg.querySelector("input[name='export-md-style']:checked");
        if (msSel) params += "&md_style=" + msSel.value;
      }
      fetch("/export?" + params)
        .then(function (resp) {
          var cd = resp.headers.get("Content-Disposition") || "";
          var match = cd.match(/filename="?([^";\s]+)"?/);
          storedFilename = match ? match[1] : "export." + format;
          return resp.text();
        })
        .then(function (text) {
          storedText = text;
          filenameEl.textContent = qlx.t("export.filename") + ": " + storedFilename;
          pre.textContent = text;
          options.hidden = true;
          preview.hidden = false;
        })
        .catch(function (err) {
          console.error("export preview failed:", err);
          qlx.showToast(qlx.t("error.connection"), true);
        });
    });

    // --- Close on backdrop click ---
    dlg.addEventListener("click", function (e) {
      if (e.target === dlg) {
        /** @type {HTMLDialogElement} */ (dlg).close();
      }
    });

    // --- Close on Escape is native, but reset on close ---
    dlg.addEventListener("close", function () {
      options.hidden = false;
      preview.hidden = true;
      storedText = "";
      storedFilename = "";
    });

    document.body.appendChild(dlg);
    return /** @type {HTMLDialogElement} */ (dlg);
  }

  qlx.openExportDialog = function openExportDialog(opts) {
    opts = opts || {};
    var dlg = getOrCreateExportDialog();

    // Reset state
    dlg.querySelectorAll("input[name='export-format']").forEach(function (r) { r.checked = false; });
    var mdGroup = dlg.querySelector(".export-md-group");
    if (mdGroup) mdGroup.hidden = true;
    var previewBtn = dlg.querySelector(".export-options .btn-primary");
    if (previewBtn) previewBtn.disabled = true;
    var optPanel = dlg.querySelector(".export-options");
    var prevPanel = dlg.querySelector(".export-preview-panel");
    if (optPanel) optPanel.hidden = false;
    if (prevPanel) prevPanel.hidden = true;

    // Container context
    if (opts.containerId) {
      dlg.setAttribute("data-container-id", opts.containerId);
    } else {
      dlg.removeAttribute("data-container-id");
    }
    var recLabel = dlg.querySelector(".export-recursive-label");
    if (recLabel) recLabel.hidden = !opts.containerId;

    dlg.showModal();
  };

  // Close dropdowns on outside click
  document.addEventListener("click", function (e) {
    if (!e.target.closest(".dropdown")) {
      document.querySelectorAll(".dropdown.open").forEach(function (d) {
        d.classList.remove("open");
      });
    }
  });
})();

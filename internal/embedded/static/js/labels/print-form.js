
(function () {
  var qlx = window.qlx = window.qlx || {};

  // ---- helpers ----

  function qs(el, sel) { return el.querySelector(sel); }
  function qsa(el, sel) { return el.querySelectorAll(sel); }

  /** Safely remove all children from an element. */
  function clearChildren(el) {
    while (el.firstChild) el.removeChild(el.firstChild);
  }

  function setResult(form, text) {
    var el = qs(form, "[data-print-result]");
    if (el) el.textContent = text;
  }

  function getFormValues(form) {
    var mode = form.getAttribute("data-print-mode");
    var printerSel = qs(form, "[data-print-printer]");
    var printerId = printerSel ? printerSel.value : "";
    var printerModel = "";
    if (printerSel && printerSel.selectedIndex >= 0) {
      printerModel = printerSel.options[printerSel.selectedIndex].getAttribute("data-model") || "";
    }

    var template = "";
    var templates = [];

    if (mode === "container-label") {
      var checked = qsa(form, 'input[name="print-schema"]:checked');
      for (var i = 0; i < checked.length; i++) {
        templates.push(checked[i].value);
      }
    } else {
      var tmplSel = qs(form, "[data-print-template]");
      if (tmplSel) template = tmplSel.value;
    }

    var dateEl = qs(form, "[data-print-date]");
    var printDate = dateEl ? dateEl.checked : false;

    var childrenEl = qs(form, "[data-print-children]");
    var showChildren = childrenEl ? childrenEl.checked : false;

    var textEl = qs(form, "[data-print-text]");
    var text = textEl ? textEl.value : "";

    return {
      mode: mode,
      printerId: printerId,
      printerModel: printerModel,
      template: template,
      templates: templates,
      printDate: printDate,
      showChildren: showChildren,
      text: text,
      endpoint: form.getAttribute("data-endpoint"),
      entityId: form.getAttribute("data-entity-id")
    };
  }

  function buildPrintBody(vals) {
    if (vals.mode === "container-label") {
      return JSON.stringify({
        printer_id: vals.printerId,
        templates: vals.templates,
        print_date: vals.printDate,
        show_children: vals.showChildren
      });
    }
    if (vals.mode === "adhoc") {
      return JSON.stringify({
        text: vals.text,
        printer_id: vals.printerId,
        template: vals.template,
        print_date: vals.printDate
      });
    }
    // item or bulk-items
    return JSON.stringify({
      printer_id: vals.printerId,
      template: vals.template,
      print_date: vals.printDate
    });
  }

  // ---- client-side rendering for designer templates ----

  function renderDesigner(response, printerId, resultEl) {
    var elements = [];
    try { elements = JSON.parse(response.template.elements || "[]"); } catch (e) { /* empty */ }
    var itemData = response.item_data || {};

    var params = {
      name: itemData.Name || "",
      description: itemData.Description || "",
      location: itemData.Location || "",
      id: itemData.BarcodeID || "",
      qr_url: itemData.QRContent || "",
      date: new Date().toISOString().slice(0, 10),
      time: new Date().toTimeString().slice(0, 5),
      printer: ""
    };

    var t = response.template;
    var w = t.width_mm > 0 ? Math.round(t.width_mm * 203 / 25.4) : (t.width_px || 384);
    var h = t.height_mm > 0 ? Math.round(t.height_mm * 203 / 25.4) : (t.height_px || 240);

    var tmpCanvas = document.createElement("canvas");
    tmpCanvas.width = w;
    tmpCanvas.height = h;
    tmpCanvas.style.display = "none";
    document.body.appendChild(tmpCanvas);

    var fc = new fabric.StaticCanvas(tmpCanvas, { width: w, height: h, backgroundColor: "#ffffff" });
    return window.QlxFormat.qlxToCanvas(fc, elements, params).then(function () {
      fc.renderAll();
      return window.LabelPrint.print(fc, printerId, 2);
    }).then(function () {
      if (resultEl) resultEl.textContent = qlx.t("inventory.printed");
    }).catch(function (e) {
      if (resultEl) resultEl.textContent = qlx.t("error.status") + ": " + e;
    }).finally(function () {
      fc.dispose();
      if (tmpCanvas.parentNode) tmpCanvas.parentNode.removeChild(tmpCanvas);
    });
  }

  // ---- preview ----

  function getPreviewDialog() {
    return document.querySelector("[data-preview-dialog]");
  }

  function showPreview(form) {
    var vals = getFormValues(form);
    var dialog = getPreviewDialog();
    if (!dialog) return;

    var titleEl = qs(dialog, "[data-preview-title]");
    var contentEl = qs(dialog, "[data-preview-content]");
    var loadingEl = qs(dialog, "[data-preview-loading]");
    var ditherCheckbox = qs(dialog, "[data-preview-dither]");

    if (titleEl) titleEl.textContent = qlx.t("labels.preview_title");
    clearChildren(contentEl);
    if (loadingEl) {
      var spinner = loadingEl.cloneNode(true);
      spinner.style.display = "";
      contentEl.appendChild(spinner);
    }

    if (ditherCheckbox) ditherCheckbox.checked = false;

    // Store form reference for print action
    dialog._printForm = form;
    dialog._previewState = null;

    dialog.showModal();

    if (vals.mode === "adhoc" && !vals.text.trim()) {
      clearChildren(contentEl);
      var p = document.createElement("p");
      p.textContent = qlx.t("inventory.enter_text");
      contentEl.appendChild(p);
      return;
    }

    if (vals.mode === "container-label") {
      if (vals.templates.length === 0) {
        clearChildren(contentEl);
        var msg = document.createElement("p");
        msg.textContent = qlx.t("inventory.select_template");
        contentEl.appendChild(msg);
        return;
      }
      loadServerPreview(vals, contentEl, dialog);
    } else if (vals.mode === "bulk-items") {
      clearChildren(contentEl);
      var note = document.createElement("p");
      note.className = "preview-note";
      note.textContent = qlx.t("labels.preview_bulk_note");
      contentEl.appendChild(note);
    } else {
      loadServerPreview(vals, contentEl, dialog);
    }
  }

  function loadServerPreview(vals, contentEl, dialog) {
    var previewUrl;

    if (vals.mode === "adhoc") {
      previewUrl = "/adhoc/preview?template=" + encodeURIComponent(vals.template) +
        "&text=" + encodeURIComponent(vals.text) +
        "&print_date=" + vals.printDate;
    } else if (vals.mode === "container-label") {
      previewUrl = "/containers/" + vals.entityId + "/preview?template=" +
        encodeURIComponent(vals.templates[0]) +
        "&print_date=" + vals.printDate +
        "&show_children=" + vals.showChildren;
    } else {
      // item
      previewUrl = "/items/" + vals.entityId + "/preview?template=" +
        encodeURIComponent(vals.template) +
        "&print_date=" + vals.printDate;
    }

    fetch(previewUrl, {
      headers: { "Accept": "application/json, image/png" }
    }).then(function (resp) {
      var ct = resp.headers.get("Content-Type") || "";
      if (ct.indexOf("image/png") >= 0) {
        return resp.blob().then(function (blob) {
          return { type: "image", blob: blob };
        });
      }
      return resp.json().then(function (data) {
        return { type: "json", data: data };
      });
    }).then(function (result) {
      clearChildren(contentEl);

      if (result.type === "image") {
        // Server-rendered PNG (built-in schema)
        var url = URL.createObjectURL(result.blob);
        var img = document.createElement("img");
        img.src = url;
        img.className = "preview-image";
        img.alt = "Label preview";
        contentEl.appendChild(img);

        dialog._previewState = { type: "image", img: img, originalUrl: url };
      } else if (result.data && result.data.render === "client") {
        // Designer template: render in Fabric.js canvas
        renderDesignerPreview(result.data, contentEl, dialog);
      } else if (result.data && result.data.error) {
        var errP = document.createElement("p");
        errP.textContent = result.data.error;
        contentEl.appendChild(errP);
      }
    }).catch(function (e) {
      clearChildren(contentEl);
      var errP = document.createElement("p");
      errP.textContent = qlx.t("error.status") + ": " + e;
      contentEl.appendChild(errP);
    });
  }

  function renderDesignerPreview(response, contentEl, dialog) {
    var elements = [];
    try { elements = JSON.parse(response.template.elements || "[]"); } catch (e) { /* empty */ }
    var itemData = response.item_data || {};
    var params = {
      name: itemData.Name || "",
      description: itemData.Description || "",
      location: itemData.Location || "",
      id: itemData.BarcodeID || "",
      qr_url: itemData.QRContent || "",
      date: new Date().toISOString().slice(0, 10),
      time: new Date().toTimeString().slice(0, 5),
      printer: ""
    };

    var t = response.template;
    var w = t.width_mm > 0 ? Math.round(t.width_mm * 203 / 25.4) : (t.width_px || 384);
    var h = t.height_mm > 0 ? Math.round(t.height_mm * 203 / 25.4) : (t.height_px || 240);

    var canvasEl = document.createElement("canvas");
    canvasEl.width = w;
    canvasEl.height = h;
    canvasEl.className = "preview-canvas";
    contentEl.appendChild(canvasEl);

    var fc = new fabric.StaticCanvas(canvasEl, { width: w, height: h, backgroundColor: "#ffffff" });
    window.QlxFormat.qlxToCanvas(fc, elements, params).then(function () {
      fc.renderAll();
      dialog._previewState = {
        type: "canvas",
        canvas: canvasEl,
        fabricCanvas: fc,
        response: response
      };
    });
  }

  function applyDither(dialog, enabled) {
    var state = dialog._previewState;
    if (!state) return;

    var contentEl = qs(dialog, "[data-preview-content]");

    if (state.type === "image") {
      if (enabled) {
        var img = state.img;
        var tmpCanvas = document.createElement("canvas");
        tmpCanvas.width = img.naturalWidth;
        tmpCanvas.height = img.naturalHeight;
        var ctx = tmpCanvas.getContext("2d");
        ctx.drawImage(img, 0, 0);
        var dithered = window.LabelDither.dither(tmpCanvas);
        img.src = dithered.toDataURL("image/png");
      } else {
        state.img.src = state.originalUrl;
      }
    } else if (state.type === "canvas") {
      if (enabled) {
        var dataUrl = state.fabricCanvas.toDataURL({ format: "png", multiplier: 2 });
        var tmpImg = new Image();
        tmpImg.onload = function () {
          var tmpC = document.createElement("canvas");
          tmpC.width = tmpImg.width;
          tmpC.height = tmpImg.height;
          var tmpCtx = tmpC.getContext("2d");
          tmpCtx.drawImage(tmpImg, 0, 0);
          var ditherResult = window.LabelDither.dither(tmpC);

          state.canvas.style.display = "none";
          var ditherImg = qs(contentEl, ".preview-dithered");
          if (!ditherImg) {
            ditherImg = document.createElement("img");
            ditherImg.className = "preview-dithered preview-image";
            contentEl.appendChild(ditherImg);
          }
          ditherImg.src = ditherResult.toDataURL("image/png");
          ditherImg.style.display = "";
        };
        tmpImg.src = dataUrl;
      } else {
        state.canvas.style.display = "";
        var existing = qs(contentEl, ".preview-dithered");
        if (existing) existing.style.display = "none";
      }
    }
  }

  // ---- direct print (no preview) ----

  function doPrint(form) {
    var vals = getFormValues(form);

    if (vals.mode === "adhoc" && !vals.text.trim()) {
      setResult(form, qlx.t("inventory.enter_text"));
      return;
    }

    if (vals.mode === "container-label" && vals.templates.length === 0) {
      setResult(form, qlx.t("inventory.select_template"));
      return;
    }

    if (vals.mode === "bulk-items") {
      doBulkPrint(form, vals);
      return;
    }

    setResult(form, "...");

    var body = buildPrintBody(vals);

    fetch(vals.endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Accept": "application/json" },
      body: body
    }).then(function (r) { return r.json(); }).then(function (d) {
      if (!d.ok) {
        setResult(form, qlx.t("error.status") + ": " + (d.error || "unknown"));
        return;
      }
      if (d.render === "client") {
        setResult(form, qlx.t("labels.rendering"));
        renderDesigner(d, vals.printerId, qs(form, "[data-print-result]"));
        return;
      }
      setResult(form, qlx.t("inventory.printed"));
    }).catch(function (e) {
      setResult(form, qlx.t("error.status") + ": " + e);
    });
  }

  // ---- bulk print ----

  function doBulkPrint(form, vals) {
    setResult(form, qlx.t("labels.fetching_items"));

    fetch("/containers/" + vals.entityId + "/items-json", {
      headers: { "Accept": "application/json" }
    }).then(function (r) { return r.json(); }).then(function (items) {
      if (!items || items.length === 0) {
        setResult(form, qlx.t("labels.no_items_to_print"));
        return;
      }

      setResult(form, qlx.t("labels.printing_count").replace("{count}", items.length));

      var printed = 0;
      var errors = [];

      function printNext(i) {
        if (i >= items.length) {
          if (errors.length > 0) {
            setResult(form, qlx.t("labels.printed_with_errors")
              .replace("{printed}", printed)
              .replace("{total}", items.length)
              .replace("{errors}", errors.join(", ")));
          } else {
            setResult(form, qlx.t("labels.printed_count").replace("{count}", printed));
          }
          return;
        }

        fetch("/items/" + items[i].id + "/print", {
          method: "POST",
          headers: { "Content-Type": "application/json", "Accept": "application/json" },
          body: JSON.stringify({ printer_id: vals.printerId, template: vals.template })
        }).then(function (r) { return r.json(); }).then(function (d) {
          if (d.ok && d.render !== "client") {
            printed++;
          } else if (d.ok && d.render === "client") {
            return renderDesigner(d, vals.printerId, null).then(function () {
              printed++;
            });
          } else {
            errors.push(items[i].name + ": " + (d.error || "failed"));
          }
        }).catch(function (e) {
          errors.push(items[i].name + ": " + e);
        }).finally(function () {
          printNext(i + 1);
        });
      }

      printNext(0);
    }).catch(function (e) {
      setResult(form, qlx.t("error.status") + ": " + e);
    });
  }

  // ---- print from preview dialog ----

  function printFromPreview(dialog) {
    var form = dialog._printForm;
    if (!form) return;

    dialog.close();
    doPrint(form);
  }

  // ---- template filtering ----

  function filterTemplates(form) {
    var printerSel = qs(form, "[data-print-printer]");
    var templateSel = qs(form, "[data-print-template]");
    if (!printerSel || !templateSel) return;

    var selected = printerSel.options[printerSel.selectedIndex];
    var model = selected ? selected.getAttribute("data-model") : "";

    var firstVisible = null;
    var currentHidden = false;

    Array.from(templateSel.options).forEach(function (opt) {
      var target = opt.getAttribute("data-target");
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

    if (currentHidden && firstVisible) {
      firstVisible.selected = true;
    }
  }

  // ---- initialization ----

  function initForm(form) {
    if (form._printFormInit) return;
    form._printFormInit = true;

    var printerSel = qs(form, "[data-print-printer]");
    if (printerSel) {
      printerSel.addEventListener("change", function () {
        filterTemplates(form);
      });
      filterTemplates(form);
    }

    var printBtn = qs(form, "[data-print-btn]");
    if (printBtn) {
      printBtn.addEventListener("click", function () {
        doPrint(form);
      });
    }

    var previewBtn = qs(form, "[data-print-preview]");
    if (previewBtn) {
      previewBtn.addEventListener("click", function () {
        showPreview(form);
      });
    }
  }

  function initDialog() {
    var dialog = getPreviewDialog();
    if (!dialog || dialog._initialized) return;
    dialog._initialized = true;

    // Set text labels from i18n
    var ditherLabel = qs(dialog, "[data-preview-dither-label]");
    if (ditherLabel) ditherLabel.textContent = qlx.t("labels.show_dithering");
    var cancelLabel = qs(dialog, "[data-preview-cancel-label]");
    if (cancelLabel) cancelLabel.textContent = qlx.t("action.cancel");
    var printLabel = qs(dialog, "[data-preview-print-label]");
    if (printLabel) printLabel.textContent = qlx.t("action.print");

    var closeBtns = qsa(dialog, "[data-preview-close]");
    for (var i = 0; i < closeBtns.length; i++) {
      closeBtns[i].addEventListener("click", function () {
        dialog.close();
      });
    }

    var ditherCheckbox = qs(dialog, "[data-preview-dither]");
    if (ditherCheckbox) {
      ditherCheckbox.addEventListener("change", function () {
        applyDither(dialog, ditherCheckbox.checked);
      });
    }

    var printBtn = qs(dialog, "[data-preview-print]");
    if (printBtn) {
      printBtn.addEventListener("click", function () {
        printFromPreview(dialog);
      });
    }

    // Close on backdrop click
    dialog.addEventListener("click", function (e) {
      if (e.target === dialog) dialog.close();
    });
  }

  function initAll(root) {
    root = root || document;
    var forms = qsa(root, "[data-print-form]");
    for (var i = 0; i < forms.length; i++) {
      initForm(forms[i]);
    }
    initDialog();
  }

  // Auto-init on page load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { initAll(); });
  } else {
    initAll();
  }

  // Re-init after HTMX swaps
  document.body.addEventListener("htmx:afterSwap", function (e) {
    initAll(e.detail.target);
  });

  // Public API
  qlx.PrintForm = {
    init: initAll,
    print: doPrint,
    preview: showPreview,
    filterTemplates: filterTemplates
  };
})();

(function () {
  var DPI = 203;
  var MM_TO_PX = DPI / 25.4;

  var app = null;
  var fabricCanvas = null;
  var previewCanvas = null;
  var templateId = null;
  var previewData = null;
  var debounceTimer = null;

  function init() {
    // Dispose previous canvases if re-initializing (HTMX navigation)
    if (fabricCanvas) {
      try { fabricCanvas.dispose(); } catch (e) {}
      fabricCanvas = null;
    }
    if (previewCanvas) {
      try { previewCanvas.dispose(); } catch (e) {}
      previewCanvas = null;
    }

    app = document.getElementById("designer-app");
    if (!app) return;

    templateId = app.getAttribute("data-template-id") || "";
    var templateJson = app.getAttribute("data-template-json") || "[]";
    previewData = {};
    try {
      previewData = JSON.parse(app.getAttribute("data-preview-data") || "{}");
    } catch (e) {}

    var elements = [];
    try {
      elements = JSON.parse(templateJson);
    } catch (e) {}

    // Determine canvas size
    var target = getTargetValue();
    var size = calculateCanvasSize(target);

    // Init Fabric canvas
    var canvasEl = document.getElementById("label-canvas");
    if (!canvasEl) return;

    // Clean up stale Fabric state from previous HTMX swap
    canvasEl.removeAttribute("data-fabric");
    canvasEl.className = "";
    canvasEl.width = size.width;
    canvasEl.height = size.height;

    // Remove any leftover Fabric wrapper from previous init
    var oldWrapper = canvasEl.closest(".canvas-container");
    if (oldWrapper && oldWrapper.parentNode) {
      oldWrapper.parentNode.insertBefore(canvasEl, oldWrapper);
      oldWrapper.remove();
    }

    fabricCanvas = new fabric.Canvas("label-canvas", {
      width: size.width,
      height: size.height,
      backgroundColor: "#ffffff",
      selection: true
    });

    // Init preview canvas
    var previewEl = document.getElementById("preview-canvas");
    if (previewEl) {
      previewEl.removeAttribute("data-fabric");
      previewEl.className = "";
      var oldPreviewWrapper = previewEl.closest(".canvas-container");
      if (oldPreviewWrapper && oldPreviewWrapper.parentNode) {
        oldPreviewWrapper.parentNode.insertBefore(previewEl, oldPreviewWrapper);
        oldPreviewWrapper.remove();
      }
      previewCanvas = new fabric.StaticCanvas("preview-canvas", {
        width: size.width,
        height: size.height,
        backgroundColor: "#ffffff"
      });
    }

    // Load existing elements
    if (elements.length > 0) {
      window.QlxFormat.qlxToCanvas(fabricCanvas, elements).then(function () {
        updatePreview();
      }).catch(function (err) {
        console.error("Failed to load template elements:", err);
      });
    }

    // Set up event listeners
    setupToolbar();
    setupProperties();
    setupCanvasEvents();
    setupSave();
    setupTargetAndSize();

    // Scale preview to fit on init
    setTimeout(scalePreviewToFit, 50);
  }

  function getTargetValue() {
    var sel = document.getElementById("template-target");
    return sel ? sel.value : "universal";
  }

  function calculateCanvasSize(target) {
    var widthInput = document.getElementById("template-width");
    var heightInput = document.getElementById("template-height");

    if (target === "universal") {
      var wmm = parseFloat(widthInput ? widthInput.value : 50) || 50;
      var hmm = parseFloat(heightInput ? heightInput.value : 30) || 30;
      return {
        width: Math.round(wmm * MM_TO_PX),
        height: Math.round(hmm * MM_TO_PX)
      };
    }
    // Printer-specific: direct px
    var wpx = parseInt(widthInput ? widthInput.value : 384) || 384;
    var hpx = parseInt(heightInput ? heightInput.value : 240) || 240;
    return { width: wpx, height: hpx };
  }

  // --- Toolbar ---
  function setupToolbar() {
    var toolbar = document.getElementById("toolbar");
    if (!toolbar) return;

    toolbar.addEventListener("click", function (e) {
      var btn = e.target.closest("[data-tool], [data-action]");
      if (!btn) return;

      var tool = btn.getAttribute("data-tool");
      var action = btn.getAttribute("data-action");

      if (tool === "text") addText();
      else if (tool === "qr") addQR();
      else if (tool === "barcode") addBarcode();
      else if (tool === "line") addLine();
      else if (tool === "img") addImgPlaceholder();

      if (action === "delete") deleteSelected();
    });
  }

  function addText() {
    if (!fabricCanvas) return;
    var tb = new fabric.Textbox("{{name}}", {
      left: 10,
      top: 10,
      width: 150,
      fontSize: 16,
      fontFamily: "Arial, sans-serif",
      fontWeight: "normal",
      fontStyle: "normal",
      textAlign: "left",
      fill: "#000000"
    });
    tb.qlxType = "text";
    tb.qlxTemplate = "{{name}}";
    fabricCanvas.add(tb);
    fabricCanvas.setActiveObject(tb);
    fabricCanvas.renderAll();
    onCanvasModified();
  }

  function addQR() {
    if (!fabricCanvas) return;
    window.QlxFormat.qlxToCanvas(fabricCanvas, [{
      type: "qr", x: 10, y: 10, size: 80, content: "{{qr_url}}"
    }]).then(function () {
      var objects = fabricCanvas.getObjects();
      var last = objects[objects.length - 1];
      if (last) {
        last.qlxType = "qr";
        last.qlxContent = "{{qr_url}}";
        fabricCanvas.setActiveObject(last);
      }
      onCanvasModified();
    }).catch(function (err) { console.error("QR add failed:", err); });
  }

  function addBarcode() {
    if (!fabricCanvas) return;
    window.QlxFormat.qlxToCanvas(fabricCanvas, [{
      type: "barcode", x: 10, y: 10, width: 150, height: 50, content: "{{id}}", format: "code128"
    }]).then(function () {
      var objects = fabricCanvas.getObjects();
      var last = objects[objects.length - 1];
      if (last) {
        last.qlxType = "barcode";
        last.qlxContent = "{{id}}";
        fabricCanvas.setActiveObject(last);
      }
      onCanvasModified();
    }).catch(function (err) { console.error("Barcode add failed:", err); });
  }

  function addLine() {
    if (!fabricCanvas) return;
    var line = new fabric.Line([0, 0, 150, 0], {
      left: 10,
      top: 50,
      stroke: "#000000",
      strokeWidth: 2,
      strokeLineCap: "round"
    });
    line.qlxType = "line";
    fabricCanvas.add(line);
    fabricCanvas.setActiveObject(line);
    fabricCanvas.renderAll();
    onCanvasModified();
  }

  function addImgPlaceholder() {
    if (!fabricCanvas) return;
    var rect = new fabric.Rect({
      left: 10,
      top: 10,
      width: 80,
      height: 80,
      fill: "#cccccc",
      stroke: "#999999",
      strokeWidth: 1,
      strokeDashArray: [4, 4]
    });
    rect.qlxType = "img";
    rect.qlxSrc = "";
    rect.qlxFit = "contain";
    fabricCanvas.add(rect);
    fabricCanvas.setActiveObject(rect);
    fabricCanvas.renderAll();
    onCanvasModified();
  }

  function deleteSelected() {
    if (!fabricCanvas) return;
    var active = fabricCanvas.getActiveObject();
    if (active) {
      fabricCanvas.remove(active);
      fabricCanvas.discardActiveObject();
      fabricCanvas.renderAll();
      onCanvasModified();
    }
  }

  // --- Properties Panel ---
  function setupProperties() {
    fabricCanvas.on("selection:created", showProperties);
    fabricCanvas.on("selection:updated", showProperties);
    fabricCanvas.on("selection:cleared", clearProperties);
  }

  function showProperties() {
    var panel = document.getElementById("props-content");
    if (!panel) return;
    var obj = fabricCanvas.getActiveObject();
    if (!obj || !obj.qlxType) {
      clearProperties();
      return;
    }

    // Build properties using safe DOM methods
    while (panel.firstChild) panel.removeChild(panel.firstChild);

    var group = document.createElement("div");
    group.className = "prop-group";

    var t = obj.qlxType;

    appendField(group, "X", "prop-x", "number", Math.round(obj.left));
    appendField(group, "Y", "prop-y", "number", Math.round(obj.top));

    if (t === "text") {
      appendTextarea(group, "Text", "prop-text", obj.qlxTemplate || obj.text || "");
      appendSelect(group, "Font", "prop-font", [
        { v: "Arial, sans-serif", l: "Sans" },
        { v: "Georgia, serif", l: "Serif" },
        { v: "Courier New, monospace", l: "Mono" }
      ], obj.fontFamily);
      appendField(group, "Size", "prop-size", "number", obj.fontSize || 16);
      appendCheckbox(group, "Bold", "prop-bold", obj.fontWeight === "bold");
      appendSelect(group, "Align", "prop-align", [
        { v: "left", l: "Left" },
        { v: "center", l: "Center" },
        { v: "right", l: "Right" }
      ], obj.textAlign);
    } else if (t === "qr") {
      appendField(group, "Content", "prop-content", "text", obj.qlxContent || "");
      appendField(group, "Size", "prop-qr-size", "number", Math.round(obj.getScaledWidth()));
    } else if (t === "barcode") {
      appendField(group, "Content", "prop-content", "text", obj.qlxContent || "");
      appendField(group, "Width", "prop-width", "number", Math.round(obj.getScaledWidth()));
      appendField(group, "Height", "prop-height", "number", Math.round(obj.getScaledHeight()));
    } else if (t === "line") {
      appendField(group, "Thickness", "prop-thickness", "number", obj.strokeWidth || 1);
    } else if (t === "img") {
      appendField(group, "Src", "prop-src", "text", obj.qlxSrc || "");

      var uploadLabel = document.createElement("label");
      uploadLabel.textContent = "Upload";
      group.appendChild(uploadLabel);

      var fileInput = document.createElement("input");
      fileInput.type = "file";
      fileInput.id = "prop-file";
      fileInput.accept = "image/*";
      group.appendChild(fileInput);

      appendSelect(group, "Fit", "prop-fit", [
        { v: "contain", l: "Contain" },
        { v: "cover", l: "Cover" },
        { v: "stretch", l: "Stretch" }
      ], obj.qlxFit || "contain");
    }

    panel.appendChild(group);

    // Bind change events
    bindPropEvents(obj);
  }

  function appendField(parent, labelText, id, type, value) {
    var label = document.createElement("label");
    label.setAttribute("for", id);
    label.textContent = labelText;
    parent.appendChild(label);

    var input = document.createElement("input");
    input.type = type;
    input.id = id;
    input.value = String(value);
    parent.appendChild(input);
  }

  function appendTextarea(parent, labelText, id, value) {
    var label = document.createElement("label");
    label.setAttribute("for", id);
    label.textContent = labelText;
    parent.appendChild(label);

    var textarea = document.createElement("textarea");
    textarea.id = id;
    textarea.rows = 2;
    textarea.textContent = value;
    parent.appendChild(textarea);
  }

  function appendSelect(parent, labelText, id, options, current) {
    var label = document.createElement("label");
    label.setAttribute("for", id);
    label.textContent = labelText;
    parent.appendChild(label);

    var select = document.createElement("select");
    select.id = id;
    for (var i = 0; i < options.length; i++) {
      var opt = document.createElement("option");
      opt.value = options[i].v;
      opt.textContent = options[i].l;
      if (options[i].v === current) opt.selected = true;
      select.appendChild(opt);
    }
    parent.appendChild(select);
  }

  function appendCheckbox(parent, labelText, id, checked) {
    var label = document.createElement("label");
    label.setAttribute("for", id);
    label.textContent = labelText;
    parent.appendChild(label);

    var input = document.createElement("input");
    input.type = "checkbox";
    input.id = id;
    input.checked = checked;
    parent.appendChild(input);
  }

  function bindPropEvents(obj) {
    var t = obj.qlxType;

    bindInput("prop-x", function (v) { obj.set("left", parseInt(v) || 0); });
    bindInput("prop-y", function (v) { obj.set("top", parseInt(v) || 0); });

    if (t === "text") {
      bindInput("prop-text", function (v) {
        obj.qlxTemplate = v;
        obj.set("text", v);
      });
      bindInput("prop-font", function (v) { obj.set("fontFamily", v); });
      bindInput("prop-size", function (v) { obj.set("fontSize", parseInt(v) || 16); });
      bindInput("prop-bold", function (v, el) {
        obj.set("fontWeight", el.checked ? "bold" : "normal");
      });
      bindInput("prop-align", function (v) { obj.set("textAlign", v); });
    } else if (t === "qr") {
      bindInput("prop-content", function (v) {
        obj.qlxContent = v;
        reRenderQR(obj, v);
      });
      bindInput("prop-qr-size", function (v) {
        var s = parseInt(v) || 80;
        obj.set({ scaleX: s / obj.width, scaleY: s / obj.height });
      });
    } else if (t === "barcode") {
      bindInput("prop-content", function (v) {
        obj.qlxContent = v;
        reRenderBarcode(obj, v);
      });
      bindInput("prop-width", function (v) {
        obj.set("scaleX", (parseInt(v) || 150) / obj.width);
      });
      bindInput("prop-height", function (v) {
        obj.set("scaleY", (parseInt(v) || 50) / obj.height);
      });
    } else if (t === "line") {
      bindInput("prop-thickness", function (v) {
        obj.set("strokeWidth", parseInt(v) || 1);
      });
    } else if (t === "img") {
      bindInput("prop-src", function (v) { obj.qlxSrc = v; });
      bindInput("prop-fit", function (v) { obj.qlxFit = v; });
      var fileInput = document.getElementById("prop-file");
      if (fileInput) {
        fileInput.addEventListener("change", function () {
          if (fileInput.files && fileInput.files[0]) {
            uploadAsset(fileInput.files[0], obj);
          }
        });
      }
    }
  }

  function bindInput(id, fn) {
    var el = document.getElementById(id);
    if (!el) return;
    var event = el.tagName === "SELECT" ? "change" : "input";
    if (el.type === "checkbox") event = "change";
    el.addEventListener(event, function () {
      fn(el.value, el);
      fabricCanvas.renderAll();
      onCanvasModified();
    });
  }

  function reRenderQR(obj, content) {
    try {
      var qr = qrcode(0, "M");
      qr.addData(content || "https://example.com");
      qr.make();
      var dataUrl = qr.createDataURL(4, 0);
      var imgEl = new Image();
      imgEl.onload = function () {
        obj.setElement(imgEl);
        fabricCanvas.renderAll();
        onCanvasModified();
      };
      imgEl.src = dataUrl;
    } catch (e) {}
  }

  function reRenderBarcode(obj, content) {
    try {
      var tmpCanvas = document.createElement("canvas");
      JsBarcode(tmpCanvas, content || "0000", {
        format: "CODE128",
        width: 2,
        height: Math.round(obj.getScaledHeight()),
        displayValue: false,
        margin: 0
      });
      var imgEl = new Image();
      imgEl.onload = function () {
        obj.setElement(imgEl);
        fabricCanvas.renderAll();
        onCanvasModified();
      };
      imgEl.src = tmpCanvas.toDataURL();
    } catch (e) {}
  }

  function uploadAsset(file, obj) {
    var formData = new FormData();
    formData.append("file", file);

    fetch("/assets", { method: "POST", body: formData, headers: { "Accept": "application/json" } })
      .then(function (resp) {
        if (!resp.ok) throw new Error("Upload failed");
        return resp.json();
      })
      .then(function (data) {
        var assetId = data.id;
        obj.qlxSrc = "asset:" + assetId;

        // Load the image and replace the placeholder
        var imgEl = new Image();
        imgEl.crossOrigin = "anonymous";
        imgEl.onload = function () {
          var w = obj.getScaledWidth();
          var h = obj.getScaledHeight();
          var left = obj.left;
          var top = obj.top;
          fabricCanvas.remove(obj);

          var fImg = new fabric.Image(imgEl, {
            left: left,
            top: top,
            scaleX: w / imgEl.width,
            scaleY: h / imgEl.height
          });
          fImg.qlxType = "img";
          fImg.qlxSrc = "asset:" + assetId;
          fImg.qlxFit = obj.qlxFit || "contain";
          fabricCanvas.add(fImg);
          fabricCanvas.setActiveObject(fImg);
          fabricCanvas.renderAll();
          onCanvasModified();
          showToast("Image uploaded");
        };
        imgEl.onerror = function () {
          showToast("Failed to load uploaded image", true);
        };
        imgEl.src = "/assets/" + assetId;
      })
      .catch(function (err) {
        showToast("Upload failed: " + err.message, true);
      });
  }

  function clearProperties() {
    var panel = document.getElementById("props-content");
    if (panel) {
      while (panel.firstChild) panel.removeChild(panel.firstChild);
      var p = document.createElement("p");
      p.className = "empty";
      p.textContent = "Select an element";
      panel.appendChild(p);
    }
  }

  // --- Canvas Events ---
  function setupCanvasEvents() {
    fabricCanvas.on("object:modified", function () { onCanvasModified(); });
    fabricCanvas.on("object:added", function () { onCanvasModified(); });
    fabricCanvas.on("object:removed", function () { onCanvasModified(); });

    // Sync Fabric in-place text editing back to qlxTemplate and properties panel
    fabricCanvas.on("text:changed", function (e) {
      var obj = e.target;
      if (obj && obj.qlxType === "text") {
        obj.qlxTemplate = obj.text;
        // Update properties panel if this object is selected
        var active = fabricCanvas.getActiveObject();
        if (active === obj) {
          var textInput = document.getElementById("prop-text");
          if (textInput) textInput.value = obj.text;
        }
      }
      onCanvasModified();
    });

    // Refresh properties when object is moved/resized
    fabricCanvas.on("object:modified", function () {
      var active = fabricCanvas.getActiveObject();
      if (active) {
        var xInput = document.getElementById("prop-x");
        var yInput = document.getElementById("prop-y");
        if (xInput) xInput.value = Math.round(active.left);
        if (yInput) yInput.value = Math.round(active.top);
      }
    });
  }

  function onCanvasModified() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(updatePreview, 200);
  }

  function updatePreview() {
    if (!previewCanvas) return;

    var elements = window.QlxFormat.canvasToQlx(fabricCanvas);
    previewCanvas.clear();
    previewCanvas.backgroundColor = "#ffffff";

    if (elements.length > 0) {
      window.QlxFormat.qlxToCanvas(previewCanvas, elements, previewData).then(function () {
        previewCanvas.renderAll();
      }).catch(function (err) {
        console.error("Preview render failed:", err);
      });
    }

    // Scale preview container to fit within preview area
    scalePreviewToFit();
  }

  function scalePreviewToFit() {
    var previewArea = document.querySelector(".designer-preview-area");
    var container = previewArea ? previewArea.querySelector(".canvas-container") : null;
    if (!container || !previewCanvas) return;

    var areaWidth = previewArea.clientWidth - 16; // padding
    var canvasWidth = previewCanvas.getWidth();
    if (canvasWidth <= 0 || areaWidth <= 0) return;

    var scale = Math.min(1, areaWidth / canvasWidth);
    container.style.transformOrigin = "top left";
    container.style.transform = "scale(" + scale + ")";
    // Set container height to match scaled size to prevent layout overflow
    container.style.height = (previewCanvas.getHeight() * scale) + "px";
  }

  // --- Target & Size ---
  function setupTargetAndSize() {
    var targetSel = document.getElementById("template-target");
    var widthInput = document.getElementById("template-width");
    var heightInput = document.getElementById("template-height");
    var unitLabel = document.getElementById("size-unit");

    if (targetSel) {
      targetSel.addEventListener("change", function () {
        var target = targetSel.value;
        if (unitLabel) {
          unitLabel.textContent = target === "universal" ? "mm" : "px";
        }
        resizeCanvas();
      });
    }

    if (widthInput) widthInput.addEventListener("change", resizeCanvas);
    if (heightInput) heightInput.addEventListener("change", resizeCanvas);
  }

  function resizeCanvas() {
    var target = getTargetValue();
    var size = calculateCanvasSize(target);

    fabricCanvas.setDimensions({ width: size.width, height: size.height });
    fabricCanvas.renderAll();

    if (previewCanvas) {
      previewCanvas.setDimensions({ width: size.width, height: size.height });
    }
    onCanvasModified();
    scalePreviewToFit();
  }

  // --- Save ---
  function setupSave() {
    var saveBtn = document.getElementById("save-template");
    if (!saveBtn) return;

    saveBtn.addEventListener("click", function () {
      var nameInput = document.getElementById("template-name");
      var tagsInput = document.getElementById("template-tags");
      var targetSel = document.getElementById("template-target");
      var widthInput = document.getElementById("template-width");
      var heightInput = document.getElementById("template-height");

      var name = nameInput ? nameInput.value.trim() : "";
      if (!name) {
        showToast("Template name is required", true);
        return;
      }

      var tags = tagsInput ? tagsInput.value.split(",").map(function (t) { return t.trim(); }).filter(Boolean) : [];
      var target = targetSel ? targetSel.value : "universal";
      var elements = window.QlxFormat.canvasToQlx(fabricCanvas);

      var payload = {
        name: name,
        tags: tags,
        target: target,
        elements: JSON.stringify(elements)
      };

      if (target === "universal") {
        payload.width_mm = parseFloat(widthInput ? widthInput.value : 50) || 50;
        payload.height_mm = parseFloat(heightInput ? heightInput.value : 30) || 30;
      } else {
        payload.width_px = parseInt(widthInput ? widthInput.value : 384) || 384;
        payload.height_px = parseInt(heightInput ? heightInput.value : 240) || 240;
      }

      var method = templateId ? "PUT" : "POST";
      var url = templateId
        ? "/templates/" + templateId
        : "/templates";

      fetch(url, {
        method: method,
        headers: { "Content-Type": "application/json", "Accept": "application/json" },
        body: JSON.stringify(payload)
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (data) {
              throw new Error(data.error || "Save failed");
            });
          }
          return resp.json();
        })
        .then(function () {
          showToast("Template saved");
          if (typeof htmx !== "undefined") {
            htmx.ajax("GET", "/templates", { target: "#content" });
          }
        })
        .catch(function (err) {
          showToast("Save failed: " + err.message, true);
        });
    });
  }

  // --- Helpers ---
  function showToast(msg, isError) {
    if (window.showToast) {
      window.showToast(msg, isError);
    }
  }

  // --- Bootstrap ---
  function tryInit() {
    if (document.getElementById("designer-app")) {
      init();
    }
  }

  // Use htmx.onLoad for proper 3rd party library init after HTMX content swap
  if (typeof htmx !== "undefined") {
    htmx.onLoad(tryInit);
  }

  // Also init on first page load (full page, not HTMX swap)
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", tryInit);
  } else {
    tryInit();
  }
})();

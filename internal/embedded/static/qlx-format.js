window.QlxFormat = (function () {
  var fontMap = {
    "Arial, sans-serif": "sans",
    "Georgia, serif": "serif",
    "Courier New, monospace": "mono"
  };
  var fontMapReverse = {
    "sans": "Arial, sans-serif",
    "serif": "Georgia, serif",
    "mono": "Courier New, monospace"
  };

  function substituteParams(text, params) {
    if (!text || !params) return text || "";
    return text.replace(/\{\{(\w+)\}\}/g, function (match, key) {
      return params.hasOwnProperty(key) ? params[key] : match;
    });
  }

  function fabricFontToQlx(family) {
    if (fontMap[family]) return fontMap[family];
    if (family && family.indexOf("serif") !== -1 && family.indexOf("sans") === -1) return "serif";
    if (family && family.indexOf("mono") !== -1) return "mono";
    return "sans";
  }

  function canvasToQlx(canvas) {
    var elements = [];
    var objects = canvas.getObjects();
    for (var i = 0; i < objects.length; i++) {
      var obj = objects[i];
      var el = objectToElement(obj);
      if (el) elements.push(el);
    }
    return elements;
  }

  function objectToElement(obj) {
    var t = obj.qlxType;
    if (!t) return null;

    if (t === "text") {
      return {
        type: "text",
        x: Math.round(obj.left),
        y: Math.round(obj.top),
        width: Math.round(obj.width),
        height: Math.round(obj.height),
        text: obj.qlxTemplate || obj.text || "",
        font: fabricFontToQlx(obj.fontFamily),
        size: obj.fontSize || 16,
        bold: obj.fontWeight === "bold",
        italic: obj.fontStyle === "italic",
        align: obj.textAlign || "left"
      };
    }

    if (t === "qr") {
      return {
        type: "qr",
        x: Math.round(obj.left),
        y: Math.round(obj.top),
        size: Math.round(obj.getScaledWidth()),
        content: obj.qlxContent || ""
      };
    }

    if (t === "barcode") {
      return {
        type: "barcode",
        x: Math.round(obj.left),
        y: Math.round(obj.top),
        width: Math.round(obj.getScaledWidth()),
        height: Math.round(obj.getScaledHeight()),
        content: obj.qlxContent || "",
        format: "code128"
      };
    }

    if (t === "line") {
      return {
        type: "line",
        x1: Math.round(obj.left + obj.x1),
        y1: Math.round(obj.top + obj.y1),
        x2: Math.round(obj.left + obj.x2),
        y2: Math.round(obj.top + obj.y2),
        thickness: obj.strokeWidth || 1
      };
    }

    if (t === "img") {
      return {
        type: "img",
        x: Math.round(obj.left),
        y: Math.round(obj.top),
        width: Math.round(obj.getScaledWidth()),
        height: Math.round(obj.getScaledHeight()),
        src: obj.qlxSrc || "",
        fit: obj.qlxFit || "contain"
      };
    }

    return null;
  }

  function qlxToCanvas(canvas, elements, params) {
    if (!elements || !elements.length) return Promise.resolve();

    var promises = [];
    for (var i = 0; i < elements.length; i++) {
      promises.push(elementToObject(elements[i], params));
    }

    return Promise.all(promises).then(function (objects) {
      for (var j = 0; j < objects.length; j++) {
        if (objects[j]) canvas.add(objects[j]);
      }
      canvas.renderAll();
    });
  }

  function elementToObject(el, params) {
    var t = el.type;

    if (t === "text") {
      var displayText = params ? substituteParams(el.text, params) : el.text;
      var tb = new fabric.Textbox(displayText, {
        left: el.x || 0,
        top: el.y || 0,
        width: el.width || 100,
        fontSize: el.size || 16,
        fontFamily: fontMapReverse[el.font] || fontMapReverse["sans"],
        fontWeight: el.bold ? "bold" : "normal",
        fontStyle: el.italic ? "italic" : "normal",
        textAlign: el.align || "left",
        fill: "#000000"
      });
      tb.qlxType = "text";
      tb.qlxTemplate = el.text;
      return Promise.resolve(tb);
    }

    if (t === "qr") {
      var content = params ? substituteParams(el.content, params) : el.content;
      return renderQR(content, el.size || 80).then(function (img) {
        img.set({ left: el.x || 0, top: el.y || 0 });
        img.qlxType = "qr";
        img.qlxContent = el.content;
        return img;
      });
    }

    if (t === "barcode") {
      var bcContent = params ? substituteParams(el.content, params) : el.content;
      return renderBarcode(bcContent, el.width || 150, el.height || 50).then(function (img) {
        img.set({ left: el.x || 0, top: el.y || 0 });
        img.qlxType = "barcode";
        img.qlxContent = el.content;
        return img;
      });
    }

    if (t === "line") {
      var x1 = el.x1 || 0, y1 = el.y1 || 0, x2 = el.x2 || 100, y2 = el.y2 || 0;
      var line = new fabric.Line([0, 0, x2 - x1, y2 - y1], {
        left: x1,
        top: y1,
        stroke: "#000000",
        strokeWidth: el.thickness || 1,
        strokeLineCap: "round"
      });
      line.qlxType = "line";
      return Promise.resolve(line);
    }

    if (t === "img") {
      var w = el.width || 80;
      var h = el.height || 80;
      if (el.src && el.src.indexOf("asset:") === 0) {
        var assetId = el.src.substring(6);
        return loadAssetImage(assetId, el.x || 0, el.y || 0, w, h, el.src, el.fit);
      }
      // Placeholder rect
      var rect = new fabric.Rect({
        left: el.x || 0,
        top: el.y || 0,
        width: w,
        height: h,
        fill: "#cccccc",
        stroke: "#999999",
        strokeWidth: 1,
        strokeDashArray: [4, 4]
      });
      rect.qlxType = "img";
      rect.qlxSrc = el.src || "";
      rect.qlxFit = el.fit || "contain";
      return Promise.resolve(rect);
    }

    return Promise.resolve(null);
  }

  function renderQR(content, size) {
    return new Promise(function (resolve) {
      try {
        var qr = qrcode(0, "M");
        qr.addData(content || "https://example.com");
        qr.make();
        var dataUrl = qr.createDataURL(4, 0);
        var imgEl = new Image();
        imgEl.onload = function () {
          var fImg = new fabric.Image(imgEl, {
            scaleX: size / imgEl.width,
            scaleY: size / imgEl.height
          });
          resolve(fImg);
        };
        imgEl.onerror = function () {
          resolve(makePlaceholder(size, size, "QR"));
        };
        imgEl.src = dataUrl;
      } catch (e) {
        resolve(makePlaceholder(size, size, "QR"));
      }
    });
  }

  function renderBarcode(content, width, height) {
    return new Promise(function (resolve) {
      try {
        var tmpCanvas = document.createElement("canvas");
        JsBarcode(tmpCanvas, content || "0000", {
          format: "CODE128",
          width: 2,
          height: height,
          displayValue: false,
          margin: 0
        });
        var imgEl = new Image();
        imgEl.onload = function () {
          var fImg = new fabric.Image(imgEl, {
            scaleX: width / imgEl.width,
            scaleY: height / imgEl.height
          });
          resolve(fImg);
        };
        imgEl.onerror = function () {
          resolve(makePlaceholder(width, height, "BC"));
        };
        imgEl.src = tmpCanvas.toDataURL();
      } catch (e) {
        resolve(makePlaceholder(width, height, "BC"));
      }
    });
  }

  function loadAssetImage(assetId, x, y, w, h, src, fit) {
    return new Promise(function (resolve) {
      var imgEl = new Image();
      imgEl.crossOrigin = "anonymous";
      imgEl.onload = function () {
        var fImg = new fabric.Image(imgEl, {
          left: x,
          top: y,
          scaleX: w / imgEl.width,
          scaleY: h / imgEl.height
        });
        fImg.qlxType = "img";
        fImg.qlxSrc = src;
        fImg.qlxFit = fit || "contain";
        resolve(fImg);
      };
      imgEl.onerror = function () {
        var rect = makePlaceholder(w, h, "IMG");
        rect.set({ left: x, top: y });
        rect.qlxType = "img";
        rect.qlxSrc = src;
        rect.qlxFit = fit || "contain";
        resolve(rect);
      };
      imgEl.src = "/ui/actions/assets/" + assetId;
    });
  }

  function makePlaceholder(w, h, label) {
    var rect = new fabric.Rect({
      width: w,
      height: h,
      fill: "#cccccc",
      stroke: "#999999",
      strokeWidth: 1,
      strokeDashArray: [4, 4]
    });
    rect.qlxType = "img";
    rect.qlxSrc = "";
    rect.qlxFit = "contain";
    return rect;
  }

  return {
    canvasToQlx: canvasToQlx,
    qlxToCanvas: qlxToCanvas,
    substituteParams: substituteParams
  };
})();

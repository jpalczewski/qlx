window.LabelDither = (function () {
  function dither(sourceCanvas) {
    var w = sourceCanvas.width;
    var h = sourceCanvas.height;
    var srcCtx = sourceCanvas.getContext("2d");
    var srcData = srcCtx.getImageData(0, 0, w, h);
    var pixels = srcData.data;

    // Convert to grayscale float array
    var gray = new Float32Array(w * h);
    for (var i = 0; i < w * h; i++) {
      var r = pixels[i * 4];
      var g = pixels[i * 4 + 1];
      var b = pixels[i * 4 + 2];
      var a = pixels[i * 4 + 3];
      // Treat transparent pixels as white
      if (a < 128) {
        gray[i] = 255;
      } else {
        gray[i] = 0.299 * r + 0.587 * g + 0.114 * b;
      }
    }

    // Floyd-Steinberg dithering
    for (var y = 0; y < h; y++) {
      for (var x = 0; x < w; x++) {
        var idx = y * w + x;
        var oldVal = gray[idx];
        var newVal = oldVal < 128 ? 0 : 255;
        gray[idx] = newVal;
        var err = oldVal - newVal;

        if (x + 1 < w) gray[idx + 1] += err * 7 / 16;
        if (y + 1 < h) {
          if (x > 0) gray[(y + 1) * w + (x - 1)] += err * 3 / 16;
          gray[(y + 1) * w + x] += err * 5 / 16;
          if (x + 1 < w) gray[(y + 1) * w + (x + 1)] += err * 1 / 16;
        }
      }
    }

    // Write to new canvas
    var outCanvas = document.createElement("canvas");
    outCanvas.width = w;
    outCanvas.height = h;
    var outCtx = outCanvas.getContext("2d");
    var outData = outCtx.createImageData(w, h);
    var out = outData.data;

    for (var j = 0; j < w * h; j++) {
      var v = gray[j] < 128 ? 0 : 255;
      out[j * 4] = v;
      out[j * 4 + 1] = v;
      out[j * 4 + 2] = v;
      out[j * 4 + 3] = 255;
    }

    outCtx.putImageData(outData, 0, 0);
    return outCanvas;
  }

  return { dither: dither };
})();

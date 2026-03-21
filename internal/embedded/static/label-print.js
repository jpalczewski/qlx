window.LabelPrint = (function () {
  function print(canvas, printerId, multiplier) {
    multiplier = multiplier || 2;

    return new Promise(function (resolve, reject) {
      // Export canvas to PNG data URL
      var dataUrl = canvas.toDataURL({ format: "png", multiplier: multiplier });

      // Load into temp image to draw onto a clean canvas
      var img = new Image();
      img.onload = function () {
        var tmpCanvas = document.createElement("canvas");
        tmpCanvas.width = img.width;
        tmpCanvas.height = img.height;
        var ctx = tmpCanvas.getContext("2d");
        ctx.drawImage(img, 0, 0);

        // Apply Floyd-Steinberg dithering
        var dithered = window.LabelDither.dither(tmpCanvas);
        var ditheredUrl = dithered.toDataURL("image/png");

        // POST to server
        fetch("/ui/actions/print-image", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            printer_id: printerId,
            png: ditheredUrl
          })
        })
          .then(function (resp) {
            if (!resp.ok) {
              return resp.json().then(function (data) {
                throw new Error(data.error || "Print failed: " + resp.status);
              });
            }
            return resp.json();
          })
          .then(resolve)
          .catch(reject);
      };
      img.onerror = function () {
        reject(new Error("Failed to load canvas image"));
      };
      img.src = dataUrl;
    });
  }

  return { print: print };
})();

window.LabelParams = (function () {
  function buildContext(entity, printerName) {
    var now = new Date();
    var date = now.getFullYear() + "-" +
      String(now.getMonth() + 1).padStart(2, "0") + "-" +
      String(now.getDate()).padStart(2, "0");
    var time = String(now.getHours()).padStart(2, "0") + ":" +
      String(now.getMinutes()).padStart(2, "0");

    return {
      name: (entity && entity.name) || "",
      description: (entity && entity.description) || "",
      location: (entity && entity.location) || "",
      id: (entity && entity.id) || "",
      qr_url: (entity && entity.qr_url) || "",
      date: date,
      time: time,
      printer: printerName || ""
    };
  }

  function substitute(text, params) {
    if (!text || !params) return text || "";
    return text.replace(/\{\{(\w+)\}\}/g, function (match, key) {
      return params.hasOwnProperty(key) ? params[key] : match;
    });
  }

  function extractParams(elements) {
    var found = {};
    if (!elements) return [];
    for (var i = 0; i < elements.length; i++) {
      var el = elements[i];
      var fields = [el.text, el.content];
      for (var j = 0; j < fields.length; j++) {
        if (!fields[j]) continue;
        var re = /\{\{(\w+)\}\}/g;
        var m;
        while ((m = re.exec(fields[j])) !== null) {
          found[m[1]] = true;
        }
      }
    }
    return Object.keys(found);
  }

  return {
    buildContext: buildContext,
    substitute: substitute,
    extractParams: extractParams
  };
})();

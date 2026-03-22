
(function () {
  var qlx = window.qlx = window.qlx || {};

  /** @type {Record<string, string>} */
  var translations = {};
  var loaded = false;

  /**
   * Return the translated string for the given key, or the key itself as fallback.
   * @param {string} key
   * @returns {string}
   */
  qlx.t = function t(key) {
    return translations[key] || key;
  };

  /** Fetch translations from the server and populate the map. */
  function loadTranslations() {
    var lang = document.documentElement.lang || "en";
    fetch("/i18n/" + encodeURIComponent(lang))
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data && typeof data === "object") {
          translations = data;
          loaded = true;
        }
      })
      .catch(function (err) {
        console.error("i18n load failed:", err);
      });
  }

  loadTranslations();
})();

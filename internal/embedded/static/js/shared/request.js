(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Re-fetch a page into #content via HTMX.
   * @param {string} [url] - defaults to the current pathname
   */
  qlx.reloadContent = function reloadContent(url) {
    htmx.ajax("GET", url || window.location.pathname, { target: "#content" });
  };

  /**
   * Send a mutating request and handle the shared outcome: an error toast on
   * a non-2xx response (server message or status), a connection-error toast
   * on network failure, and on success a toast, an optional callback and a
   * content reload.
   *
   * @param {string} url
   * @param {Object} opts
   * @param {string} [opts.method] - HTTP method, default "POST"
   * @param {Object} [opts.json] - JSON request body
   * @param {string} [opts.form] - urlencoded request body (used when opts.json is absent)
   * @param {string} [opts.success] - success toast message
   * @param {function(): void} [opts.afterSuccess] - runs before the reload
   * @param {string|false} [opts.reloadUrl] - page to reload; default current pathname, false to skip
   * @param {string} [opts.logLabel] - prefix for the console error on network failure
   * @returns {Promise<void>}
   */
  qlx.bulkRequest = function bulkRequest(url, opts) {
    var headers = {};
    var body;
    if (opts.json !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(opts.json);
    } else {
      headers["Content-Type"] = "application/x-www-form-urlencoded";
      body = opts.form;
    }
    return fetch(url, { method: opts.method || "POST", headers: headers, body: body })
      .then(function (resp) {
        if (!resp.ok) {
          return resp.json().then(function (data) {
            qlx.showToast(data.error || qlx.t("error.status") + " " + resp.status, true);
          });
        }
        if (opts.success) qlx.showToast(opts.success, false);
        if (opts.afterSuccess) opts.afterSuccess();
        if (opts.reloadUrl !== false) qlx.reloadContent(opts.reloadUrl);
      })
      .catch(function (err) {
        console.error((opts.logLabel || "request failed") + ":", err);
        qlx.showToast(qlx.t("error.connection"), true);
      });
  };
})();

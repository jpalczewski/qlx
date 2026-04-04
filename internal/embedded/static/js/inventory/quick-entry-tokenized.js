(function () {
  "use strict";

  var qlx = window.qlx = window.qlx || {};

  // TokenizedQuickEntry manages a contenteditable div as a rich token input
  // supporting @container and #tag tokens with autocomplete dropdowns.
  qlx.TokenizedQuickEntry = function (opts) {
    var el = opts.el;
    if (!el) return;

    var input = el.querySelector(".qe-input");
    var toggleBtn = el.querySelector(".qe-type-toggle");
    var submitBtn = el.querySelector(".qe-submit");

    if (!input) return;

    // Read prefill and default container data from element dataset
    var prefillContainerID   = el.dataset.prefillContainerId   || "";
    var prefillContainerName = el.dataset.prefillContainerName || "";
    var prefillContainerIcon = el.dataset.prefillContainerIcon || "";
    var defaultContainerID   = el.dataset.defaultContainerId   || "";

    // mode: 'item' or 'container'
    var mode = "item";

    // Active autocomplete instances
    var containerAC = null;
    var tagAC = null;

    // Current trigger state detected in the contenteditable
    var activeTrigger = null;

    // ------------------------------------------------------------------
    // Token creation
    // ------------------------------------------------------------------

    function createToken(type, id, name, icon, isDefault) {
      var span = document.createElement("span");
      span.contentEditable = "false";
      span.className = "qe-token qe-token--" + type + (isDefault ? " qe-token--default" : "");
      span.dataset.tokenType = type;
      span.dataset.tokenId = id;

      if (icon) {
        var iconEl = document.createElement("i");
        iconEl.className = "ph ph-" + icon;
        span.appendChild(iconEl);
      }

      var label = document.createElement("span");
      label.textContent = (type === "container" ? "@" : "#") + name;
      span.appendChild(label);

      var rm = document.createElement("span");
      rm.className = "qe-token-remove";
      rm.textContent = "\u00D7"; // x
      rm.addEventListener("mousedown", function (e) {
        e.preventDefault();
        removeToken(span);
      });
      span.appendChild(rm);

      return span;
    }

    // ------------------------------------------------------------------
    // Token removal
    // ------------------------------------------------------------------

    function removeToken(span) {
      var wasContainer = span.dataset.tokenType === "container";
      if (span.parentNode) span.parentNode.removeChild(span);
      if (wasContainer) {
        restoreDefaultContainerToken();
      }
    }

    function removeExistingContainerToken() {
      var existing = input.querySelector("[data-token-type='container']");
      if (existing && existing.parentNode) existing.parentNode.removeChild(existing);
    }

    function restoreDefaultContainerToken() {
      if (!prefillContainerID) return;
      removeExistingContainerToken();
      var t = createToken("container", prefillContainerID, prefillContainerName, prefillContainerIcon, true);
      input.insertBefore(t, input.firstChild);
      var nbsp = document.createTextNode("\u00A0");
      if (t.nextSibling) {
        input.insertBefore(nbsp, t.nextSibling);
      } else {
        input.appendChild(nbsp);
      }
    }

    // ------------------------------------------------------------------
    // Trigger detection
    // ------------------------------------------------------------------

    function detectTrigger() {
      var sel = window.getSelection();
      if (!sel || !sel.rangeCount || !input.contains(sel.anchorNode)) return null;
      var node = sel.anchorNode;
      if (node.nodeType !== Node.TEXT_NODE) return null;
      var text = node.textContent;
      var offset = sel.anchorOffset;
      var before = text.substring(0, offset);
      // Find last @ or # at word boundary (start, space, or NBSP)
      var match = before.match(/(?:^|[\s\u00A0])([@#])([^\s\u00A0]*)$/);
      if (!match) return null;
      return {
        char: match[1],
        query: match[2],
        textNode: node,
        triggerStart: before.lastIndexOf(match[1])
      };
    }

    // ------------------------------------------------------------------
    // Remove trigger text from contenteditable before inserting token
    // ------------------------------------------------------------------

    function removeTriggerText(state) {
      var sel = window.getSelection();
      var endOffset = sel ? sel.anchorOffset : state.triggerStart + 1 + state.query.length;
      var text = state.textNode.textContent;
      state.textNode.textContent = text.substring(0, state.triggerStart) + text.substring(endOffset);
    }

    // ------------------------------------------------------------------
    // Insert token at cursor (after removing trigger text)
    // ------------------------------------------------------------------

    function insertTokenAtTrigger(token, triggerState) {
      removeTriggerText(triggerState);

      var textNode = triggerState.textNode;
      var parent = textNode.parentNode;
      if (!parent) {
        // Fallback: append to input
        input.appendChild(token);
        input.appendChild(document.createTextNode("\u00A0"));
        return;
      }

      // Split the text node at triggerStart and insert token between the halves
      var before = textNode.textContent.substring(0, triggerState.triggerStart);
      var after = textNode.textContent.substring(triggerState.triggerStart);

      var beforeNode = document.createTextNode(before);
      var afterNode = document.createTextNode("\u00A0" + after);

      parent.insertBefore(beforeNode, textNode);
      parent.insertBefore(token, textNode);
      parent.insertBefore(afterNode, textNode);
      parent.removeChild(textNode);

      // Place caret after the NBSP that follows the token
      var range = document.createRange();
      range.setStart(afterNode, 1);
      range.collapse(true);
      var sel = window.getSelection();
      if (sel) {
        sel.removeAllRanges();
        sel.addRange(range);
      }
    }

    // ------------------------------------------------------------------
    // Autocomplete lifecycle
    // ------------------------------------------------------------------

    function closeAllAC() {
      if (containerAC) { containerAC.close(); containerAC = null; }
      if (tagAC) { tagAC.close(); tagAC = null; }
      activeTrigger = null;
    }

    function openContainerAC(triggerState) {
      if (tagAC) { tagAC.close(); tagAC = null; }
      if (!containerAC) {
        containerAC = qlx.ContainerAutocomplete({
          anchor: input,
          onSelect: function (container) {
            var savedTrigger = activeTrigger || triggerState;
            containerAC = null;
            activeTrigger = null;
            removeExistingContainerToken();
            var t = createToken("container", container.id, container.name, container.icon || "", false);
            insertTokenAtTrigger(t, savedTrigger);
          },
          onCancel: function () {
            containerAC = null;
            activeTrigger = null;
          }
        });
      }
      containerAC.update(triggerState.query);
      activeTrigger = triggerState;
    }

    function openTagAC(triggerState) {
      if (containerAC) { containerAC.close(); containerAC = null; }
      if (!tagAC) {
        tagAC = qlx.TagAutocomplete({
          anchor: input,
          onSelect: function (tag) {
            var savedTrigger = activeTrigger || triggerState;
            tagAC = null;
            activeTrigger = null;
            var t = createToken("tag", tag.id, tag.name, "", false);
            insertTokenAtTrigger(t, savedTrigger);
          },
          onCancel: function () {
            tagAC = null;
            activeTrigger = null;
          }
        });
      }
      tagAC.update(triggerState.query);
      activeTrigger = triggerState;
    }

    // ------------------------------------------------------------------
    // Parse contenteditable for submit
    // ------------------------------------------------------------------

    function parseInput() {
      var containerID = "";
      var tagIDs = [];
      var textParts = [];

      input.childNodes.forEach(function (node) {
        if (node.nodeType === Node.ELEMENT_NODE && node.classList.contains("qe-token")) {
          var type = node.dataset.tokenType;
          var id = node.dataset.tokenId;
          if (type === "container") containerID = id;
          else if (type === "tag") tagIDs.push(id);
        } else if (node.nodeType === Node.TEXT_NODE) {
          var t = node.textContent.replace(/\u00A0/g, " ").trim();
          if (t) textParts.push(t);
        }
      });

      var fullText = textParts.join(" ").trim();
      var qty = 1;

      if (mode === "item") {
        var qm = fullText.match(/\bx(\d+)\b/);
        if (qm && parseInt(qm[1], 10) > 0) {
          qty = parseInt(qm[1], 10);
          fullText = fullText.replace(qm[0], "").replace(/\s+/g, " ").trim();
        }
      }

      return { name: fullText, containerID: containerID, tagIDs: tagIDs, qty: qty };
    }

    // ------------------------------------------------------------------
    // Submit
    // ------------------------------------------------------------------

    function submit() {
      var data = parseInput();
      if (!data.name) return;

      var form = new FormData();
      form.append("name", data.name);
      data.tagIDs.forEach(function (id) { form.append("tag_ids", id); });

      var url, target;
      if (mode === "item") {
        url = "/items";
        target = "item-list";
        form.append("container_id", data.containerID || defaultContainerID);
        form.append("quantity", String(data.qty));
      } else {
        url = "/containers";
        target = "container-list";
        form.append("parent_id", data.containerID || defaultContainerID);
      }

      fetch(url, {
        method: "POST",
        headers: { "HX-Request": "true", "HX-Target": target },
        body: form
      }).then(function (resp) {
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        return resp.text();
      }).then(function (html) {
        var list = document.getElementById(target);
        if (list) {
          // Parse server-generated HTML fragment using DOMParser (trusted server response)
          var doc = (new DOMParser()).parseFromString(html, "text/html");
          var children = Array.prototype.slice.call(doc.body.childNodes);
          children.forEach(function (child) {
            list.appendChild(document.adoptNode(child));
          });
        }
        if (qlx.invalidateContainerCache) qlx.invalidateContainerCache();
        resetInput();
      }).catch(function (err) {
        console.error("quick-entry submit failed:", err);
      });
    }

    // ------------------------------------------------------------------
    // Reset
    // ------------------------------------------------------------------

    function resetInput() {
      input.textContent = "";
      if (prefillContainerID) {
        var t = createToken("container", prefillContainerID, prefillContainerName, prefillContainerIcon, true);
        input.appendChild(t);
        input.appendChild(document.createTextNode("\u00A0"));
      }
      input.focus();
    }

    // ------------------------------------------------------------------
    // Mode toggle (Tab key or toggle button)
    // ------------------------------------------------------------------

    function toggleMode() {
      if (mode === "item") {
        mode = "container";
        if (toggleBtn) toggleBtn.textContent = "\uD83D\uDCE6"; // 📦
        input.dataset.placeholder = "Nowy kontener... (\u21B5)";
      } else {
        mode = "item";
        if (toggleBtn) toggleBtn.textContent = "\uD83C\uDFF7"; // 🏷
        input.dataset.placeholder = "Nowy item... (x5 #tag \u21B5)";
      }
    }

    // ------------------------------------------------------------------
    // Event: input (contenteditable)
    // ------------------------------------------------------------------

    input.addEventListener("input", function () {
      var trigger = detectTrigger();
      if (!trigger) {
        closeAllAC();
        return;
      }

      // Update activeTrigger as user types (query expands)
      activeTrigger = trigger;

      if (trigger.char === "@") {
        if (containerAC) {
          containerAC.update(trigger.query);
        } else {
          openContainerAC(trigger);
        }
      } else if (trigger.char === "#") {
        if (tagAC) {
          tagAC.update(trigger.query);
        } else {
          openTagAC(trigger);
        }
      }
    });

    // ------------------------------------------------------------------
    // Event: keydown (contenteditable)
    // ------------------------------------------------------------------

    input.addEventListener("keydown", function (e) {
      // Let active autocomplete handle navigation keys first
      if (containerAC && containerAC.isOpen()) {
        if (containerAC.onKeydown(e)) return;
      }
      if (tagAC && tagAC.isOpen()) {
        if (tagAC.onKeydown(e)) return;
      }

      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        closeAllAC();
        submit();
        return;
      }

      if (e.key === "Escape") {
        e.preventDefault();
        closeAllAC();
        return;
      }

      if (e.key === "Tab") {
        e.preventDefault();
        toggleMode();
        return;
      }

      if (e.key === "Backspace") {
        // Remove entire token if caret is immediately after one
        var sel = window.getSelection();
        if (sel && sel.rangeCount) {
          var range = sel.getRangeAt(0);
          if (range.collapsed) {
            var node = range.startContainer;
            var offset = range.startOffset;
            var prevSibling = null;
            if (node.nodeType === Node.TEXT_NODE && offset === 0 && node.previousSibling) {
              prevSibling = node.previousSibling;
            } else if (node.nodeType === Node.ELEMENT_NODE && offset > 0) {
              prevSibling = node.childNodes[offset - 1];
            }
            if (prevSibling && prevSibling.nodeType === Node.ELEMENT_NODE &&
                prevSibling.classList && prevSibling.classList.contains("qe-token")) {
              e.preventDefault();
              removeToken(prevSibling);
            }
          }
        }
      }
    });

    // ------------------------------------------------------------------
    // Toggle button and submit button click events
    // ------------------------------------------------------------------

    if (toggleBtn) {
      toggleBtn.addEventListener("click", function () {
        toggleMode();
        input.focus();
      });
    }

    if (submitBtn) {
      submitBtn.addEventListener("click", function () {
        closeAllAC();
        submit();
      });
    }

    // ------------------------------------------------------------------
    // Initialize
    // ------------------------------------------------------------------

    resetInput();
  };
})();


(function () {
  var qlx = window.qlx = window.qlx || {};

  /**
   * Replace the visual content of a note card with an inline edit form.
   * Uses safe DOM methods only — no innerHTML.
   *
   * @param {HTMLElement} card  The .note-card element.
   */
  qlx.editNote = function editNote(card) {
    if (!card) return;
    var noteID = card.dataset.noteId;
    if (!noteID) return;

    // Snapshot children so we can restore on cancel
    var savedChildren = [];
    var children = card.childNodes;
    for (var i = 0; i < children.length; i++) {
      savedChildren.push(children[i].cloneNode(true));
    }

    // Clear card children
    while (card.firstChild) {
      card.removeChild(card.firstChild);
    }

    // Build form with safe DOM methods
    var form = document.createElement("form");
    form.className = "note-edit-form";

    // Title input
    var titleInput = document.createElement("input");
    titleInput.type = "text";
    titleInput.name = "title";
    titleInput.className = "note-edit-title";
    titleInput.value = card.dataset.noteTitle || "";
    titleInput.required = true;
    form.appendChild(titleInput);

    // Content textarea
    var contentArea = document.createElement("textarea");
    contentArea.name = "content";
    contentArea.className = "note-edit-content";
    contentArea.textContent = card.dataset.noteContent || "";
    form.appendChild(contentArea);

    // Footer with Save and Cancel
    var footer = document.createElement("div");
    footer.className = "note-edit-footer";

    var cancelBtn = document.createElement("button");
    cancelBtn.type = "button";
    cancelBtn.className = "btn btn-secondary btn-small";
    cancelBtn.textContent = "Cancel";
    cancelBtn.addEventListener("click", function () {
      restoreCard(card, savedChildren);
    });
    footer.appendChild(cancelBtn);

    var saveBtn = document.createElement("button");
    saveBtn.type = "submit";
    saveBtn.className = "btn btn-primary btn-small";
    saveBtn.textContent = "Save";
    footer.appendChild(saveBtn);

    form.appendChild(footer);

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      submitNoteUpdate(card, noteID, titleInput.value, contentArea.value, savedChildren);
    });

    card.appendChild(form);
    titleInput.focus();
    titleInput.select();
  };

  /**
   * Submit a PUT /notes/{id} request and update the card data attributes.
   *
   * @param {HTMLElement} card
   * @param {string} noteID
   * @param {string} title
   * @param {string} content
   * @param {Node[]} savedChildren
   */
  function submitNoteUpdate(card, noteID, title, content, savedChildren) {
    fetch("/notes/" + noteID, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title: title, content: content })
    })
      .then(function (resp) {
        if (!resp.ok) {
          throw new Error("HTTP " + resp.status);
        }
        return resp.json();
      })
      .then(function (note) {
        // Update data attributes with new values
        card.dataset.noteTitle = note.title || "";
        card.dataset.noteContent = note.content || "";

        // Update the saved children's title and content text nodes
        // by rebuilding a fresh snapshot from the server response
        updateSavedTitle(savedChildren, note.title || "");
        updateSavedContent(savedChildren, note.content || "");

        restoreCard(card, savedChildren);

        if (typeof qlx.showToast === "function") {
          qlx.showToast("Note saved", false);
        }
      })
      .catch(function (err) {
        console.error("note update failed:", err);
        if (typeof qlx.showToast === "function") {
          qlx.showToast("Failed to save note", true);
        }
      });
  }

  /**
   * Restore saved child nodes back into the card, replacing the edit form.
   *
   * @param {HTMLElement} card
   * @param {Node[]} savedChildren
   */
  function restoreCard(card, savedChildren) {
    while (card.firstChild) {
      card.removeChild(card.firstChild);
    }
    for (var i = 0; i < savedChildren.length; i++) {
      card.appendChild(savedChildren[i].cloneNode(true));
    }
  }

  /**
   * Find the .note-card-title element in a list of nodes and update its text.
   *
   * @param {Node[]} nodes
   * @param {string} text
   */
  function updateSavedTitle(nodes, text) {
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      if (node.nodeType === 1 && node.classList && node.classList.contains("note-card-title")) {
        node.textContent = text;
        return;
      }
    }
  }

  /**
   * Find the .note-card-content element in a list of nodes and update its text.
   *
   * @param {Node[]} nodes
   * @param {string} text
   */
  function updateSavedContent(nodes, text) {
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      if (node.nodeType === 1 && node.classList && node.classList.contains("note-card-content")) {
        node.textContent = text;
        return;
      }
    }
  }

  /**
   * Update the notes badge count when a notes-changed event fires.
   * The HX-Trigger response header "notes-changed" is sent by the server
   * after create/delete operations to keep the tab badge in sync.
   *
   * Looks for .tab-badge[data-tab-badge="notes"] within the same
   * .tab-container as the triggered element.
   *
   * @param {number} count  New note count.
   */
  function updateNotesBadge(count) {
    var badges = document.querySelectorAll(".tab-badge[data-tab-badge=\"notes\"]");
    for (var i = 0; i < badges.length; i++) {
      badges[i].textContent = String(count);
    }
  }

  // Listen for htmx:afterRequest and check for notes-changed trigger header
  document.body.addEventListener("htmx:afterRequest", function (event) {
    var detail = event.detail;
    if (!detail || !detail.successful) return;

    var xhr = detail.xhr;
    if (!xhr) return;

    var trigger = xhr.getResponseHeader("HX-Trigger");
    if (!trigger) return;

    // HX-Trigger may be a plain string or JSON object
    var notesChanged = false;
    var newCount = null;

    if (trigger.indexOf("{") === 0) {
      try {
        var parsed = JSON.parse(trigger);
        if (parsed["notes-changed"] !== undefined) {
          notesChanged = true;
          newCount = parsed["notes-changed"];
        }
      } catch (e) {
        // not valid JSON, ignore
      }
    } else if (trigger.indexOf("notes-changed") !== -1) {
      notesChanged = true;
    }

    if (!notesChanged) return;

    // If count was embedded in the trigger value, update badge directly
    if (newCount !== null && typeof newCount === "number") {
      updateNotesBadge(newCount);
      return;
    }

    // Otherwise count visible note cards in the list as a fallback
    var list = document.querySelector(".note-list");
    if (list) {
      updateNotesBadge(list.querySelectorAll(".note-card").length);
    }
  });
})();

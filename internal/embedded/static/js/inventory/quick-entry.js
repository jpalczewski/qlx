// Quick-entry collapsible description
document.addEventListener("click", function (e) {
    var trigger = e.target.closest(".quick-entry-desc-trigger");
    if (!trigger) return;
    e.preventDefault();
    var form = trigger.closest(".quick-entry");
    toggleDesc(form);
});

document.addEventListener("keydown", function (e) {
    // Enter/Space on trigger
    if ((e.key === "Enter" || e.key === " ") && e.target.closest(".quick-entry-desc-trigger")) {
        e.preventDefault();
        var form = e.target.closest(".quick-entry");
        toggleDesc(form);
        return;
    }
    // Escape on textarea or trigger — collapse
    if (e.key === "Escape") {
        var form = e.target.closest(".quick-entry");
        if (!form || !form.hasAttribute("data-desc-open")) return;
        if (e.target.matches(".quick-entry-desc-body textarea") || e.target.closest(".quick-entry-desc-trigger")) {
            collapseDesc(form);
        }
    }
});

function toggleDesc(form) {
    if (form.hasAttribute("data-desc-open")) {
        collapseDesc(form);
    } else {
        expandDesc(form);
    }
}

function expandDesc(form) {
    form.setAttribute("data-desc-open", "");
    var textarea = form.querySelector(".quick-entry-desc-body textarea");
    if (textarea) textarea.focus();
}

function collapseDesc(form) {
    form.removeAttribute("data-desc-open");
    var trigger = form.querySelector(".quick-entry-desc-trigger");
    if (trigger) trigger.focus();
}

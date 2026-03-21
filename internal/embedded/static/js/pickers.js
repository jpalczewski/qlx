document.addEventListener("click", function (e) {
    // Color swatch selection
    var colorSwatch = e.target.closest(".color-swatch");
    if (colorSwatch) {
        var grid = colorSwatch.closest("[data-picker='color']");
        if (!grid) return;
        var hidden = grid.parentElement.querySelector("input[name='color']");
        grid.querySelectorAll(".color-swatch").forEach(function (s) {
            s.classList.remove("selected");
        });
        colorSwatch.classList.add("selected");
        if (hidden) hidden.value = colorSwatch.getAttribute("data-value");
        return;
    }

    // Icon category header toggle
    var header = e.target.closest(".icon-picker-category-header");
    if (header) {
        header.parentElement.classList.toggle("open");
        return;
    }

    // Icon swatch selection
    var iconSwatch = e.target.closest(".icon-swatch");
    if (iconSwatch) {
        var container = iconSwatch.closest("[data-picker='icon']");
        if (!container) return;
        var iconHidden = container.parentElement.querySelector("input[name='icon']");
        container.querySelectorAll(".icon-swatch").forEach(function (s) {
            s.classList.remove("selected");
        });
        iconSwatch.classList.add("selected");
        if (iconHidden) iconHidden.value = iconSwatch.getAttribute("data-value");
        return;
    }
});

document.addEventListener("DOMContentLoaded", function () {
    // Color picker
    document.querySelectorAll("[data-picker='color']").forEach(function (grid) {
        var hidden = grid.parentElement.querySelector("input[name='color']");
        grid.addEventListener("click", function (e) {
            var swatch = e.target.closest(".color-swatch");
            if (!swatch) return;
            grid.querySelectorAll(".color-swatch").forEach(function (s) {
                s.classList.remove("selected");
            });
            swatch.classList.add("selected");
            hidden.value = swatch.getAttribute("data-value");
        });
    });

    // Icon picker
    document.querySelectorAll("[data-picker='icon']").forEach(function (container) {
        var hidden = container.parentElement.querySelector("input[name='icon']");

        // Category toggle
        container.querySelectorAll(".icon-picker-category-header").forEach(function (header) {
            header.addEventListener("click", function () {
                header.parentElement.classList.toggle("open");
            });
        });

        // Icon selection
        container.addEventListener("click", function (e) {
            var swatch = e.target.closest(".icon-swatch");
            if (!swatch) return;
            container.querySelectorAll(".icon-swatch").forEach(function (s) {
                s.classList.remove("selected");
            });
            swatch.classList.add("selected");
            hidden.value = swatch.getAttribute("data-value");
        });
    });
});

# Quick-Entry Description Field

**Date:** 2026-03-22
**Status:** Approved

## Problem

The quick-entry forms for adding containers and items only capture `name` (and `quantity` for items). The `description` field is fully supported in the data model, service layer, and handlers, but the only way to set it during creation is through the `<details>` forms in the "actions" section — a separate, less discoverable UI surface.

## Solution

Add a collapsible description textarea to both quick-entry forms (container and item), accessible via Tab navigation. The description section is hidden by default and expands with animation when activated.

## UX Specification

### Layout

**Container quick-entry:**
```
┌─────────────────────────────────────────────────┐
│ + [Name_____________________________] [↵]       │
│   ▾ Description                                 │  ← trigger
│   ┌───────────────────────────────────────────┐  │
│   │ textarea (2 rows)                         │  │  ← expanded
│   └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

**Item quick-entry:**
```
┌─────────────────────────────────────────────────┐
│ + [Name_____________________________] [qty] [↵] │
│   ▾ Description                                 │  ← trigger
│   ┌───────────────────────────────────────────┐  │
│   │ textarea (2 rows)                         │  │  ← expanded
│   └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

### Tab Order

- Container: `name` → trigger → (textarea if expanded) → submit
- Item: `name` → `quantity` → trigger → (textarea if expanded) → submit

### Interactions

| Action | Behavior |
|--------|----------|
| Tab to trigger + Enter/Space | Expands description, focus moves to textarea |
| Click trigger | Same as above |
| Escape on textarea or trigger | Collapses description, focus returns to trigger |
| Form submit (success) | Form resets (name, quantity, textarea cleared), but expanded state is **preserved**. Focus returns to name input |
| Enter in textarea | Inserts newline (default textarea behavior), does NOT submit |

### State Persistence

A `data-desc-open` attribute on the `<form>` element tracks whether the description section is expanded. On successful form reset, the JS checks this attribute and keeps the section open if it was previously expanded. Rationale: if the user expanded the description, they likely want to enter descriptions for subsequent items too.

## Implementation

### Files to Modify

| File | Change |
|------|--------|
| `internal/embedded/templates/pages/inventory/containers.html` | Add description block to both quick-entry forms (container ~L33, item ~L64). Remove redundant `<details>` forms in actions section (L146-164) |
| `internal/embedded/static/css/inventory/quick-entry.css` | Styles for trigger button, collapsible wrapper, textarea, expand/collapse animation |
| `internal/embedded/static/js/quick-entry.js` | **New file.** Toggle logic, Escape handling, state preservation after form reset |

### Files NOT Modified

- `internal/ui/handlers.go` — `HandleContainerCreate` and `HandleItemCreate` already read `description` from form values
- `internal/service/inventory.go` — already validates description (optional, max 500 chars)
- `internal/store/models.go` — `Description` field already exists on both structs

### HTML Structure (per quick-entry form)

```html
<form class="quick-entry" ...>
    <!-- existing fields row -->
    <input type="hidden" name="parent_id" value="...">
    <span class="quick-entry-icon">+</span>
    <input type="text" name="name" placeholder="..." required>
    <button type="submit" class="quick-entry-submit">↵</button>

    <!-- new: collapsible description -->
    <div class="quick-entry-desc">
        <button type="button" class="quick-entry-desc-trigger" tabindex="0">
            <span class="quick-entry-desc-arrow">▸</span> Description
        </button>
        <div class="quick-entry-desc-body">
            <textarea name="description" rows="2" placeholder="Optional description..."></textarea>
        </div>
    </div>
</form>
```

The `.quick-entry` form changes from `display: flex` (single row) to `display: flex; flex-wrap: wrap` so the description block can sit on a new line below the main fields row. The description block gets `width: 100%` to force it onto its own line.

### CSS Animation

```css
.quick-entry-desc-body {
    max-height: 0;
    overflow: hidden;
    transition: max-height 0.2s ease;
}
.quick-entry[data-desc-open] .quick-entry-desc-body {
    max-height: 6rem; /* enough for 2-row textarea + padding */
}
.quick-entry[data-desc-open] .quick-entry-desc-arrow {
    transform: rotate(90deg);
}
```

### JS Logic (`quick-entry.js`)

1. **Toggle**: Click or Enter/Space on `.quick-entry-desc-trigger` toggles `data-desc-open` on the parent form. On expand, focus the textarea. On collapse, focus the trigger.
2. **Escape**: Keydown on textarea or trigger — if Escape, collapse and focus trigger.
3. **Form reset hook**: Modify the `hx-on::after-request` handler to preserve `data-desc-open` state across resets. After `this.reset()`, re-apply `data-desc-open` if it was set.

### Removing Redundant UI

The `<details>` forms in the `<section class="actions">` block (containers.html L146-164) provide name + description entry — the same capability now available in quick-entry. These should be removed to avoid duplicate creation paths.

## E2E Tests

New Playwright tests in `e2e/tests/`:
- Tab from name (container) / quantity (item) reaches description trigger
- Enter/Space on trigger expands textarea
- Escape collapses textarea
- Submit with description creates item/container with description set
- Submit without expanding creates item/container with empty description
- Expanded state persists after successful submit

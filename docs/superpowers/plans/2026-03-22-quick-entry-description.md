# Quick-Entry Description Field Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a collapsible description textarea to quick-entry forms for containers and items, accessible via Tab, with expand/collapse animation and state persistence.

**Architecture:** Pure UI change. The description field already exists in the data model, service validation, and handlers. We add HTML markup (collapsible div inside each quick-entry form), CSS (animation, trigger styles), and JS (toggle, Escape, state persistence). We also remove the now-redundant `<details>` creation forms in the actions section.

**Tech Stack:** Go HTML templates, vanilla JS, CSS transitions, HTMX, Playwright E2E

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/embedded/static/js/inventory/quick-entry.js` | Create | Toggle logic, Escape handling, state preservation after HTMX reset |
| `internal/embedded/static/css/inventory/quick-entry.css` | Modify | Add styles for trigger, collapsible body, textarea, arrow animation |
| `internal/embedded/templates/pages/inventory/containers.html` | Modify | Add description block to both quick-entry forms, remove `<details>` actions section |
| `internal/embedded/templates/layouts/base.html` | Modify | Add `<script>` tag for new `quick-entry.js` |
| `e2e/tests/quick-entry-description.spec.ts` | Create | E2E tests for expand/collapse, keyboard nav, submit with description |

---

### Task 1: Create `quick-entry.js` with toggle logic

**Files:**
- Create: `internal/embedded/static/js/inventory/quick-entry.js`

- [ ] **Step 1: Create the JS file with toggle, Escape, and state-preservation logic**

```js
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/static/js/inventory/quick-entry.js
git commit -m "feat(ui): add quick-entry description toggle JS"
```

---

### Task 2: Register `quick-entry.js` in base layout

**Files:**
- Modify: `internal/embedded/templates/layouts/base.html:44`

- [ ] **Step 1: Add script tag after the last JS include (line 44: `pickers.js`)**

Add this line after line 44:
```html
    <script src="/static/js/inventory/quick-entry.js" defer></script>
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/templates/layouts/base.html
git commit -m "feat(ui): register quick-entry.js in base layout"
```

---

### Task 3: Add CSS for collapsible description

**Files:**
- Modify: `internal/embedded/static/css/inventory/quick-entry.css`

- [ ] **Step 1: Add styles for flex-wrap, trigger, collapsible body, and arrow**

Append to the end of `quick-entry.css`:

```css
/* Collapsible description */
.quick-entry {
    flex-wrap: wrap;
}
.quick-entry-desc {
    width: 100%;
    padding-left: 1.7rem; /* align with input (icon width + gap) */
}
.quick-entry-desc-trigger {
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: 0.8rem;
    padding: 0.25rem 0;
    display: flex;
    align-items: center;
    gap: 0.25rem;
}
.quick-entry-desc-trigger:hover,
.quick-entry-desc-trigger:focus {
    color: var(--color-text);
    outline: none;
}
.quick-entry-desc-arrow {
    display: inline-block;
    transition: transform 0.2s ease;
    font-size: 0.7rem;
}
.quick-entry[data-desc-open] .quick-entry-desc-arrow {
    transform: rotate(90deg);
}
.quick-entry-desc-body {
    max-height: 0;
    overflow: hidden;
    transition: max-height 0.2s ease;
}
.quick-entry[data-desc-open] .quick-entry-desc-body {
    max-height: 6rem;
}
.quick-entry-desc-body textarea {
    width: 100%;
    padding: 0.4rem;
    border: 1px solid var(--color-border);
    border-radius: 4px;
    background: var(--color-bg);
    color: var(--color-text);
    font-size: 0.85rem;
    resize: vertical;
    margin-top: 0.25rem;
    font-family: inherit;
}
.quick-entry-desc-body textarea:focus {
    border-color: var(--color-accent);
    outline: none;
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/static/css/inventory/quick-entry.css
git commit -m "feat(ui): add quick-entry description collapsible styles"
```

---

### Task 4: Add description block to container quick-entry form

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/containers.html:33-42`

- [ ] **Step 1: Add the collapsible description div to the container quick-entry form**

Replace lines 33-42 (the container quick-entry `<form>`) with:

```html
        <form class="quick-entry"
              hx-post="/ui/actions/containers"
              hx-target="#container-list"
              hx-swap="beforeend"
              hx-on::after-request="if(event.detail.successful) { var wasOpen = this.hasAttribute('data-desc-open'); this.reset(); if(wasOpen) this.setAttribute('data-desc-open',''); this.querySelector('input[name=name]').focus(); }">
            <input type="hidden" name="parent_id" value="{{ if .Data.Container }}{{ .Data.Container.ID }}{{ end }}">
            <span class="quick-entry-icon">+</span>
            <input type="text" name="name" placeholder="{{.T "inventory.new_container_placeholder"}}" required>
            <button type="submit" class="quick-entry-submit">↵</button>
            <div class="quick-entry-desc">
                <button type="button" class="quick-entry-desc-trigger">
                    <span class="quick-entry-desc-arrow">▸</span> {{.T "form.description"}}
                </button>
                <div class="quick-entry-desc-body">
                    <textarea name="description" rows="2" placeholder="{{.T "form.description"}}..."></textarea>
                </div>
            </div>
        </form>
```

Key changes from original:
- `hx-on::after-request` now preserves `data-desc-open` state across reset
- New `.quick-entry-desc` block added after submit button

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/templates/pages/inventory/containers.html
git commit -m "feat(ui): add collapsible description to container quick-entry"
```

---

### Task 5: Add description block to item quick-entry form

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/containers.html:64-74`

- [ ] **Step 1: Add the collapsible description div to the item quick-entry form**

Replace lines 64-74 (the item quick-entry `<form>`) with:

```html
        <form class="quick-entry"
              hx-post="/ui/actions/items"
              hx-target="#item-list"
              hx-swap="beforeend"
              hx-on::after-request="if(event.detail.successful) { var wasOpen = this.hasAttribute('data-desc-open'); this.reset(); if(wasOpen) this.setAttribute('data-desc-open',''); this.querySelector('input[name=name]').focus(); }">
            <input type="hidden" name="container_id" value="{{ .Data.Container.ID }}">
            <span class="quick-entry-icon">+</span>
            <input type="text" name="name" placeholder="{{.T "inventory.new_item_placeholder"}}" required>
            <input type="number" name="quantity" value="1" min="1">
            <button type="submit" class="quick-entry-submit">↵</button>
            <div class="quick-entry-desc">
                <button type="button" class="quick-entry-desc-trigger">
                    <span class="quick-entry-desc-arrow">▸</span> {{.T "form.description"}}
                </button>
                <div class="quick-entry-desc-body">
                    <textarea name="description" rows="2" placeholder="{{.T "form.description"}}..."></textarea>
                </div>
            </div>
        </form>
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/templates/pages/inventory/containers.html
git commit -m "feat(ui): add collapsible description to item quick-entry"
```

---

### Task 6: Remove redundant `<details>` creation forms

**Files:**
- Modify: `internal/embedded/templates/pages/inventory/containers.html:146-165`

- [ ] **Step 1: Remove the `<section class="actions">` block**

Delete lines 146-165 (the entire `<section class="actions">` block with both `<details>` forms). The quick-entry forms now cover the same functionality (name + description).

The section to remove:
```html
    <section class="actions">
        <details>
            <summary>+ {{.T "inventory.add_container"}}</summary>
            <form hx-post="/ui/actions/containers" hx-target="#content">
                <input type="hidden" name="parent_id" value="{{ if .Data.Container }}{{ .Data.Container.ID }}{{ end }}">
                {{ template "fields/name-desc" dict "name" "" "description" "" "rows" "2" "nameLabel" (.T "form.name") "descLabel" (.T "form.description") }}
                <button type="submit">{{.T "inventory.create"}}</button>
            </form>
        </details>
        {{ if .Data.Container }}
        <details>
            <summary>+ {{.T "inventory.add_item"}}</summary>
            <form hx-post="/ui/actions/items" hx-target="#content">
                <input type="hidden" name="container_id" value="{{ .Data.Container.ID }}">
                {{ template "fields/name-desc" dict "name" "" "description" "" "rows" "2" "nameLabel" (.T "form.name") "descLabel" (.T "form.description") }}
                <button type="submit">{{.T "inventory.create"}}</button>
            </form>
        </details>
        {{ end }}
    </section>
```

- [ ] **Step 2: Commit**

```bash
git add internal/embedded/templates/pages/inventory/containers.html
git commit -m "refactor(ui): remove redundant details creation forms"
```

---

### Task 7: Manual smoke test

- [ ] **Step 1: Build and run**

```bash
make build-mac && make run
```

- [ ] **Step 2: Verify in browser**

Open `http://localhost:8080/ui` and test:
1. Container quick-entry: Tab from name field reaches description trigger
2. Enter/Space on trigger expands textarea with animation
3. Type description, press Tab to submit button, Enter submits
4. After submit: form resets, description stays expanded, focus on name
5. Escape on textarea collapses, focus returns to trigger
6. Navigate into a container — repeat for item quick-entry (Tab: name → quantity → trigger)
7. Submit item without expanding description — no description set
8. Submit item with description — description visible in item list

---

### Task 8: E2E tests

> **Note:** E2E test for "submit container with description" verifies both name and description appear in list.

**Files:**
- Create: `e2e/tests/quick-entry-description.spec.ts`

- [ ] **Step 1: Write E2E tests**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Quick-entry description', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;

  test('create container then test description toggle', async ({ page, app }) => {
    containerName = `Desc Test ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });

    // Create a container first
    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(containerName);
    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await nameInput.press('Enter');
    await resp;
    await expect(page.locator('#container-list')).toContainText(containerName);
  });

  test('container description trigger expands on click', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    const body = page.locator('.containers .quick-entry-desc-body');

    // Initially collapsed
    await expect(body).not.toBeVisible();

    // Click trigger to expand
    await trigger.click();
    await expect(body).toBeVisible();

    // Textarea is focused
    const textarea = page.locator('.containers .quick-entry-desc-body textarea');
    await expect(textarea).toBeFocused();
  });

  test('escape collapses description', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    const body = page.locator('.containers .quick-entry-desc-body');

    await trigger.click();
    await expect(body).toBeVisible();

    // Escape collapses
    await page.keyboard.press('Escape');
    await expect(body).not.toBeVisible();

    // Focus returns to trigger
    await expect(trigger).toBeFocused();
  });

  test('submit container with description', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const name = `WithDesc ${Date.now()}`;

    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(name);

    // Expand and fill description
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    await trigger.click();
    const textarea = page.locator('.containers .quick-entry-desc-body textarea');
    await textarea.fill('Test description text');

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.locator('.containers .quick-entry-submit').click();
    await resp;

    // Container appears in list with name and description
    await expect(page.locator('#container-list')).toContainText(name);
    await expect(page.locator('#container-list')).toContainText('Test description text');
  });

  test('expanded state persists after submit', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });

    // Expand description
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    await trigger.click();

    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(`Persist ${Date.now()}`);

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await nameInput.press('Enter');
    await resp;

    // Description should still be expanded after reset
    const body = page.locator('.containers .quick-entry-desc-body');
    await expect(body).toBeVisible();
  });

  test('item description toggle works', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    // Navigate into the container
    await page.click(`a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    const trigger = page.locator('.items .quick-entry-desc-trigger');
    const body = page.locator('.items .quick-entry-desc-body');

    // Click to expand
    await trigger.click();
    await expect(body).toBeVisible();

    // Fill and submit item with description
    const nameInput = page.locator('.items .quick-entry input[name="name"]');
    await nameInput.fill(`Item Desc ${Date.now()}`);
    const textarea = page.locator('.items .quick-entry-desc-body textarea');
    await textarea.fill('Item description here');

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.locator('.items .quick-entry-submit').click();
    await resp;

    // Item appears in list
    await expect(page.locator('#item-list')).toContainText(`Item Desc`);
  });
});
```

- [ ] **Step 2: Run E2E tests**

```bash
make test-e2e
```

Expected: All 6 tests in `quick-entry-description.spec.ts` pass.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/quick-entry-description.spec.ts
git commit -m "test(e2e): add quick-entry description toggle tests"
```

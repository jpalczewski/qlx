import { test, expect } from '../fixtures/app';

test.describe('Nav colour tokens', () => {
  test('tokens used by the connection indicator resolve to non-empty values', async ({ page, app }) => {
    await page.goto(app.baseURL, { waitUntil: 'domcontentloaded' });
    const values = await page.evaluate(() => {
      const cs = getComputedStyle(document.documentElement);
      return ['--color-text-muted', '--color-success', '--color-warning', '--color-error', '--color-info']
        .map((n) => [n, cs.getPropertyValue(n).trim()]);
    });
    for (const [name, value] of values) {
      expect(value, name).not.toBe('');
    }
  });

  test('connection dot variants render distinct resolved colours', async ({ page, app }) => {
    await page.goto(app.baseURL, { waitUntil: 'domcontentloaded' });
    const colours = await page.evaluate(() => {
      const out: Record<string, string> = {};
      for (const cls of ['', 'conn-dot--ok', 'conn-dot--warn', 'conn-dot--error', 'conn-dot--pulse']) {
        const el = document.createElement('span');
        el.className = ('conn-dot ' + cls).trim();
        document.body.appendChild(el);
        out[cls || 'base'] = getComputedStyle(el).backgroundColor;
        el.remove();
      }
      return out;
    });
    for (const v of Object.values(colours)) {
      expect(v).not.toBe('rgba(0, 0, 0, 0)');
    }
    expect(new Set(Object.values(colours)).size).toBe(5);
  });
});

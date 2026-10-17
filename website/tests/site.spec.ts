import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
const routes = ['/', '/caddy/quickstart/', '/nginx/quickstart/'];
const installationRoutes = ['/installation/', '/installation/source/'];
for (const width of [1440, 390]) {
  for (const route of [...routes, ...installationRoutes]) {
    test(`main route ${route} at ${width}px has readable layout`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
      const response = await page.goto(route);
      expect(response?.status()).toBe(200);
      await expect(page.locator('h1')).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const accessibility = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
      expect(accessibility.violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => ({ target: n.target, summary: n.failureSummary })) }))).toEqual([]);
      if (route !== '/') await expect(page.locator('pre').first()).toBeVisible();
      const name = route === '/' ? 'landing' : route.split('/').filter(Boolean).join('-');
      await page.screenshot({ path: testInfo.outputPath(`${name}-${width}.png`), fullPage: true });
    });
  }
}
test('integration navigation and keyboard skip link work', async ({ page }) => {
  await page.goto('/');
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused();
  await page.keyboard.press('Enter');
  expect(await page.evaluate(() => document.activeElement?.id)).toBe('main');
  await page.getByRole('link', { name: 'Caddy quickstart', exact: true }).click();
  await expect(page).toHaveURL(/\/caddy\/quickstart\//);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Caddy quickstart');
  await page.goto('/');
  await page.getByRole('link', { name: 'Certbot / Nginx quickstart', exact: true }).click();
  await expect(page).toHaveURL(/\/nginx\/quickstart\//);
});
test('static search finds credential workflow and opens a result', async ({ page }) => {
  await page.goto('/overview/');
  await page.getByRole('button', { name: /Search/ }).click();
  const input = page.getByRole('textbox', { name: 'Search', exact: true });
  await input.fill('CSR');
  const result = page.locator('.pagefind-ui__result-link').filter({ hasText: /2-QWAC|CSR|credential/i }).first();
  await expect(result).toBeVisible();
  await result.click();
  await expect(page.locator('h1')).toBeVisible();
  expect(page.url()).not.toContain('/overview/');
});
test('missing route returns a useful 404', async ({ page }) => {
  const response = await page.goto('/this-route-does-not-exist/');
  expect(response?.status()).toBe(404);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('This path has');
  await expect(page.getByRole('link', { name: /home|qwacamole/i }).first()).toBeVisible();
});
test('mobile docs menu, search focus and keyboard code scrolling work', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/caddy/quickstart/');
  await page.getByRole('button', { name: 'Menu', exact: true }).click();
  await page.getByRole('link', { name: 'Certbot / Nginx quickstart', exact: true }).first().click();
  await expect(page).toHaveURL(/\/nginx\/quickstart\//);
  const block = page.locator('pre').filter({ hasText: 'qwacamole-agent setup' });
  await expect(block).toHaveCount(1);
  await block.focus();
  expect(await block.evaluate(el => getComputedStyle(el).outlineStyle)).toBe('solid');
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => block.evaluate(el => el.scrollLeft)).toBeGreaterThan(0);
  const search = page.getByRole('button', { name: 'Search', exact: true });
  await search.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('textbox', { name: 'Search', exact: true })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(search).toBeFocused();
});
test('documentation dark theme retains contrast', async ({ page }) => {
  for (const route of routes.slice(1)) {
    await page.goto(route);
    await page.getByRole('combobox', { name: 'Select theme' }).selectOption('dark');
    const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
    expect(result.violations.map(v => ({ id: v.id, targets: v.nodes.map(n => n.target) }))).toEqual([]);
  }
});
test('maintainer routes and release/setup material stay outside product navigation and search', async ({ page }) => {
  for (const route of ['/project/readme/', '/project/contributing/', '/project/website/', '/project/alpha/', '/development/firefox/', '/development/demos/']) {
    expect((await page.goto(route))?.status()).toBe(404);
  }
  await page.goto('/overview/');
  for (const route of ['/project/readme/', '/project/contributing/', '/project/website/', '/project/alpha/'])
    await expect(page.locator(`a[href="${route}"]`)).toHaveCount(0);
  // Inspect every indexed page rather than a fuzzy query that can match words
  // like "website" and "deployment" in legitimate credential documentation.
  const indexed = await page.evaluate(async () => {
    const bundle = '/pagefind/pagefind.js';
    const pagefind = await import(/* @vite-ignore */ bundle);
    const results = await pagefind.search(null);
    return Promise.all(results.results.map(async (result: { data: () => Promise<{ url: string; content: string }> }) => {
      const data = await result.data();
      return { route: new URL(data.url, location.origin).pathname, content: data.content };
    }));
  });
  expect(indexed.map(result => result.route).sort()).toEqual([
    '/overview/', '/installation/', '/installation/source/', '/caddy/quickstart/', '/nginx/quickstart/', '/caddy/reference/', '/nginx/reference/',
    '/credentials/', '/operations/', '/nginx/migration/', '/support/', '/demonstration/', '/project/licensing/', '/project/security/',
  ].sort());
  expect(indexed.some(result => /WEBSITE_DEPLOY_ENABLED|owner publication approval|this task is complete/i.test(result.content))).toBe(false);
});

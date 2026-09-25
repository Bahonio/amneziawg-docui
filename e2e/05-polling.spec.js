// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp } = require('./helpers');

test('keeps polling live host traffic', async ({ page }) => {
  let polls = 0;
  page.on('request', request => { if (request.url().endsWith('/api/traffic')) polls++; });
  await startApp(page);
  await expect.poll(() => polls, {timeout: 12_000}).toBeGreaterThanOrEqual(2);
});

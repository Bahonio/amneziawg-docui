// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp, api } = require('./helpers');

test('frontend boots and reports the host backend', async ({ page, request }) => {
  const errors = await startApp(page);
  await expect(page).toHaveTitle('AWG DocUI');
  const status = await api(request, '/api/system/status');
  expect(status.backend.vpn_runtime).toBe('Kernel');
  await page.locator('[data-action="health"]').click();
  await expect(page.locator('.health-list')).toContainText('Host agent');
  expect(errors).toEqual([]);
});

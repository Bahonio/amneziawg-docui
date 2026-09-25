// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp, api, e2eServer, selectServer } = require('./helpers');

test('rejects a duplicate port and removes only the explicit E2E interface', async ({ page, request }) => {
  await startApp(page);
  const server = await e2eServer(request);
  await page.locator('[data-action="new-server"]').first().click();
  await page.locator('#server-name').fill('duplicate port');
  await page.locator('#server-port').fill(String(server.port));
  await page.locator('form[data-form="server"] button[type="submit"]').click();
  await expect(page.locator('#toast')).toHaveClass(/visible/);
  expect((await api(request, '/api/servers')).filter(item => item.name === 'duplicate port')).toHaveLength(0);
  await page.locator('[data-action="close-dialog"]').click();
  await selectServer(page, server);
  await page.locator('[data-tab="network"]').click();
  await page.locator('[data-action="delete-server"]').click();
  await page.locator('[data-confirm="delete-server"]').click();
  await expect.poll(async () => (await api(request, '/api/servers')).some(item => item.id === server.id)).toBeFalsy();
});

// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp, api, e2eServer, selectServer } = require('./helpers');

test('shows a real QR, config and AmneziaVPN link', async ({ page, request }) => {
  const errors = await startApp(page);
  const server = await e2eServer(request);
  const client = (await api(request, `/api/servers/${server.id}/clients`)).find(item => item.name === 'e2e laptop renamed');
  await selectServer(page, server);
  await page.locator(`tr[data-client="${client.id}"]`).click();
  await page.locator('[data-action="client-config"]').first().click();
  await expect(page.locator('img.qr')).toBeVisible();
  const qr = await request.get(`/api/servers/${server.id}/clients/${client.id}/qr`);
  expect(qr.headers()['content-type']).toContain('image/png');
  await page.locator('[data-config-tab="link"]').click();
  await expect(page.locator('.copy-row input')).toHaveValue(/^vpn:\/\//);
  await page.locator('[data-config-tab="clean"]').click();
  await expect(page.locator('pre.code')).toContainText('[Interface]');
  expect(errors).toEqual([]);
});

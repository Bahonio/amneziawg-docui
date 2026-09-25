// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp, api, serverName } = require('./helpers');

test('creates a host-owned interface through the form', async ({ page, request }) => {
  const errors = await startApp(page);
  expect((await api(request, '/api/servers')).some(server => server.name === serverName)).toBeFalsy();
  await page.locator('[data-action="new-server"]').first().click();
  await page.locator('#server-name').fill(serverName);
  await page.locator('#server-port').fill(process.env.E2E_VPN_PORT || '55281');
  await page.locator('#server-subnet').fill(process.env.E2E_VPN_SUBNET || '10.231.73.0/24');
  await page.locator('form[data-form="server"] button[type="submit"]').click();
  await expect.poll(async () => (await api(request, '/api/servers')).some(server => server.name === serverName)).toBeTruthy();
  const server = (await api(request, '/api/servers')).find(item => item.name === serverName);
  expect(server.config_path).toContain('/etc/amnezia/amneziawg/');
  expect(server.obfuscation_params.HeaderProtectionKey).not.toBe('');
  expect(server.endpoint).toBe('');

  await page.locator('[data-tab="network"]').click();
  await page.locator('[data-action="edit-endpoint"]').click();
  await page.locator('#edit-server-endpoint').fill('vpn.example.com');
  await page.locator('form[data-form="endpoint"] button[type="submit"]').click();
  await expect.poll(async () => (await api(request, '/api/servers')).find(item => item.name === serverName)?.endpoint).toBe('vpn.example.com');
  expect(errors).toEqual([]);
});

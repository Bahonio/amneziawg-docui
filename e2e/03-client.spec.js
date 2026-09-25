// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { test, expect } = require('@playwright/test');
const { startApp, api, e2eServer, selectServer } = require('./helpers');

test('adds and edits a peer without cycling the interface', async ({ page, request }) => {
  const errors = await startApp(page);
  const server = await e2eServer(request);
  expect(server).toBeTruthy();
  await selectServer(page, server);
  await page.locator('[data-action="add-client"]').first().click();
  await page.locator('#client-name').fill('e2e laptop');
  await page.locator('form[data-form="client"] button[type="submit"]').click();
  await expect.poll(async () => (await api(request, `/api/servers/${server.id}/clients`)).some(client => client.name === 'e2e laptop')).toBeTruthy();
  await page.locator('tr[data-client]').filter({hasText:'e2e laptop'}).click();
  await page.locator('[data-action="edit-client"]').click();
  await page.locator('#client-name').fill('e2e laptop renamed');
  await page.locator('form[data-form="client"] button[type="submit"]').click();
  await expect.poll(async () => (await api(request, `/api/servers/${server.id}/clients`)).some(client => client.name === 'e2e laptop renamed')).toBeTruthy();
  expect(errors).toEqual([]);
});

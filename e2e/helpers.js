// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

const { expect } = require('@playwright/test');
const serverName = process.env.E2E_SERVER_NAME || 'AWG DocUI E2E';

async function startApp(page) {
  const errors = [];
  page.on('pageerror', error => errors.push(String(error)));
  await page.addInitScript(() => localStorage.setItem('awg-docui-language', 'en'));
  await page.goto('/', { waitUntil: 'networkidle' });
  await expect(page.locator('.brand')).toContainText('AWG DocUI');
  return errors;
}
async function api(request, path) {
  const response = await request.get(path);
  expect(response.ok(), `${path} -> ${response.status()}`).toBeTruthy();
  return response.json();
}
async function e2eServer(request) {
  return (await api(request, '/api/servers')).find(server => server.name === serverName);
}
async function selectServer(page, server) {
  await page.locator(`[data-server="${server.id}"]`).click();
}
module.exports = { startApp, api, e2eServer, selectServer, serverName };

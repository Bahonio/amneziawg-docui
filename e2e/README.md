<!-- Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio. -->
<!-- SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later -->

# Browser tests

Playwright drives the production DOM and the real REST API. The target must be
a disposable Linux host with the official AmneziaWG module and tools plus the
AWG DocUI agent. The suite creates an interface, changes a live peer, and then
explicitly deletes that test interface. Never point it at a production host.

```sh
cd e2e
npm install --no-audit --no-fund
E2E_URL=http://test-host:54845 \
E2E_USER=admin E2E_PASSWORD='test-only-password' \
npx playwright test
```

Use a unique `E2E_VPN_PORT` and `E2E_VPN_SUBNET` if the defaults clash with the
test host. Existing interfaces are not selected or deleted.

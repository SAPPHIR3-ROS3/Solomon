import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { isSensitiveNetworkAddress } = await server.ssrLoadModule("/src/settings/networkAddressPrivacy.ts");
after(() => server.close());

test("public IPs are obscured while LAN, shared and loopback addresses stay readable", () => {
  for (const address of ["8.8.8.8/32", "1.1.1.1", "172.15.0.1", "172.32.0.1", "192.169.0.1", "100.63.255.255", "100.128.0.1"]) {
    assert.equal(isSensitiveNetworkAddress(address), true, address);
  }
  for (const address of ["127.0.0.1/8", "10.8.0.1/24", "172.16.0.1", "172.31.255.255", "192.168.1.10/24", "169.254.1.1", "100.64.0.1", "100.127.255.255"]) {
    assert.equal(isSensitiveNetworkAddress(address), false, address);
  }
});

test("Tailscale addresses are obscured regardless of their address range", () => {
  for (const address of ["100.73.30.96/32", "10.0.0.1", "8.8.8.8"]) assert.equal(isSensitiveNetworkAddress(address, true), true);
});

test("unknown address formats are obscured conservatively", () => {
  for (const address of ["", "192.168.999.1", "192.168.x.1", "fd7a:115c:a1e0::1"]) assert.equal(isSensitiveNetworkAddress(address), true);
});

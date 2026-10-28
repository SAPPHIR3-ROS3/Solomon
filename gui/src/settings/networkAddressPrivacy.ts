/** Keep local/shared IPv4 addresses readable; obscure Tailscale and other addresses. */
export function isSensitiveNetworkAddress(address: string, tailscale = false): boolean {
  if (tailscale) return true;
  const host = address.split("/", 1)[0];
  const parts = host.split(".");
  if (parts.length !== 4 || parts.some(part => !/^\d{1,3}$/.test(part) || Number(part) > 255)) return true;
  const [a, b] = parts.map(Number);
  return !(a === 0 || a === 10 || a === 127
    || (a === 169 && b === 254)
    || (a === 172 && b >= 16 && b <= 31)
    || (a === 192 && b === 168)
    || (a === 100 && b >= 64 && b <= 127));
}

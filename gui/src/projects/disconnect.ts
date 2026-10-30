import { serverEndpoint } from "../platform";
import { invalidateModelCatalogCache } from "./models";

export async function disconnectProvider(provider: string): Promise<void> {
  const response = await fetch(await serverEndpoint("/__solomon/disconnect-provider"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ provider, confirmed: true }),
  });
  if (!response.ok) throw new Error("Unable to remove provider");
  invalidateModelCatalogCache();
}

export interface OADProvider {
  name: string;
  display_name: string;
  authority: string;
  client_id: string;
  scope: string;
}

export interface OADConfig {
  providers: OADProvider[];
  redirect_uri: string;
  post_logout_uri: string;
}

export async function fetchConfig(): Promise<OADConfig> {
  const res = await fetch("/config.json");
  if (!res.ok) {
    throw new Error(`Failed to fetch /config.json: ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<OADConfig>;
}

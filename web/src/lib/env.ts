import { z } from "zod";

const urlOrEmpty = z.string().refine(
  (v) => v === "" || z.string().url().safeParse(v).success,
  { message: "Must be empty or a valid URL" }
);

const envSchema = z.object({
  // Empty string means "use relative paths" — nginx proxies /api to the backend.
  VITE_API_BASE_URL: urlOrEmpty.default(""),

  // OIDC vars are optional in Phase 7.1 (auth is wired up in Phase 7.2).
  VITE_OIDC_AUTHORITY: z.string().default(""),
  VITE_OIDC_CLIENT_ID: z.string().default(""),
  VITE_OIDC_REDIRECT_URI: z.string().default(""),
  VITE_OIDC_POST_LOGOUT_URI: z.string().default(""),
  VITE_OIDC_SCOPE: z.string().default("openid profile email"),
});

type Env = z.infer<typeof envSchema>;

function loadEnv(): Env {
  const raw = {
    VITE_API_BASE_URL: import.meta.env["VITE_API_BASE_URL"],
    VITE_OIDC_AUTHORITY: import.meta.env["VITE_OIDC_AUTHORITY"],
    VITE_OIDC_CLIENT_ID: import.meta.env["VITE_OIDC_CLIENT_ID"],
    VITE_OIDC_REDIRECT_URI: import.meta.env["VITE_OIDC_REDIRECT_URI"],
    VITE_OIDC_POST_LOGOUT_URI: import.meta.env["VITE_OIDC_POST_LOGOUT_URI"],
    VITE_OIDC_SCOPE: import.meta.env["VITE_OIDC_SCOPE"],
  };

  const result = envSchema.safeParse(raw);

  if (!result.success) {
    const errors = result.error.issues
      .map((e) => `  ${e.path.join(".")}: ${e.message}`)
      .join("\n");
    throw new Error(`Invalid environment configuration:\n${errors}`);
  }

  return result.data;
}

export const env = loadEnv();

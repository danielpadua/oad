import { z } from "zod";

const urlOrEmpty = z.string().refine(
  (v) => v === "" || z.string().url().safeParse(v).success,
  { message: "Must be empty or a valid URL" }
);

const envSchema = z.object({
  // Empty string means "use relative paths" — the API and frontend share the same origin.
  // Override with an absolute URL when running the Vite dev server against a separate API.
  VITE_API_BASE_URL: urlOrEmpty.default(""),
});

type Env = z.infer<typeof envSchema>;

function loadEnv(): Env {
  const raw = {
    VITE_API_BASE_URL: import.meta.env["VITE_API_BASE_URL"],
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

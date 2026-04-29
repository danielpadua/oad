import { vi } from "vitest";
import { http as mswHttp, HttpResponse } from "msw";
import { server } from "@/test/mocks/server";

// Mock oidc module — prevents UserManager from running in jsdom
vi.mock("@/lib/oidc", () => ({
  getUserManager: () => ({
    settings: { authority: "http://localhost", client_id: "test-client" },
  }),
  initUserManager: vi.fn(),
}));

import { http, HttpError, NetworkError, setTokenGetter, setUnauthorizedHandler } from "@/lib/http-client";

describe("HttpError", () => {
  it("carries status and body", () => {
    const err = new HttpError(403, { code: "FORBIDDEN", message: "Forbidden" });
    expect(err.status).toBe(403);
    expect(err.body.code).toBe("FORBIDDEN");
    expect(err.message).toBe("Forbidden");
    expect(err.name).toBe("HttpError");
  });
});

describe("http client — success cases", () => {
  it("GET resolves with parsed JSON", async () => {
    server.use(
      mswHttp.get("/api/test", () => HttpResponse.json({ ok: true })),
    );

    const result = await http.get<{ ok: boolean }>("/api/test");
    expect(result.ok).toBe(true);
  });

  it("POST sends body and resolves", async () => {
    server.use(
      mswHttp.post("/api/items", async ({ request }) => {
        const body = await request.json();
        return HttpResponse.json({ received: body }, { status: 201 });
      }),
    );

    const result = await http.post<{ received: unknown }>("/api/items", { name: "x" });
    expect(result).toEqual({ received: { name: "x" } });
  });

  it("DELETE resolves with undefined on 204", async () => {
    server.use(
      mswHttp.delete("/api/items/1", () => new HttpResponse(null, { status: 204 })),
    );

    const result = await http.del("/api/items/1");
    expect(result).toBeUndefined();
  });

  it("injects Bearer token from tokenGetter", async () => {
    setTokenGetter(() => "test-token");

    let capturedAuth = "";
    server.use(
      mswHttp.get("/api/secured", ({ request }) => {
        capturedAuth = request.headers.get("Authorization") ?? "";
        return HttpResponse.json({ ok: true });
      }),
    );

    await http.get("/api/secured");
    expect(capturedAuth).toBe("Bearer test-token");

    // reset
    setTokenGetter(() => null);
  });

  it("attaches X-Correlation-ID header", async () => {
    let correlationId = "";
    server.use(
      mswHttp.get("/api/corr", ({ request }) => {
        correlationId = request.headers.get("X-Correlation-ID") ?? "";
        return HttpResponse.json({});
      }),
    );

    await http.get("/api/corr");
    expect(correlationId).toMatch(/^[0-9a-f-]{36}$/);
  });
});

describe("http client — error cases", () => {
  it("throws HttpError for 4xx responses", async () => {
    server.use(
      mswHttp.get("/api/not-found", () =>
        HttpResponse.json({ code: "NOT_FOUND", message: "Not found" }, { status: 404 }),
      ),
    );

    await expect(http.get("/api/not-found")).rejects.toMatchObject({
      name: "HttpError",
      status: 404,
      body: { code: "NOT_FOUND" },
    });
  });

  it("throws HttpError for 5xx responses", async () => {
    server.use(
      mswHttp.get("/api/boom", () =>
        HttpResponse.json({ code: "INTERNAL_ERROR", message: "Server error" }, { status: 500 }),
      ),
    );

    await expect(http.get("/api/boom")).rejects.toMatchObject({
      name: "HttpError",
      status: 500,
    });
  });

  it("calls the unauthorized handler on 401", async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);

    server.use(
      mswHttp.get("/api/auth-required", () =>
        HttpResponse.json({ code: "UNAUTHORIZED", message: "Unauthorized" }, { status: 401 }),
      ),
    );

    await expect(http.get("/api/auth-required")).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledOnce();

    // reset
    setUnauthorizedHandler(() => {});
  });

  it("throws NetworkError on transport failure", async () => {
    server.use(
      mswHttp.get("/api/network-fail", () => HttpResponse.error()),
    );

    await expect(http.get("/api/network-fail")).rejects.toMatchObject({
      name: "NetworkError",
    });
  });

  it("falls back to UNKNOWN error code when response body is not JSON", async () => {
    server.use(
      mswHttp.get("/api/bad-body", () =>
        new HttpResponse("not json", { status: 503, headers: { "Content-Type": "text/plain" } }),
      ),
    );

    await expect(http.get("/api/bad-body")).rejects.toMatchObject({
      status: 503,
      body: { code: "UNKNOWN" },
    });
  });
});

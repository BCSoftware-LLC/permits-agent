import test from "node:test";
import assert from "node:assert/strict";
import worker from "../deploy/cloudflare/worker.mjs";

test("proxy uses configured origin, removes ambient credentials, and refuses redirects", async () => {
  const original = globalThis.fetch;
  try {
    let seen;
    globalThis.fetch = async (url, options) => {
      seen = { url: String(url), options };
      return new Response("redirect", {
        status: 302,
        headers: { location: "https://elsewhere.example" },
      });
    };
    const req = new Request("https://product.example/api/call?version=1", {
      method: "POST",
      body: "{}",
      headers: {
        authorization: "Bearer synthetic-customer",
        cookie: "private=x",
        "CF-Access-Client-Secret": "caller-value",
      },
    });
    const res = await worker.fetch(req, {
      ORIGIN_URL: "https://origin.example",
      ORIGIN_ACCESS_CLIENT_ID: "service-id",
      ORIGIN_ACCESS_CLIENT_SECRET: "service-secret",
    });
    assert.equal(res.status, 502);
    assert.equal(seen.url, "https://origin.example/api/call?version=1");
    assert.equal(seen.options.redirect, "manual");
    assert.equal(seen.options.headers.get("cookie"), null);
    assert.equal(
      seen.options.headers.get("authorization"),
      "Bearer synthetic-customer",
    );
    assert.equal(
      seen.options.headers.get("CF-Access-Client-Secret"),
      "service-secret",
    );
  } finally {
    globalThis.fetch = original;
  }
});
test("proxy fails closed on missing, insecure or looping origin", async () => {
  for (const origin of [
    undefined,
    "http://origin.example",
    "https://user:pass@origin.example",
    "https://product.example",
    "https://origin.example/path",
  ]) {
    assert.equal(
      (
        await worker.fetch(new Request("https://product.example/"), {
          ORIGIN_URL: origin,
        })
      ).status,
      503,
    );
  }
});

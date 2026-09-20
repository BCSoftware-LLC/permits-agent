// Optional same-origin edge proxy. The Go origin owns authentication, tenant
// authorization, quotas and storage. It must have HTTPS and a durable volume.
export default {
  async fetch(request, env) {
    let origin;
    try {
      origin = new URL(env.ORIGIN_URL);
    } catch {
      return new Response("Origin not configured", { status: 503 });
    }
    if (
      origin.protocol !== "https:" ||
      origin.username ||
      origin.password ||
      origin.search ||
      origin.hash ||
      !["", "/"].includes(origin.pathname)
    )
      return new Response("Invalid origin configuration", { status: 503 });
    const incoming = new URL(request.url);
    if (incoming.hostname === origin.hostname)
      return new Response("Origin must not point to this worker", {
        status: 503,
      });
    const target = new URL(origin.origin);
    target.pathname = incoming.pathname;
    target.search = incoming.search;
    const headers = new Headers(request.headers);
    headers.delete("host");
    headers.delete("cookie"); // This deployment uses bearer keys, never ambient cookies.
    headers.delete("cf-access-client-id");
    headers.delete("cf-access-client-secret");
    // Optional Cloudflare Access service token restricting direct origin access.
    if (env.ORIGIN_ACCESS_CLIENT_ID && env.ORIGIN_ACCESS_CLIENT_SECRET) {
      headers.set("CF-Access-Client-Id", env.ORIGIN_ACCESS_CLIENT_ID);
      headers.set("CF-Access-Client-Secret", env.ORIGIN_ACCESS_CLIENT_SECRET);
    }
    if (
      request.method !== "GET" &&
      request.method !== "HEAD" &&
      Number(request.headers.get("content-length")) > 65536
    )
      return new Response("Request too large", { status: 413 });
    try {
      const upstream = await fetch(target, {
        method: request.method,
        headers,
        body: ["GET", "HEAD"].includes(request.method)
          ? undefined
          : request.body,
        redirect: "manual",
        signal: request.signal,
        cf: { cacheTtl: 0, cacheEverything: false },
      });
      // Never follow origin redirects with caller bearer keys or origin secrets.
      if (upstream.status >= 300 && upstream.status < 400)
        return new Response("Unexpected origin redirect", { status: 502 });
      const response = new Response(upstream.body, upstream);
      response.headers.set("Cache-Control", "no-store");
      response.headers.delete("set-cookie");
      return response;
    } catch {
      return new Response("Service temporarily unavailable", { status: 502 });
    }
  },
};

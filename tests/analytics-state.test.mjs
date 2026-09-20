import test from "node:test";
import assert from "node:assert/strict";
import vm from "node:vm";
import fs from "node:fs";

test("analytics clears on identity changes and ignores delayed private results", async () => {
  const nodes = new Map();
  class Node {
    children = [];
    value = "";
    textContent = "";
    hidden = false;
    handlers = {};
    append(...items) {
      this.children.push(...items);
    }
    replaceChildren(...items) {
      this.children = items;
      this.textContent = "";
    }
    addEventListener(name, fn) {
      this.handlers[name] = fn;
    }
    setAttribute() {}
  }
  const document = {
    getElementById(id) {
      if (!nodes.has(id)) nodes.set(id, new Node());
      return nodes.get(id);
    },
    createElement() {
      return new Node();
    },
  };
  const pending = [];
  const context = vm.createContext({
    document,
    AbortController,
    AbortSignal,
    URLSearchParams,
    fetch(path, options) {
      return new Promise((resolve) => pending.push({ path, options, resolve }));
    },
  });
  vm.runInContext(
    fs.readFileSync(
      new URL("../internal/platform/web/analytics.js", import.meta.url),
      "utf8",
    ),
    context,
  );
  nodes.get("analytics-key").value = "synthetic-platform-key";
  nodes.get("analytics-scope").value = "platform";
  nodes.get("analytics-days").value = "30";
  const first = nodes
    .get("analytics-form")
    .handlers.submit({ preventDefault() {} });
  assert.equal(pending.length, 1);
  assert.equal(nodes.get("analytics-key").value, "");
  nodes.get("analytics-clients").textContent = "private operator metrics";
  nodes.get("analytics-disconnect").handlers.click();
  assert.equal(nodes.get("analytics-clients").textContent, "");
  assert.equal(nodes.get("analytics-report").hidden, true);
  pending[0].resolve({
    ok: true,
    json: async () => ({ rows: [{ client_id: "private-data" }] }),
  });
  await first;
  assert.equal(nodes.get("analytics-report").hidden, true);
  await nodes.get("analytics-form").handlers.submit({ preventDefault() {} });
  assert.equal(pending.length, 1, "disconnect must discard the key");
  nodes.get("analytics-key").value = "synthetic-tenant-key";
  const second = nodes
    .get("analytics-form")
    .handlers.submit({ preventDefault() {} });
  nodes.get("analytics-scope").value = "tenant";
  nodes.get("analytics-scope").handlers.input();
  pending[1].resolve({
    ok: true,
    json: async () => ({ rows: [{ client_id: "old-scope" }] }),
  });
  await second;
  assert.equal(
    nodes.get("analytics-report").hidden,
    true,
    "old scope cannot repopulate the page",
  );
});

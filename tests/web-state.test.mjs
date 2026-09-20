import test from "node:test";
import assert from "node:assert/strict";
import vm from "node:vm";
import fs from "node:fs";
import { webcrypto } from "node:crypto";

function environment() {
  const nodes = new Map();
  class Node {
    children = [];
    hidden = false;
    value = "";
    textContent = "";
    resets = 0;
    handlers = {};
    classList = { toggle() {} };
    elements = {
      namedItem() {
        return null;
      },
    };
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
    querySelectorAll() {
      return [];
    }
    reset() {
      this.resets++;
      this.value = "";
    }
  }
  const document = {
    getElementById(id) {
      if (!nodes.has(id)) nodes.set(id, new Node());
      return nodes.get(id);
    },
    createElement() {
      return new Node();
    },
    createTextNode(text) {
      return { textContent: text };
    },
  };
  const pending = [];
  const context = vm.createContext({
    document,
    crypto: webcrypto,
    AbortController,
    AbortSignal,
    DOMException,
    setTimeout,
    URL,
    Blob,
    FormData,
    fetch(path, options) {
      // No browser and no network: deliberately retain delayed responses even
      // after abort to verify the generation guard, not merely AbortController.
      return new Promise((resolve) => pending.push({ path, options, resolve }));
    },
  });
  vm.runInContext(
    fs.readFileSync(
      new URL("../internal/platform/web/app.js", import.meta.url),
      "utf8",
    ),
    context,
  );
  return { context, nodes, pending };
}

test("disconnect removes customer content and rejects a delayed previous-tenant response", async () => {
  const { context, nodes, pending } = environment();
  vm.runInContext(
    "accessKey='tenant-a-synthetic'; $('notes').textContent='Tenant A private note'; $('assistant-form').value='Tenant A question';",
    context,
  );
  const promise = vm.runInContext("loadCase('tenant-a-case')", context);
  const request = pending.find((r) => r.path === "/api/call");
  vm.runInContext(
    "resetConnection(); accessKey='tenant-b-synthetic';",
    context,
  );
  request.resolve({
    ok: true,
    json: async () => ({
      structuredContent: {
        data: {
          case: {
            id: "tenant-a-case",
            intake: { business_name: "Tenant A" },
            notes: [],
          },
          plan: { questions: [], steps: [], sources: [] },
        },
      },
    }),
  });
  await assert.rejects(promise, (err) => err.name === "AbortError");
  assert.equal(vm.runInContext("currentCase", context), null);
  assert.equal(nodes.get("notes").textContent, "");
  assert.equal(nodes.get("assistant-form").value, "");
  assert.equal(nodes.get("case-view").hidden, true);
  assert.equal(vm.runInContext("accessKey", context), "tenant-b-synthetic");
});

test("customer data uses text nodes and access keys are not stored persistently", () => {
  const { context, nodes } = environment();
  vm.runInContext(
    `render({id:'test',intake:{business_name:'<script>unsafe</script>',city:'Example'},notes:[{kind:'research',recorded_at:'2026-09-20T00:00:00Z',text:'<img src=x onerror=alert(1)>'}]},{questions:[],steps:[],sources:[]},false)`,
    context,
  );
  assert.equal(nodes.get("case-title").textContent, "<script>unsafe</script>");
  assert.equal(
    nodes.get("notes").children[0].children[1].textContent,
    "<img src=x onerror=alert(1)>",
  );
  assert.equal(
    vm.runInContext(
      "typeof localStorage + '/' + typeof sessionStorage",
      context,
    ),
    "undefined/undefined",
  );
});

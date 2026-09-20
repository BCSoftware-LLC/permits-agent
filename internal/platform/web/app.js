"use strict";
const $ = (id) => document.getElementById(id);
let connectionEpoch = 0;
const pendingRequests = new Set();
let assistantEnabled = false,
  editingCase = null,
  caseOffset = 0;
let accessKey = "",
  currentCase = null,
  example = true,
  createKey = crypto.randomUUID();
const notice = (message, error = false) => {
  $("notice").textContent = message;
  $("notice").classList.toggle("error", error);
};
function element(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}
async function request(path, options = {}) {
  const epoch = connectionEpoch,
    controller = new AbortController();
  pendingRequests.add(controller);
  try {
    const response = await fetch(path, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...(accessKey ? { Authorization: `Bearer ${accessKey}` } : {}),
        ...options.headers,
      },
      signal: AbortSignal.any([
        controller.signal,
        AbortSignal.timeout(path === "/api/agent" ? 120000 : 30000),
      ]),
      cache: "no-store",
    });
    const result = await response.json();
    if (epoch !== connectionEpoch)
      throw new DOMException("Connection changed", "AbortError");
    if (!response.ok || result.isError)
      throw new Error(
        result.error ||
          result.content?.find((c) => c.type === "text")?.text ||
          "The request could not be completed.",
      );
    return result;
  } catch (err) {
    if (epoch !== connectionEpoch)
      throw new DOMException("Connection changed", "AbortError");
    throw err;
  } finally {
    pendingRequests.delete(controller);
  }
}
async function call(name, args = {}) {
  const result = await request("/api/call", {
    method: "POST",
    body: JSON.stringify({ name, arguments: args }),
  });
  return result.structuredContent?.data ?? JSON.parse(result.content[0].text);
}
function task(form, action) {
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const buttons = [...form.querySelectorAll('button[type="submit"]')];
    buttons.forEach((b) => (b.disabled = true));
    try {
      await action(new FormData(form));
    } catch (err) {
      if (err.name !== "AbortError")
        notice(
          err.name === "TimeoutError"
            ? "The request timed out. Retry to check the result."
            : err.message,
          true,
        );
    } finally {
      buttons.forEach((b) => (b.disabled = false));
    }
  });
}
function requireConnection() {
  if (!accessKey)
    throw new Error(
      "Connect with a pilot access key to save and export your own cases.",
    );
}
async function updateUsage() {
  if (!accessKey) return;
  const v = await request("/api/usage");
  $("usage").textContent =
    `${v.requests.toLocaleString()} / ${v.monthly_request_limit.toLocaleString()} requests this month. ${v.model_usage.reserved_calls} / ${v.monthly_model_call_limit} model calls. Billing is not active.`;
}
async function loadCases(append = false) {
  if (!append) {
    caseOffset = 0;
    $("cases-list").replaceChildren();
  }
  const cases = await call("case_list", { limit: 50, offset: caseOffset });
  caseOffset += cases.length;
  $("more-cases").hidden = cases.length < 50;
  if (!cases.length)
    $("cases-list").append(
      element(
        "p",
        "No cases yet. Start with the business and premises.",
        "small muted",
      ),
    );
  for (const c of cases) {
    const button = element("button", undefined, "case-link");
    button.type = "button";
    button.append(
      element("strong", c.intake.business_name || c.intake.business_type),
      element("span", c.intake.city || `${c.intake.county} County`),
    );
    button.addEventListener("click", () =>
      loadCase(c.id).catch((e) => {
        if (e.name !== "AbortError") notice(e.message, true);
      }),
    );
    $("cases-list").append(button);
  }
  await updateUsage();
}
async function loadCase(id) {
  const data = await call("case_get", { case_id: id });
  render(data.case, data.plan, false);
  notice(
    "Case loaded. Review the checklist and record the information you confirm.",
  );
}
function render(c, plan, isExample) {
  currentCase = c;
  editingCase = null;
  example = isExample;
  $("edit-intake").hidden = isExample;
  $("intake-form").hidden = true;
  $("case-view").hidden = false;
  $("case-title").textContent =
    c.intake.business_name || c.intake.business_type;
  $("case-location").textContent = [
    c.intake.address,
    c.intake.city,
    c.intake.county && `${c.intake.county} County`,
  ]
    .filter(Boolean)
    .join(" · ");
  $("questions").replaceChildren(element("h4", "Questions to resolve"));
  for (const q of plan.questions) $("questions").append(element("p", `○ ${q}`));
  $("steps").replaceChildren();
  for (const step of plan.steps) {
    const card = element("article", undefined, "step");
    card.append(
      element("h4", step.title),
      element("p", step.authority, "step-authority"),
      element("p", step.action),
    );
    $("steps").append(card);
  }
  $("sources").replaceChildren();
  for (const source of plan.sources) {
    const row = element("div", undefined, "source-row");
    const link = element("a", `${source.title} ↗`);
    link.href = source.url;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    row.append(link);
    if (!isExample) {
      const button = element("button", "Read official page", "text-button");
      button.type = "button";
      const detail = element("details");
      row.append(button, detail);
      button.addEventListener("click", async () => {
        button.disabled = true;
        try {
          requireConnection();
          const doc = await call("source_read", { source_id: source.id });
          if (currentCase?.id !== c.id) return;
          detail.replaceChildren(
            element(
              "summary",
              `${doc.stale ? "Stale cached" : "Retrieved"} evidence · ${new Date(doc.fetched_at).toLocaleDateString()}`,
            ),
            element(
              "p",
              doc.warning || "Public evidence; applicability requires review.",
              "small muted",
            ),
            element("p", doc.text, "source-text"),
          );
          detail.open = true;
          notice(
            "Official page retrieved. Review its applicability and form instructions before recording a conclusion.",
          );
        } catch (err) {
          if (err.name !== "AbortError") notice(err.message, true);
        } finally {
          button.disabled = false;
        }
      });
    }
    $("sources").append(row);
  }
  $("notes").replaceChildren();
  if (!c.notes.length)
    $("notes").append(
      element("p", "No information recorded yet.", "small muted"),
    );
  for (const n of c.notes) {
    const note = element("div", undefined, "note");
    note.append(
      element(
        "small",
        `${n.kind.replaceAll("_", " ")} · ${new Date(n.recorded_at).toLocaleString()} · user supplied`,
      ),
      document.createTextNode(n.text),
    );
    if (n.source_url) {
      const link = element("a", " View source ↗");
      link.href = n.source_url;
      link.rel = "noopener noreferrer";
      link.target = "_blank";
      note.append(link);
    }
    $("notes").append(note);
  }
  $("assistant-section").hidden = isExample || !assistantEnabled;
  $("assistant-answer").replaceChildren();
  $("assistant-form").reset();
  $("export").disabled = isExample;
  $("note-form").hidden = isExample;
  $("reminder-form").hidden = isExample;
}
function startNew() {
  editingCase = null;
  $("save-intake").textContent = "Create preparation case ↗";
  $("cancel-edit").hidden = true;
  currentCase = null;
  example = false;
  createKey = crypto.randomUUID();
  $("intake-form").reset();
  $("intake-form").hidden = false;
  $("case-view").hidden = true;
  notice(
    accessKey
      ? "Create a case using the business information you know. Unknown details can remain blank."
      : "Connect with a pilot access key to save this case. You can explore the fictional example at any time.",
  );
}
function download(file) {
  const url = URL.createObjectURL(
    new Blob([file.content], { type: file.mime_type }),
  );
  const a = element("a");
  a.href = url;
  a.download = file.filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 10000);
}
async function showExample() {
  const data = await request("/api/example");
  render(data.case, data.plan, true);
}
function resetConnection() {
  connectionEpoch++;
  for (const request of pendingRequests) request.abort();
  pendingRequests.clear();
  accessKey = "";
  currentCase = null;
  for (const id of [
    "case-title",
    "case-location",
    "questions",
    "steps",
    "sources",
    "notes",
    "assistant-answer",
  ])
    $(id).replaceChildren();
  createKey = crypto.randomUUID();
  $("access-key").value = "";
  $("connect-form").hidden = false;
  $("disconnect").hidden = true;
  $("connection-status").textContent = "Example workspace";
  $("cases-list").replaceChildren();
  const button = element(
    "button",
    "Explore fictional café example",
    "case-link",
  );
  button.type = "button";
  button.addEventListener("click", () =>
    showExample().catch((e) => {
      if (e.name !== "AbortError") notice(e.message, true);
    }),
  );
  $("cases-list").append(button);
  $("usage").textContent = "Preview only. No customer information is stored.";
  editingCase = null;
  caseOffset = 0;
  $("more-cases").hidden = true;
  $("assistant-form").reset();
  $("note-form").reset();
  $("intake-form").reset();
  $("reminder-form").reset();
  $("case-view").hidden = true;
  $("intake-form").hidden = false;
}
task($("connect-form"), async (data) => {
  const key = $("access-key").value.trim();
  resetConnection();
  accessKey = key;
  try {
    await loadCases();
    const capabilities = await request("/.well-known/permits-agent.json");
    assistantEnabled = capabilities.owner_assistant_enabled;
  } catch (err) {
    if (err.name !== "AbortError") resetConnection();
    throw err;
  }
  $("connection-status").textContent = "Customer workspace";
  $("disconnect").hidden = false;
  startNew();
  notice(
    "Connected. Your access key is held only in this tab; reload or disconnect to clear it.",
  );
});
$("disconnect").addEventListener("click", () => {
  resetConnection();
  notice("Disconnected. Access key and customer views cleared.");
  showExample().catch((e) => {
    if (e.name !== "AbortError") notice(e.message, true);
  });
});
$("new-case").addEventListener("click", startNew);
$("example-case").addEventListener("click", () =>
  showExample().catch((e) => {
    if (e.name !== "AbortError") notice(e.message, true);
  }),
);
task($("intake-form"), async (data) => {
  requireConnection();
  const intake = Object.fromEntries(data.entries());
  intake.activities = data.getAll("activities");
  const c = editingCase
    ? await call("case_update_intake", {
        case_id: editingCase.id,
        version: editingCase.version,
        intake,
      })
    : await call("case_create", { intake, request_key: createKey });
  createKey = crypto.randomUUID();
  await loadCases();
  await loadCase(c.id);
  notice(
    "Preparation case saved. Start by confirming the premises jurisdiction.",
  );
});
task($("note-form"), async (data) => {
  requireConnection();
  if (!currentCase || example) return;
  await call("case_add_note", {
    case_id: currentCase.id,
    version: currentCase.version,
    ...Object.fromEntries(data.entries()),
  });
  $("note-form").reset();
  await loadCase(currentCase.id);
  notice("Note recorded as user-supplied information.");
});
$("export").addEventListener("click", async () => {
  try {
    requireConnection();
    if (!currentCase || example) return;
    download(await call("case_export", { case_id: currentCase.id }));
    notice(
      "Preparation brief downloaded. It includes the checklist, recorded notes, and an unsent agency inquiry.",
    );
  } catch (err) {
    if (err.name !== "AbortError") notice(err.message, true);
  }
});
task($("assistant-form"), async (data) => {
  requireConnection();
  if (!currentCase || example) return;
  const caseID = currentCase.id;
  $("assistant-answer").textContent =
    "Reviewing the case and available sources…";
  const result = await request("/api/agent", {
    method: "POST",
    body: JSON.stringify({ case_id: caseID, message: data.get("message") }),
  });
  if (currentCase?.id !== caseID) return;
  $("assistant-answer").replaceChildren(
    element(
      "p",
      "AI draft · verify before relying on this answer",
      "small muted",
    ),
    element("p", result.answer, "assistant-text"),
  );
  notice(
    "Assistant answer prepared. It has not changed the case or taken an external action.",
  );
  await updateUsage();
});
task($("reminder-form"), async (data) => {
  requireConnection();
  if (!currentCase || example) return;
  download(
    await call("case_reminder", {
      case_id: currentCase.id,
      date: data.get("date"),
    }),
  );
  notice(
    "Review reminder downloaded. Import it into your calendar if you want to use it.",
  );
});
showExample().catch((e) => {
  if (e.name !== "AbortError") notice(e.message, true);
});

request("/.well-known/permits-agent.json")
  .then((v) => {
    assistantEnabled = v.owner_assistant_enabled;
  })
  .catch((e) => {
    if (e.name !== "AbortError") notice(e.message, true);
  });

$("more-cases").addEventListener("click", () =>
  loadCases(true).catch((e) => {
    if (e.name !== "AbortError") notice(e.message, true);
  }),
);
$("edit-intake").addEventListener("click", () => {
  if (!currentCase || example) return;
  editingCase = currentCase;
  const form = $("intake-form");
  form.reset();
  for (const [name, value] of Object.entries(currentCase.intake)) {
    if (name === "activities") {
      for (const input of form.querySelectorAll('input[name="activities"]'))
        input.checked = value.includes(input.value);
    } else if (form.elements.namedItem(name))
      form.elements.namedItem(name).value = value;
  }
  $("save-intake").textContent = "Save business details ↗";
  $("cancel-edit").hidden = false;
  form.hidden = false;
  $("case-view").hidden = true;
  notice(
    "Update the intake. This will rebuild the research checklist and preserve existing notes.",
  );
});
$("cancel-edit").addEventListener("click", () => {
  editingCase = null;
  $("intake-form").hidden = true;
  $("case-view").hidden = false;
});

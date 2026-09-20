"use strict";
const analyticsNode = (id) => document.getElementById(id);
let analyticsToken = "",
  analyticsEpoch = 0,
  analyticsController = null;
function clearAnalytics() {
  analyticsEpoch++;
  analyticsController?.abort();
  analyticsNode("analytics-report").hidden = true;
  for (const id of [
    "analytics-window",
    "analytics-totals",
    "analytics-clients",
    "analytics-tools",
    "analytics-daily",
    "analytics-events",
    "analytics-notes",
  ])
    analyticsNode(id).replaceChildren();
}
function analyticsElement(tag, text) {
  const n = document.createElement(tag);
  if (text !== undefined) n.textContent = text;
  return n;
}
function analyticsRow(target, values) {
  const row = analyticsElement("tr");
  for (const value of values) row.append(analyticsElement("td", String(value)));
  analyticsNode(target).append(row);
}
function grouped(rows, fields) {
  const groups = new Map();
  for (const row of rows) {
    const key = JSON.stringify(fields.map((f) => row[f]));
    if (!groups.has(key))
      groups.set(key, {
        ...row,
        count: 0,
        failed_or_incomplete: 0,
        duration_ms_total: 0,
      });
    const item = groups.get(key);
    item.count += row.count;
    item.failed_or_incomplete += row.failed_or_incomplete;
    item.duration_ms_total += row.duration_ms_total;
  }
  return [...groups.values()].sort((a, b) => b.count - a.count);
}
function renderAnalytics(report) {
  analyticsNode("analytics-window").textContent =
    `${report.scope === "platform" ? "Whole product" : "This organization"} · ${report.window_start} through ${report.window_end} · rolling window`;
  for (const [label, key] of [
    ["Authenticated requests", "requests"],
    ["Failed / incomplete requests", "request_failures_or_incomplete"],
    ["Tool attempts", "tool_calls"],
    ["Owner-assistant model calls", "model_calls"],
    ["Input tokens", "input_tokens"],
    ["Output tokens", "output_tokens"],
  ]) {
    const card = analyticsElement("div");
    card.className = "metric";
    card.append(
      analyticsElement("span", label),
      analyticsElement("strong", report.totals[key].toLocaleString()),
    );
    analyticsNode("analytics-totals").append(card);
  }
  const requests = report.rows.filter((r) => r.kind === "request");
  for (const r of grouped(requests, [
    "client_id",
    "actor_kind",
    "agent_family",
    "channel",
  ]))
    analyticsRow("analytics-clients", [
      r.client_id,
      r.actor_kind,
      r.agent_family,
      r.channel,
      r.count,
      r.failed_or_incomplete,
    ]);
  for (const r of grouped(
    report.rows.filter((r) => r.kind === "tool"),
    ["client_id", "actor_kind", "agent_family", "operation", "executor"],
  ))
    analyticsRow("analytics-tools", [
      r.client_id,
      r.actor_kind,
      r.agent_family,
      r.operation,
      r.executor,
      r.count,
      r.failed_or_incomplete,
      `${Math.round(r.duration_ms_total / r.count)} ms`,
    ]);
  const days = grouped(requests, ["day_utc"]).sort((a, b) =>
    a.day_utc.localeCompare(b.day_utc),
  );
  const peak = Math.max(1, ...days.map((r) => r.count));
  for (const day of days) {
    const row = analyticsElement("div");
    row.className = "daily-row";
    const bar = analyticsElement("progress");
    bar.max = peak;
    bar.value = day.count;
    bar.setAttribute("aria-label", `${day.day_utc}: ${day.count} requests`);
    row.append(
      analyticsElement("span", day.day_utc),
      bar,
      analyticsElement("span", day.count),
    );
    analyticsNode("analytics-daily").append(row);
  }
  for (const row of report.rows)
    analyticsRow("analytics-events", [
      row.day_utc,
      row.client_id,
      row.actor_kind,
      row.agent_family,
      row.channel,
      row.executor,
      row.kind,
      row.operation,
      row.count,
      row.input_tokens,
      row.output_tokens,
    ]);
  for (const note of report.notes)
    analyticsNode("analytics-notes").append(analyticsElement("li", note));
  analyticsNode("analytics-report").hidden = false;
  analyticsNode("analytics-status").textContent = report.rows_truncated
    ? "Totals cover the full window. Tables and daily bars show only the first 2,000 groups; select a shorter window."
    : report.rows.length
      ? "Measured traffic loaded."
      : "No measured traffic in this window. This does not include local connector use or public page visits.";
}
analyticsNode("analytics-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  clearAnalytics();
  const epoch = analyticsEpoch;
  const entered = analyticsNode("analytics-key").value.trim();
  if (entered) analyticsToken = entered;
  analyticsNode("analytics-key").value = "";
  if (!analyticsToken) {
    analyticsNode("analytics-status").textContent =
      "Enter an authorized analytics key.";
    return;
  }
  analyticsController = new AbortController();
  analyticsNode("analytics-status").textContent = "Loading measured traffic…";
  try {
    const query = new URLSearchParams({
      scope: analyticsNode("analytics-scope").value,
      days: analyticsNode("analytics-days").value,
    });
    const response = await fetch(`/api/analytics?${query}`, {
      headers: { Authorization: `Bearer ${analyticsToken}` },
      signal: AbortSignal.any([
        analyticsController.signal,
        AbortSignal.timeout(30000),
      ]),
      cache: "no-store",
    });
    const report = await response.json();
    if (epoch !== analyticsEpoch) return;
    if (!response.ok) {
      if (response.status === 401) analyticsToken = "";
      throw new Error(report.error || "Analytics could not be loaded.");
    }
    renderAnalytics(report);
  } catch (err) {
    if (epoch === analyticsEpoch && err.name !== "AbortError")
      analyticsNode("analytics-status").textContent =
        err.name === "TimeoutError"
          ? "The request timed out. Load analytics again."
          : err.message;
  }
});
analyticsNode("analytics-disconnect").addEventListener("click", () => {
  clearAnalytics();
  analyticsToken = "";
  analyticsNode("analytics-key").value = "";
  analyticsNode("analytics-status").textContent =
    "Disconnected. Key and analytics cleared.";
});
for (const id of ["analytics-key", "analytics-scope", "analytics-days"])
  analyticsNode(id).addEventListener("input", () => {
    clearAnalytics();
    if (id === "analytics-key") analyticsToken = "";
    analyticsNode("analytics-status").textContent =
      "Load analytics to apply your selection.";
  });

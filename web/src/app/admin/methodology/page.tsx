"use client";

import { useState, useEffect, useCallback } from "react";
import {
  adminMethodologyApi,
  type MentorMethodology,
  type MethodologyStat,
  type MethodologyWriteReq,
  type MethodologyPreviewResponse,
} from "@/lib/admin-api";

// ── Helpers ────────────────────────────────────────────────────

const CATEGORIES = ["reference", "mental_model", "heuristic", "routing"] as const;

function CategoryBadge({ category }: { category: string }) {
  const styles: Record<string, string> = {
    reference: "bg-blue-500/10 text-blue-400 border-blue-500/30",
    mental_model: "bg-purple-500/10 text-purple-400 border-purple-500/30",
    heuristic: "bg-green-500/10 text-green-400 border-green-500/30",
    routing: "bg-orange-500/10 text-orange-400 border-orange-500/30",
  };
  return (
    <span
      className={`inline-block rounded border px-1.5 py-0.5 text-[10px] font-medium ${
        styles[category] || styles.reference
      }`}
    >
      {category}
    </span>
  );
}

function SourceBadge({ source }: { source: string }) {
  const isInternal = source === "internal-ensoul";
  return (
    <span
      className={`inline-block rounded border px-1.5 py-0.5 text-[10px] font-medium ${
        isInternal
          ? "bg-cyan-500/10 text-cyan-400 border-cyan-500/30"
          : "bg-gray-500/10 text-gray-400 border-gray-500/30"
      }`}
    >
      {source}
    </span>
  );
}

const emptyForm: MethodologyWriteReq = {
  category: "heuristic",
  slug: "",
  locale: "zh",
  title: "",
  summary: "",
  body_md: "",
  tags: "",
  priority: 50,
  enabled: true,
};

// ── Page ───────────────────────────────────────────────────────

export default function MethodologyAdminPage() {
  const [records, setRecords] = useState<MentorMethodology[]>([]);
  const [stats, setStats] = useState<MethodologyStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");

  // filters
  const [fCategory, setFCategory] = useState("");
  const [fSource, setFSource] = useState("");
  const [fEnabled, setFEnabled] = useState("");
  const [fQ, setFQ] = useState("");

  // edit state
  const [editing, setEditing] = useState<MentorMethodology | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<MethodologyWriteReq>(emptyForm);
  const [saving, setSaving] = useState(false);

  // preview tester
  const [previewMsg, setPreviewMsg] = useState("");
  const [previewResult, setPreviewResult] = useState<MethodologyPreviewResponse | null>(
    null
  );
  const [previewLoading, setPreviewLoading] = useState(false);
  const [showFullPrompt, setShowFullPrompt] = useState(false);

  // feedback stats
  const [feedbackRows, setFeedbackRows] = useState<
    { scenario: string; up: number; down: number; total: number }[]
  >([]);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await adminMethodologyApi.list({
        category: fCategory,
        source: fSource,
        enabled: fEnabled,
        q: fQ,
      });
      setRecords(res.records || []);
      setStats(res.stats || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    } finally {
      setLoading(false);
    }
  }, [fCategory, fSource, fEnabled, fQ]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    adminMethodologyApi
      .feedback()
      .then((r) => setFeedbackRows(r.rows || []))
      .catch(() => {});
  }, []);

  function startCreate() {
    setEditing(null);
    setCreating(true);
    setForm(emptyForm);
  }

  function startEdit(r: MentorMethodology) {
    setCreating(false);
    setEditing(r);
    setForm({
      category: r.category,
      slug: r.slug,
      locale: r.locale,
      title: r.title,
      summary: r.summary,
      body_md: r.body_md,
      tags: r.tags,
      priority: r.priority,
      enabled: r.enabled,
    });
  }

  function closeForm() {
    setCreating(false);
    setEditing(null);
  }

  async function save() {
    setSaving(true);
    setError("");
    setMsg("");
    try {
      if (creating) {
        await adminMethodologyApi.create(form);
        setMsg("Created.");
      } else if (editing) {
        const force = editing.source !== "internal-ensoul";
        await adminMethodologyApi.update(editing.id, form, force);
        setMsg(force ? "Updated (force)." : "Updated.");
      }
      closeForm();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "save failed");
    } finally {
      setSaving(false);
    }
  }

  async function remove(r: MentorMethodology, hard: boolean) {
    const label = hard ? "HARD DELETE" : "disable";
    if (!confirm(`${label} "${r.slug}"?`)) return;
    setError("");
    setMsg("");
    try {
      await adminMethodologyApi.delete(r.id, hard);
      setMsg(hard ? "Hard deleted." : "Disabled (soft delete).");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    }
  }

  async function runPreview() {
    if (!previewMsg.trim()) return;
    setPreviewLoading(true);
    try {
      const res = await adminMethodologyApi.preview(previewMsg);
      setPreviewResult(res);
      setShowFullPrompt(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "preview failed");
    } finally {
      setPreviewLoading(false);
    }
  }

  // ── Render ───────────────────────────────────────────────────

  return (
    <div className="space-y-6 p-6 text-sm text-[#e5e5ec]">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Methodology</h1>
          <p className="text-xs text-[#8a8a9a]">
            Mentor knowledge base. Records are loaded into LLM prompts based on
            scenario detection (writing/topic/review/growth/diagnosis).
          </p>
        </div>
        <button
          onClick={startCreate}
          className="rounded bg-cyan-600 px-4 py-2 text-sm font-medium hover:bg-cyan-500"
        >
          + New Record
        </button>
      </div>

      {error && (
        <div className="rounded border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-300">
          {error}
        </div>
      )}
      {msg && (
        <div className="rounded border border-green-500/30 bg-green-500/10 px-3 py-2 text-xs text-green-300">
          {msg}
        </div>
      )}

      {/* Stats */}
      <div className="rounded border border-[#2a2a3a] bg-[#15151f] p-4">
        <h2 className="mb-2 text-xs font-semibold uppercase text-[#8a8a9a]">
          Statistics
        </h2>
        <div className="flex flex-wrap gap-2">
          {stats.length === 0 ? (
            <span className="text-xs text-[#5a5a6a]">No records.</span>
          ) : (
            stats.map((s) => (
              <div
                key={`${s.category}-${s.source}`}
                className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1 text-xs"
              >
                <CategoryBadge category={s.category} />{" "}
                <SourceBadge source={s.source} />{" "}
                <span className="font-mono text-[#c5c5d5]">{s.n}</span>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Feedback by scenario (last 30d) */}
      <div className="rounded border border-[#2a2a3a] bg-[#15151f] p-4">
        <h2 className="mb-2 text-xs font-semibold uppercase text-[#8a8a9a]">
          User Feedback by Scenario (last 30 days)
        </h2>
        {feedbackRows.length === 0 ? (
          <span className="text-xs text-[#5a5a6a]">No feedback yet.</span>
        ) : (
          <table className="w-full text-xs">
            <thead className="text-[#8a8a9a]">
              <tr>
                <th className="px-2 py-1 text-left">Scenario</th>
                <th className="px-2 py-1 text-right">👍</th>
                <th className="px-2 py-1 text-right">👎</th>
                <th className="px-2 py-1 text-right">Total msgs</th>
                <th className="px-2 py-1 text-right">Net rate</th>
              </tr>
            </thead>
            <tbody>
              {feedbackRows.map((r) => {
                const rated = r.up + r.down;
                const net = rated > 0 ? ((r.up - r.down) / rated) * 100 : 0;
                return (
                  <tr key={r.scenario} className="border-t border-[#2a2a3a]">
                    <td className="px-2 py-1 font-mono text-[#c5c5d5]">
                      {r.scenario}
                    </td>
                    <td className="px-2 py-1 text-right text-emerald-400">{r.up}</td>
                    <td className="px-2 py-1 text-right text-red-400">{r.down}</td>
                    <td className="px-2 py-1 text-right text-[#c5c5d5]">{r.total}</td>
                    <td
                      className={`px-2 py-1 text-right font-mono ${
                        rated === 0
                          ? "text-[#5a5a6a]"
                          : net >= 0
                          ? "text-emerald-400"
                          : "text-red-400"
                      }`}
                    >
                      {rated === 0 ? "—" : `${net.toFixed(0)}%`}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Preview tester */}
      <div className="rounded border border-[#2a2a3a] bg-[#15151f] p-4">
        <h2 className="mb-2 text-xs font-semibold uppercase text-[#8a8a9a]">
          Scenario Preview Tester
        </h2>
        <div className="flex gap-2">
          <input
            value={previewMsg}
            onChange={(e) => setPreviewMsg(e.target.value)}
            placeholder='e.g. "帮我看看这条推文写得怎么样"'
            className="flex-1 rounded border border-[#2a2a3a] bg-[#0a0a14] px-3 py-2 text-sm focus:border-cyan-500 focus:outline-none"
          />
          <button
            onClick={runPreview}
            disabled={previewLoading || !previewMsg.trim()}
            className="rounded bg-purple-600 px-4 py-2 text-sm font-medium hover:bg-purple-500 disabled:opacity-50"
          >
            {previewLoading ? "..." : "Preview"}
          </button>
        </div>
        {previewResult && (
          <div className="mt-3 space-y-2 text-xs">
            <div className="flex flex-wrap gap-2">
              <span className="rounded border border-orange-500/30 bg-orange-500/10 px-2 py-1 font-mono text-orange-300">
                scenario: {previewResult.scenario}
              </span>
              <span className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1 text-[#c5c5d5]">
                heuristics: {previewResult.heuristics}
              </span>
              <span className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1 text-[#c5c5d5]">
                refs: {previewResult.references}
              </span>
              <span className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1 text-[#c5c5d5]">
                models: {previewResult.mental_models}
              </span>
              <span className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1 text-[#c5c5d5]">
                prompt: {previewResult.prompt_chars} chars
              </span>
            </div>
            <div className="text-[#8a8a9a]">
              Used slugs:{" "}
              <span className="font-mono text-[#c5c5d5]">
                {previewResult.used_slugs.join(", ") || "—"}
              </span>
            </div>
            <button
              onClick={() => setShowFullPrompt((v) => !v)}
              className="text-cyan-400 hover:underline"
            >
              {showFullPrompt ? "Hide" : "Show"} rendered prompt
            </button>
            {showFullPrompt && (
              <pre className="max-h-96 overflow-auto rounded border border-[#2a2a3a] bg-[#0a0a14] p-3 text-[10px] leading-tight text-[#a5a5b5]">
                {previewResult.prompt}
              </pre>
            )}
          </div>
        )}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-end gap-2 rounded border border-[#2a2a3a] bg-[#15151f] p-4">
        <label className="flex flex-col gap-1 text-xs">
          <span className="text-[#8a8a9a]">Category</span>
          <select
            value={fCategory}
            onChange={(e) => setFCategory(e.target.value)}
            className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1"
          >
            <option value="">All</option>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs">
          <span className="text-[#8a8a9a]">Source</span>
          <input
            value={fSource}
            onChange={(e) => setFSource(e.target.value)}
            placeholder="any"
            className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1"
          />
        </label>
        <label className="flex flex-col gap-1 text-xs">
          <span className="text-[#8a8a9a]">Enabled</span>
          <select
            value={fEnabled}
            onChange={(e) => setFEnabled(e.target.value)}
            className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1"
          >
            <option value="">All</option>
            <option value="true">Enabled</option>
            <option value="false">Disabled</option>
          </select>
        </label>
        <label className="flex flex-1 flex-col gap-1 text-xs">
          <span className="text-[#8a8a9a]">Search (title/summary/tags)</span>
          <input
            value={fQ}
            onChange={(e) => setFQ(e.target.value)}
            placeholder="..."
            className="rounded border border-[#2a2a3a] bg-[#0a0a14] px-2 py-1"
          />
        </label>
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded border border-[#2a2a3a]">
        <table className="w-full text-xs">
          <thead className="bg-[#15151f] text-[#8a8a9a]">
            <tr>
              <th className="px-3 py-2 text-left">Category</th>
              <th className="px-3 py-2 text-left">Slug</th>
              <th className="px-3 py-2 text-left">Title</th>
              <th className="px-3 py-2 text-left">Source</th>
              <th className="px-3 py-2 text-left">Tags</th>
              <th className="px-3 py-2 text-right">Pri</th>
              <th className="px-3 py-2 text-center">On</th>
              <th className="px-3 py-2 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={8} className="px-3 py-6 text-center text-[#5a5a6a]">
                  Loading...
                </td>
              </tr>
            ) : records.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-3 py-6 text-center text-[#5a5a6a]">
                  No records.
                </td>
              </tr>
            ) : (
              records.map((r) => (
                <tr
                  key={r.id}
                  className="border-t border-[#2a2a3a] hover:bg-[#15151f]"
                >
                  <td className="px-3 py-2">
                    <CategoryBadge category={r.category} />
                  </td>
                  <td className="px-3 py-2 font-mono text-[#c5c5d5]">{r.slug}</td>
                  <td className="px-3 py-2">{r.title}</td>
                  <td className="px-3 py-2">
                    <SourceBadge source={r.source} />
                  </td>
                  <td className="px-3 py-2 text-[#8a8a9a]">{r.tags || "—"}</td>
                  <td className="px-3 py-2 text-right font-mono">{r.priority}</td>
                  <td className="px-3 py-2 text-center">
                    {r.enabled ? "✓" : "—"}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button
                      onClick={() => startEdit(r)}
                      className="mr-2 text-cyan-400 hover:underline"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => remove(r, false)}
                      className="mr-2 text-yellow-400 hover:underline"
                    >
                      Disable
                    </button>
                    {r.source === "internal-ensoul" && (
                      <button
                        onClick={() => remove(r, true)}
                        className="text-red-400 hover:underline"
                      >
                        Hard
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Edit modal */}
      {(creating || editing) && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
          <div className="max-h-[90vh] w-full max-w-3xl overflow-auto rounded border border-[#2a2a3a] bg-[#0a0a14] p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">
                {creating ? "New Methodology Record" : `Edit: ${editing?.slug}`}
              </h2>
              <button
                onClick={closeForm}
                className="text-[#8a8a9a] hover:text-white"
              >
                ✕
              </button>
            </div>

            {editing && editing.source !== "internal-ensoul" && (
              <div className="mb-3 rounded border border-yellow-500/30 bg-yellow-500/10 px-3 py-2 text-xs text-yellow-300">
                ⚠ This record is from <code>{editing.source}</code>. Saving will
                overwrite it but next <code>seed --force</code> will revert.
              </div>
            )}

            <div className="grid grid-cols-2 gap-3">
              <label className="flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Category</span>
                <select
                  value={form.category}
                  onChange={(e) => setForm({ ...form, category: e.target.value })}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                >
                  {CATEGORIES.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Slug *</span>
                <input
                  value={form.slug}
                  onChange={(e) => setForm({ ...form, slug: e.target.value })}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5 font-mono"
                />
              </label>
              <label className="col-span-2 flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Title *</span>
                <input
                  value={form.title}
                  onChange={(e) => setForm({ ...form, title: e.target.value })}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                />
              </label>
              <label className="col-span-2 flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Summary</span>
                <input
                  value={form.summary || ""}
                  onChange={(e) => setForm({ ...form, summary: e.target.value })}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Tags (comma)</span>
                <input
                  value={form.tags || ""}
                  onChange={(e) => setForm({ ...form, tags: e.target.value })}
                  placeholder="topic,review,growth,diagnosis,writing,general"
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Locale</span>
                <input
                  value={form.locale || "zh"}
                  onChange={(e) => setForm({ ...form, locale: e.target.value })}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                />
              </label>
              <label className="flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Priority (higher first)</span>
                <input
                  type="number"
                  value={form.priority ?? 50}
                  onChange={(e) =>
                    setForm({ ...form, priority: Number(e.target.value) })
                  }
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5"
                />
              </label>
              <label className="flex items-center gap-2 text-xs">
                <input
                  type="checkbox"
                  checked={form.enabled !== false}
                  onChange={(e) =>
                    setForm({ ...form, enabled: e.target.checked })
                  }
                />
                <span>Enabled</span>
              </label>
              <label className="col-span-2 flex flex-col gap-1 text-xs">
                <span className="text-[#8a8a9a]">Body (Markdown) *</span>
                <textarea
                  value={form.body_md}
                  onChange={(e) => setForm({ ...form, body_md: e.target.value })}
                  rows={14}
                  className="rounded border border-[#2a2a3a] bg-[#15151f] px-2 py-1.5 font-mono text-xs"
                />
              </label>
            </div>

            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={closeForm}
                className="rounded border border-[#2a2a3a] px-4 py-2 text-sm hover:bg-[#15151f]"
              >
                Cancel
              </button>
              <button
                onClick={save}
                disabled={saving || !form.slug || !form.title || !form.body_md}
                className="rounded bg-cyan-600 px-4 py-2 text-sm font-medium hover:bg-cyan-500 disabled:opacity-50"
              >
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

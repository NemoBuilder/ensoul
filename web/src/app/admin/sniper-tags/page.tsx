"use client";

import { useState, useEffect, useCallback } from "react";
import {
  adminSniperApi,
  type SniperTag,
} from "@/lib/admin-api";

// ── Reusable badge ─────────────────────────────────────────────

function CategoryBadge({ category }: { category: string }) {
  const styles: Record<string, string> = {
    ecosystem: "bg-blue-500/10 text-blue-400 border-blue-500/30",
    track: "bg-purple-500/10 text-purple-400 border-purple-500/30",
    custom: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
  };
  return (
    <span
      className={`inline-block rounded-full border px-2 py-0.5 text-xs font-medium ${
        styles[category] || styles.custom
      }`}
    >
      {category || "custom"}
    </span>
  );
}

// ── Tag Form (create / edit) ───────────────────────────────────

interface TagFormData {
  id: string;
  name: string;
  name_en: string;
  icon: string;
  category: string;
  description: string;
  is_default: boolean;
  sort_order: number;
}

const emptyTag: TagFormData = {
  id: "",
  name: "",
  name_en: "",
  icon: "🔹",
  category: "ecosystem",
  description: "",
  is_default: false,
  sort_order: 0,
};

function TagFormModal({
  initial,
  isEdit,
  onSave,
  onCancel,
  saving,
}: {
  initial: TagFormData;
  isEdit: boolean;
  onSave: (data: TagFormData) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const [form, setForm] = useState<TagFormData>(initial);

  const update = (key: keyof TagFormData, value: unknown) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-lg rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-6">
        <h3 className="mb-4 text-lg font-bold text-[#e2e8f0]">
          {isEdit ? "Edit Tag" : "Create Tag"}
        </h3>

        <div className="space-y-3">
          {/* ID (only for create) */}
          {!isEdit && (
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">
                Tag ID (slug, e.g. &quot;bnb_official&quot;)
              </label>
              <input
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.id}
                onChange={(e) => update("id", e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, "_"))}
                placeholder="my_tag_id"
              />
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">Name (中文)</label>
              <input
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.name}
                onChange={(e) => update("name", e.target.value)}
                placeholder="BNB Chain 官方"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">Name (EN)</label>
              <input
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.name_en}
                onChange={(e) => update("name_en", e.target.value)}
                placeholder="BNB Chain Official"
              />
            </div>
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">Icon (emoji)</label>
              <input
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.icon}
                onChange={(e) => update("icon", e.target.value)}
                placeholder="🔹"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">Category</label>
              <select
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.category}
                onChange={(e) => update("category", e.target.value)}
              >
                <option value="ecosystem">ecosystem</option>
                <option value="track">track</option>
                <option value="custom">custom</option>
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">Sort Order</label>
              <input
                type="number"
                className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                value={form.sort_order}
                onChange={(e) => update("sort_order", parseInt(e.target.value) || 0)}
              />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs text-[#94a3b8]">Description</label>
            <textarea
              className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
              rows={2}
              value={form.description}
              onChange={(e) => update("description", e.target.value)}
              placeholder="Tag description..."
            />
          </div>

          <label className="flex items-center gap-2 text-sm text-[#94a3b8]">
            <input
              type="checkbox"
              checked={form.is_default}
              onChange={(e) => update("is_default", e.target.checked)}
              className="accent-[#8b5cf6]"
            />
            Default (auto-selected for new users)
          </label>
        </div>

        <div className="mt-5 flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg border border-[#2a2a3e] px-4 py-2 text-sm text-[#94a3b8] hover:bg-[#1e1e2e]"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(form)}
            disabled={saving || (!isEdit && !form.id) || !form.name}
            className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed] disabled:opacity-40"
          >
            {saving ? "Saving..." : isEdit ? "Update" : "Create"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Add Account Modal ──────────────────────────────────────────

function AddAccountModal({
  tagId,
  onSave,
  onCancel,
  saving,
}: {
  tagId: string;
  onSave: (handle: string, displayName: string, realtime: boolean) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const [handle, setHandle] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [realtime, setRealtime] = useState(false);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-md rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-6">
        <h3 className="mb-4 text-lg font-bold text-[#e2e8f0]">
          Add Account to <span className="text-[#8b5cf6]">{tagId}</span>
        </h3>

        <div className="space-y-3">
          <div>
            <label className="mb-1 block text-xs text-[#94a3b8]">Twitter Handle</label>
            <input
              className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
              value={handle}
              onChange={(e) => setHandle(e.target.value)}
              placeholder="@elonmusk"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-[#94a3b8]">Display Name (optional)</label>
            <input
              className="w-full rounded-lg border border-[#2a2a3e] bg-[#12121a] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Elon Musk"
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-[#94a3b8]">
            <input
              type="checkbox"
              checked={realtime}
              onChange={(e) => setRealtime(e.target.checked)}
              className="accent-[#8b5cf6]"
            />
            Realtime Priority (allocate to Twitter Stream)
          </label>
        </div>

        <div className="mt-5 flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg border border-[#2a2a3e] px-4 py-2 text-sm text-[#94a3b8] hover:bg-[#1e1e2e]"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(handle, displayName, realtime)}
            disabled={saving || !handle.replace("@", "")}
            className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed] disabled:opacity-40"
          >
            {saving ? "Adding..." : "Add Account"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Main Page ──────────────────────────────────────────────────

export default function SniperTagsPage() {
  const [tags, setTags] = useState<SniperTag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");

  // Tag form modal
  const [showTagForm, setShowTagForm] = useState(false);
  const [editingTag, setEditingTag] = useState<TagFormData | null>(null);
  const [savingTag, setSavingTag] = useState(false);

  // Account modal
  const [addAccountTagId, setAddAccountTagId] = useState<string | null>(null);
  const [savingAccount, setSavingAccount] = useState(false);

  // Expanded tags (show accounts)
  const [expandedTags, setExpandedTags] = useState<Set<string>>(new Set());

  // ── Load tags ────────────────────────────────────────────────

  const loadTags = useCallback(async () => {
    try {
      setLoading(true);
      const res = await adminSniperApi.listTags();
      setTags(res.tags || []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load tags");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadTags();
  }, [loadTags]);

  useEffect(() => {
    if (actionMsg) {
      const t = setTimeout(() => setActionMsg(""), 4000);
      return () => clearTimeout(t);
    }
  }, [actionMsg]);

  // ── Toggle expand ────────────────────────────────────────────

  const toggleExpand = (tagId: string) => {
    setExpandedTags((prev) => {
      const next = new Set(prev);
      if (next.has(tagId)) next.delete(tagId);
      else next.add(tagId);
      return next;
    });
  };

  // ── Tag CRUD ─────────────────────────────────────────────────

  const handleCreateTag = () => {
    setEditingTag(null);
    setShowTagForm(true);
  };

  const handleEditTag = (tag: SniperTag) => {
    setEditingTag({
      id: tag.id,
      name: tag.name,
      name_en: tag.name_en,
      icon: tag.icon,
      category: tag.category,
      description: tag.description,
      is_default: tag.is_default,
      sort_order: tag.sort_order,
    });
    setShowTagForm(true);
  };

  const handleSaveTag = async (data: TagFormData) => {
    setSavingTag(true);
    setError("");
    try {
      if (editingTag) {
        // Update existing tag
        await adminSniperApi.updateTag(data.id, {
          name: data.name,
          name_en: data.name_en,
          icon: data.icon,
          category: data.category,
          description: data.description,
          is_default: data.is_default,
          sort_order: data.sort_order,
        });
        setActionMsg(`Updated tag "${data.id}"`);
      } else {
        // Create new tag
        await adminSniperApi.createTag(data);
        setActionMsg(`Created tag "${data.id}"`);
      }
      setShowTagForm(false);
      loadTags();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save tag");
    } finally {
      setSavingTag(false);
    }
  };

  const handleDeleteTag = async (tagId: string, tagName: string) => {
    if (!confirm(`Delete tag "${tagName}" (${tagId})? This will remove all associated accounts.`))
      return;
    try {
      await adminSniperApi.deleteTag(tagId);
      setActionMsg(`Deleted tag "${tagId}"`);
      loadTags();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to delete tag");
    }
  };

  // ── Account CRUD ─────────────────────────────────────────────

  const handleAddAccount = async (handle: string, displayName: string, realtime: boolean) => {
    if (!addAccountTagId) return;
    setSavingAccount(true);
    setError("");
    try {
      await adminSniperApi.addTagAccount(addAccountTagId, handle, displayName, realtime);
      setActionMsg(`Added @${handle.replace("@", "")} to ${addAccountTagId}`);
      setAddAccountTagId(null);
      loadTags();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add account");
    } finally {
      setSavingAccount(false);
    }
  };

  const handleRemoveAccount = async (tagId: string, handle: string) => {
    if (!confirm(`Remove @${handle} from tag "${tagId}"?`)) return;
    try {
      await adminSniperApi.removeTagAccount(tagId, handle);
      setActionMsg(`Removed @${handle} from ${tagId}`);
      loadTags();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to remove account");
    }
  };

  // ── Render ───────────────────────────────────────────────────

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-[#e2e8f0]">🏷️ Sniper Tags &amp; KOL Accounts</h2>
          <p className="mt-1 text-sm text-[#64748b]">
            Manage content tags and their associated Twitter/KOL accounts
          </p>
        </div>
        <button
          onClick={handleCreateTag}
          className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed]"
        >
          + New Tag
        </button>
      </div>

      {/* Messages */}
      {error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          {error}
          <button onClick={() => setError("")} className="ml-2 text-red-300 hover:text-red-200">
            ✕
          </button>
        </div>
      )}
      {actionMsg && (
        <div className="rounded-lg border border-green-500/30 bg-green-500/10 px-4 py-3 text-sm text-green-400">
          ✓ {actionMsg}
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="flex justify-center py-12">
          <div className="text-[#94a3b8]">Loading tags...</div>
        </div>
      )}

      {/* Tags list */}
      {!loading && tags.length === 0 && (
        <div className="rounded-xl border border-[#1e1e2e] bg-[#12121a] p-12 text-center">
          <div className="text-4xl">🏷️</div>
          <p className="mt-2 text-[#94a3b8]">No tags yet. Create your first tag to get started.</p>
        </div>
      )}

      {!loading &&
        tags.map((tag) => {
          const isExpanded = expandedTags.has(tag.id);
          const accountCount = tag.accounts?.length || 0;

          return (
            <div
              key={tag.id}
              className="rounded-xl border border-[#1e1e2e] bg-[#12121a] overflow-hidden"
            >
              {/* Tag header row */}
              <div className="flex items-center gap-3 px-4 py-3">
                {/* Expand button */}
                <button
                  onClick={() => toggleExpand(tag.id)}
                  className="text-[#64748b] hover:text-[#e2e8f0] transition-transform"
                  style={{ transform: isExpanded ? "rotate(90deg)" : "rotate(0deg)" }}
                >
                  ▶
                </button>

                {/* Icon + Name */}
                <span className="text-xl">{tag.icon || "🔹"}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-[#e2e8f0]">{tag.name}</span>
                    {tag.name_en && (
                      <span className="text-sm text-[#64748b]">({tag.name_en})</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2 mt-0.5">
                    <code className="text-xs text-[#8b5cf6]/70">{tag.id}</code>
                    <CategoryBadge category={tag.category} />
                    {tag.is_default && (
                      <span className="rounded-full bg-green-500/10 border border-green-500/30 px-2 py-0.5 text-xs text-green-400">
                        default
                      </span>
                    )}
                    {!tag.active && (
                      <span className="rounded-full bg-red-500/10 border border-red-500/30 px-2 py-0.5 text-xs text-red-400">
                        inactive
                      </span>
                    )}
                  </div>
                </div>

                {/* Account count */}
                <div className="text-sm text-[#64748b]">
                  <span className="font-medium text-[#94a3b8]">{accountCount}</span> accounts
                </div>

                {/* Sort order */}
                <div className="text-xs text-[#4a4a5a]">#{tag.sort_order}</div>

                {/* Actions */}
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setAddAccountTagId(tag.id)}
                    className="rounded-lg px-2 py-1 text-xs text-[#8b5cf6] hover:bg-[#8b5cf6]/10"
                    title="Add account"
                  >
                    + KOL
                  </button>
                  <button
                    onClick={() => handleEditTag(tag)}
                    className="rounded-lg px-2 py-1 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] hover:text-[#e2e8f0]"
                    title="Edit tag"
                  >
                    ✏️
                  </button>
                  <button
                    onClick={() => handleDeleteTag(tag.id, tag.name)}
                    className="rounded-lg px-2 py-1 text-xs text-red-400/60 hover:bg-red-500/10 hover:text-red-400"
                    title="Delete tag"
                  >
                    🗑️
                  </button>
                </div>
              </div>

              {/* Description */}
              {tag.description && (
                <div className="px-4 pb-2 pl-14 text-xs text-[#64748b]">{tag.description}</div>
              )}

              {/* Expanded: account list */}
              {isExpanded && (
                <div className="border-t border-[#1e1e2e]">
                  {accountCount === 0 ? (
                    <div className="px-4 py-6 text-center text-sm text-[#4a4a5a]">
                      No accounts in this tag.{" "}
                      <button
                        onClick={() => setAddAccountTagId(tag.id)}
                        className="text-[#8b5cf6] hover:underline"
                      >
                        Add one
                      </button>
                    </div>
                  ) : (
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-[#1e1e2e] text-left text-xs text-[#64748b]">
                          <th className="px-4 py-2 pl-14">Handle</th>
                          <th className="px-4 py-2">Display Name</th>
                          <th className="px-4 py-2">Realtime</th>
                          <th className="px-4 py-2 text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {tag.accounts.map((acct) => (
                          <tr
                            key={acct.handle}
                            className="border-b border-[#1e1e2e]/50 hover:bg-[#1e1e2e]/30"
                          >
                            <td className="px-4 py-2 pl-14">
                              <a
                                href={`https://x.com/${acct.handle}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-[#8b5cf6] hover:underline"
                              >
                                @{acct.handle}
                              </a>
                            </td>
                            <td className="px-4 py-2 text-[#94a3b8]">
                              {acct.name || "—"}
                            </td>
                            <td className="px-4 py-2">
                              {acct.realtime_priority ? (
                                <span className="text-green-400">⚡ Yes</span>
                              ) : (
                                <span className="text-[#4a4a5a]">No</span>
                              )}
                            </td>
                            <td className="px-4 py-2 text-right">
                              <button
                                onClick={() => handleRemoveAccount(tag.id, acct.handle)}
                                className="rounded px-2 py-1 text-xs text-red-400/60 hover:bg-red-500/10 hover:text-red-400"
                              >
                                Remove
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
              )}
            </div>
          );
        })}

      {/* Modals */}
      {showTagForm && (
        <TagFormModal
          initial={editingTag || emptyTag}
          isEdit={!!editingTag}
          onSave={handleSaveTag}
          onCancel={() => setShowTagForm(false)}
          saving={savingTag}
        />
      )}

      {addAccountTagId && (
        <AddAccountModal
          tagId={addAccountTagId}
          onSave={handleAddAccount}
          onCancel={() => setAddAccountTagId(null)}
          saving={savingAccount}
        />
      )}
    </div>
  );
}

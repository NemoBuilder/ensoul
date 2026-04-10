"use client";

import { useState, useEffect, useCallback } from "react";
import {
  adminCandidatesApi,
  adminTaxWalletApi,
  type MintCandidate,
} from "@/lib/admin-api";

// ── Status badge ───────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-yellow-500/10 text-yellow-400 border-yellow-500/30",
    queued: "bg-blue-500/10 text-blue-400 border-blue-500/30",
    minted: "bg-green-500/10 text-green-400 border-green-500/30",
    skipped: "bg-gray-500/10 text-gray-400 border-gray-500/30",
    failed: "bg-red-500/10 text-red-400 border-red-500/30",
  };
  return (
    <span
      className={`inline-block rounded-full border px-2 py-0.5 text-xs font-medium ${
        styles[status] || styles.pending
      }`}
    >
      {status}
    </span>
  );
}

// ── Helper functions ───────────────────────────────────────────

function formatFollowers(count: number | undefined | null): string {
  if (count == null || count === 0) return "—";
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return count.toString();
}

function formatWei(wei: string | undefined | null): string {
  if (!wei || wei === "0") return "—";
  try {
    const num = Number(BigInt(wei)) / 1e18;
    return `${num} BNB`;
  } catch {
    return "—";
  }
}

function TierBadge({ tier }: { tier: string }) {
  const styles: Record<string, string> = {
    micro: "bg-gray-500/10 text-gray-400 border-gray-500/30",
    small: "bg-blue-500/10 text-blue-400 border-blue-500/30",
    medium: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
    large: "bg-purple-500/10 text-purple-400 border-purple-500/30",
    top: "bg-orange-500/10 text-orange-400 border-orange-500/30",
    super: "bg-red-500/10 text-red-400 border-red-500/30",
  };
  if (!tier) return <span className="text-[#4a4a5a]">—</span>;
  return (
    <span
      className={`inline-block rounded-full border px-2 py-0.5 text-xs font-medium ${
        styles[tier] || styles.micro
      }`}
    >
      {tier}
    </span>
  );
}

// ── Candidates Page ────────────────────────────────────────────

export default function CandidatesPage() {
  const [candidates, setCandidates] = useState<MintCandidate[]>([]);
  const [filter, setFilter] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");

  // Add form state
  const [showAdd, setShowAdd] = useState(false);
  const [addMode, setAddMode] = useState<"single" | "batch" | "import">("single");
  const [addHandle, setAddHandle] = useState("");
  const [addHandles, setAddHandles] = useState("");
  const [addPriority, setAddPriority] = useState(0);
  const [addReason, setAddReason] = useState("");
  const [addLoading, setAddLoading] = useState(false);
  const [refreshingHandle, setRefreshingHandle] = useState<string | null>(null);
  const [refreshingAll, setRefreshingAll] = useState(false);
  // Batch selection state
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [batchMinting, setBatchMinting] = useState(false);
  // Import following state
  const [importHandle, setImportHandle] = useState("");
  const [importMaxUsers, setImportMaxUsers] = useState(500);
  const [importMinFollowers, setImportMinFollowers] = useState(10000);
  const [importResult, setImportResult] = useState<{
    source_handle: string;
    total_following: number;
    fetched: number;
    added: number;
    skipped: number;
    filtered_out: number;
    errors: string[];
    api_calls_used: number;
  } | null>(null);

  const loadCandidates = useCallback(async () => {
    try {
      setLoading(true);
      const res = await adminCandidatesApi.list(filter || undefined);
      setCandidates(res.candidates);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    loadCandidates();
  }, [loadCandidates]);

  // Clear action messages after 4 seconds
  useEffect(() => {
    if (actionMsg) {
      const t = setTimeout(() => setActionMsg(""), 4000);
      return () => clearTimeout(t);
    }
  }, [actionMsg]);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setAddLoading(true);
    setError("");
    setImportResult(null);
    try {
      if (addMode === "single") {
        const c = await adminCandidatesApi.add(addHandle, addPriority, addReason);
        setActionMsg(`Added @${c.handle}`);
        setAddHandle("");
        setShowAdd(false);
      } else if (addMode === "batch") {
        const handles = addHandles
          .split(/[\n,]/)
          .map((h) => h.trim())
          .filter(Boolean);
        const res = await adminCandidatesApi.addBatch(handles, addPriority, addReason);
        setActionMsg(`Added ${res.added}, skipped ${res.skipped}`);
        setAddHandles("");
        setShowAdd(false);
      } else if (addMode === "import") {
        const res = await adminCandidatesApi.importFollowing(
          importHandle,
          importMaxUsers,
          importMinFollowers,
          addPriority,
          addReason || `following @${importHandle.replace("@", "")}`
        );
        setImportResult(res);
        setActionMsg(
          `Imported from @${res.source_handle}: ${res.added} added, ${res.skipped} skipped, ${res.filtered_out} filtered (<${importMinFollowers} followers)`
        );
      }
      setAddPriority(0);
      setAddReason("");
      loadCandidates();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add");
    } finally {
      setAddLoading(false);
    }
  };

  const handleRemove = async (handle: string) => {
    if (!confirm(`Remove @${handle} from candidates?`)) return;
    try {
      await adminCandidatesApi.remove(handle);
      setActionMsg(`Removed @${handle}`);
      loadCandidates();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to remove");
    }
  };

  const handleMintSingle = async (handle: string) => {
    if (!confirm(`Mint @${handle} now using tax wallet?`)) return;
    try {
      await adminTaxWalletApi.mintSingle(handle);
      setActionMsg(`Mint triggered for @${handle} (check server logs)`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to mint");
    }
  };

  const handleRefreshSingle = async (handle: string) => {
    setRefreshingHandle(handle);
    setError("");
    try {
      const updated = await adminCandidatesApi.refreshFollowers(handle);
      setActionMsg(`@${handle}: ${formatFollowers(updated.followers)} followers, ${updated.tier} tier`);
      loadCandidates();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to refresh");
    } finally {
      setRefreshingHandle(null);
    }
  };

  const handleRefreshAll = async () => {
    if (!confirm("Refresh follower data for all pending candidates? This may take a while.")) return;
    setRefreshingAll(true);
    setError("");
    try {
      const res = await adminCandidatesApi.refreshAll();
      setActionMsg(`Refreshed ${res.updated} candidates${res.errors.length ? `, ${res.errors.length} errors` : ""}`);
      loadCandidates();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to refresh all");
    } finally {
      setRefreshingAll(false);
    }
  };

  // Selection helpers
  const isMintable = (c: MintCandidate) =>
    c.status === "pending" || c.status === "queued" || c.status === "failed" || c.status === "skipped";
  const mintableCandidates = candidates.filter(isMintable);
  const allMintableSelected =
    mintableCandidates.length > 0 && mintableCandidates.every((c) => selected.has(c.handle));

  const toggleSelect = (handle: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(handle)) next.delete(handle);
      else next.add(handle);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (allMintableSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(mintableCandidates.map((c) => c.handle)));
    }
  };

  const handleBatchMint = async () => {
    const handles = Array.from(selected);
    if (handles.length === 0) return;
    if (!confirm(`Mint ${handles.length} selected candidate${handles.length > 1 ? "s" : ""} using tax wallet?`)) return;
    setBatchMinting(true);
    setError("");
    let ok = 0;
    let fail = 0;
    for (const h of handles) {
      try {
        await adminTaxWalletApi.mintSingle(h);
        ok++;
      } catch {
        fail++;
      }
    }
    setActionMsg(`Batch mint triggered: ${ok} started${fail ? `, ${fail} failed` : ""} (check server logs)`);
    setSelected(new Set());
    setBatchMinting(false);
    // Reload after a short delay to let background mints start
    setTimeout(loadCandidates, 2000);
  };

  return (
    <div className="space-y-4">
      {/* Header with actions */}
      <div className="flex flex-wrap items-center gap-3">
        {/* Filter */}
        <select
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="rounded-lg border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
        >
          <option value="">All statuses</option>
          <option value="pending">Pending</option>
          <option value="queued">Queued</option>
          <option value="minted">Minted</option>
          <option value="failed">Failed</option>
          <option value="skipped">Skipped</option>
        </select>

        <button
          onClick={() => setShowAdd(!showAdd)}
          className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa]"
        >
          {showAdd ? "Cancel" : "+ Add Candidate"}
        </button>

        <button
          onClick={loadCandidates}
          className="rounded-lg border border-[#1e1e2e] px-3 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
        >
          ↻ Refresh
        </button>

        <button
          onClick={handleRefreshAll}
          disabled={refreshingAll}
          className="rounded-lg border border-cyan-500/30 px-3 py-2 text-sm text-cyan-400 transition-colors hover:bg-cyan-500/10 disabled:opacity-50"
        >
          {refreshingAll ? "Refreshing..." : "⟳ Refresh All Followers"}
        </button>

        {selected.size > 0 && (
          <button
            onClick={handleBatchMint}
            disabled={batchMinting}
            className="rounded-lg bg-[#22c55e] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#16a34a] disabled:opacity-50"
          >
            {batchMinting
              ? `Minting ${selected.size}...`
              : `🚀 Mint Selected (${selected.size})`}
          </button>
        )}
      </div>

      {/* Messages */}
      {error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          {error}
          <button onClick={() => setError("")} className="ml-2 underline">dismiss</button>
        </div>
      )}
      {actionMsg && (
        <div className="rounded-lg border border-green-500/30 bg-green-500/10 px-4 py-3 text-sm text-green-400">
          {actionMsg}
        </div>
      )}

      {/* Add form */}
      {showAdd && (
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-4 flex gap-4">
            <button
              onClick={() => setAddMode("single")}
              className={`text-sm font-medium ${addMode === "single" ? "text-[#8b5cf6]" : "text-[#94a3b8]"}`}
            >
              Single
            </button>
            <button
              onClick={() => setAddMode("batch")}
              className={`text-sm font-medium ${addMode === "batch" ? "text-[#8b5cf6]" : "text-[#94a3b8]"}`}
            >
              Batch
            </button>
            <button
              onClick={() => setAddMode("import")}
              className={`text-sm font-medium ${addMode === "import" ? "text-[#8b5cf6]" : "text-[#94a3b8]"}`}
            >
              📥 Import Following
            </button>
          </div>

          <form onSubmit={handleAdd} className="space-y-3">
            {addMode === "single" ? (
              <input
                type="text"
                placeholder="@handle"
                value={addHandle}
                onChange={(e) => setAddHandle(e.target.value)}
                required
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
              />
            ) : addMode === "batch" ? (
              <textarea
                placeholder="One handle per line, or comma-separated"
                value={addHandles}
                onChange={(e) => setAddHandles(e.target.value)}
                required
                rows={4}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
              />
            ) : (
              <div className="space-y-3">
                <div className="rounded-lg border border-[#8b5cf6]/20 bg-[#8b5cf6]/5 px-3 py-2 text-xs text-[#c4b5fd]">
                  📥 Enter a KOL handle to import all accounts they follow as mint candidates.
                  <br />
                  Each followed account will be checked via SocialData API (~$0.0002/account).
                </div>
                <input
                  type="text"
                  placeholder="KOL handle, e.g. @elonmusk"
                  value={importHandle}
                  onChange={(e) => setImportHandle(e.target.value)}
                  required
                  className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                />
                <div className="flex gap-3">
                  <div className="flex-1">
                    <label className="mb-1 block text-xs text-[#4a4a5a]">Max accounts to fetch</label>
                    <input
                      type="number"
                      value={importMaxUsers}
                      onChange={(e) => setImportMaxUsers(Number(e.target.value))}
                      min={1}
                      max={2000}
                      className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="mb-1 block text-xs text-[#4a4a5a]">Min followers filter</label>
                    <input
                      type="number"
                      value={importMinFollowers}
                      onChange={(e) => setImportMinFollowers(Number(e.target.value))}
                      min={0}
                      className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                    />
                  </div>
                </div>
                <p className="text-xs text-[#4a4a5a]">
                  Estimated API cost: ~${((importMaxUsers / 20 + 1 + importMaxUsers) * 0.0002).toFixed(2)}
                  {" "}({Math.ceil(importMaxUsers / 20) + 1} page requests + up to {importMaxUsers} profile lookups)
                </p>
              </div>
            )}

            <div className="flex gap-3">
              <div className="flex-1">
                <label className="mb-1 block text-xs text-[#4a4a5a]">Priority</label>
                <input
                  type="number"
                  value={addPriority}
                  onChange={(e) => setAddPriority(Number(e.target.value))}
                  className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                />
              </div>
              <div className="flex-[2]">
                <label className="mb-1 block text-xs text-[#4a4a5a]">Reason (optional)</label>
                <input
                  type="text"
                  placeholder="Why this handle?"
                  value={addReason}
                  onChange={(e) => setAddReason(e.target.value)}
                  className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={addLoading}
              className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {addLoading
                ? addMode === "import" ? "Importing... (this may take a while)" : "Adding..."
                : addMode === "import" ? "📥 Import Following" : "Add"}
            </button>
          </form>

          {/* Import result summary */}
          {importResult && addMode === "import" && (
            <div className="mt-4 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-4 text-sm">
              <h4 className="mb-2 font-medium text-[#e2e8f0]">
                Import Result — @{importResult.source_handle}
              </h4>
              <div className="grid grid-cols-2 gap-x-6 gap-y-1 text-xs sm:grid-cols-4">
                <div>
                  <span className="text-[#4a4a5a]">Total following:</span>{" "}
                  <span className="text-[#e2e8f0]">{importResult.total_following.toLocaleString()}</span>
                </div>
                <div>
                  <span className="text-[#4a4a5a]">Fetched:</span>{" "}
                  <span className="text-[#e2e8f0]">{importResult.fetched}</span>
                </div>
                <div>
                  <span className="text-[#4a4a5a]">Added:</span>{" "}
                  <span className="text-green-400 font-medium">{importResult.added}</span>
                </div>
                <div>
                  <span className="text-[#4a4a5a]">Skipped:</span>{" "}
                  <span className="text-[#94a3b8]">{importResult.skipped}</span>
                </div>
                <div>
                  <span className="text-[#4a4a5a]">Filtered out:</span>{" "}
                  <span className="text-amber-400">{importResult.filtered_out}</span>
                </div>
                <div>
                  <span className="text-[#4a4a5a]">API calls:</span>{" "}
                  <span className="text-[#94a3b8]">~{importResult.api_calls_used} (~${(importResult.api_calls_used * 0.0002).toFixed(3)})</span>
                </div>
              </div>
              {importResult.errors.length > 0 && (
                <div className="mt-2">
                  <p className="text-xs text-red-400">{importResult.errors.length} errors:</p>
                  <div className="mt-1 max-h-24 overflow-y-auto text-xs text-red-400/70">
                    {importResult.errors.slice(0, 10).map((e, i) => (
                      <div key={i}>{e}</div>
                    ))}
                    {importResult.errors.length > 10 && (
                      <div>...and {importResult.errors.length - 10} more</div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Table */}
      {loading ? (
        <div className="text-sm text-[#94a3b8]">Loading...</div>
      ) : candidates.length === 0 ? (
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-sm text-[#4a4a5a]">
          No candidates found
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-[#1e1e2e] bg-[#14141f]">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#1e1e2e] text-left text-xs text-[#4a4a5a] uppercase">
                <th className="w-8 px-3 py-3">
                  <input
                    type="checkbox"
                    checked={allMintableSelected}
                    onChange={toggleSelectAll}
                    className="accent-[#8b5cf6] cursor-pointer"
                    title="Select all mintable"
                  />
                </th>
                <th className="px-4 py-3">Handle</th>
                <th className="px-4 py-3">Followers</th>
                <th className="px-4 py-3">Price</th>
                <th className="px-4 py-3">Tier</th>
                <th className="px-4 py-3">Priority</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Reason</th>
                <th className="px-4 py-3">Error</th>
                <th className="px-4 py-3">Added</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {candidates.map((c) => (
                <tr
                  key={c.id}
                  className={`border-b border-[#1e1e2e]/50 transition-colors hover:bg-[#1a1a2e] ${
                    selected.has(c.handle) ? "bg-[#8b5cf6]/5" : ""
                  }`}
                >
                  <td className="w-8 px-3 py-3">
                    {isMintable(c) ? (
                      <input
                        type="checkbox"
                        checked={selected.has(c.handle)}
                        onChange={() => toggleSelect(c.handle)}
                        className="accent-[#8b5cf6] cursor-pointer"
                      />
                    ) : (
                      <span className="block w-4" />
                    )}
                  </td>
                  <td className="px-4 py-3 font-mono text-[#e2e8f0]">@{c.handle}</td>
                  <td className="px-4 py-3 text-[#94a3b8]">{formatFollowers(c.followers)}</td>
                  <td className="px-4 py-3 text-[#94a3b8] font-mono">{formatWei(c.price_wei)}</td>
                  <td className="px-4 py-3">
                    <TierBadge tier={c.tier} />
                  </td>
                  <td className="px-4 py-3 text-[#94a3b8]">{c.priority}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={c.status} />
                  </td>
                  <td className="px-4 py-3 text-[#94a3b8] truncate max-w-[180px]">
                    {c.reason || "—"}
                  </td>
                  <td className="px-4 py-3 text-red-400 truncate max-w-[180px]">
                    {c.error_msg || "—"}
                  </td>
                  <td className="px-4 py-3 text-[#4a4a5a] whitespace-nowrap">
                    {new Date(c.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      {(c.status === "pending" || c.status === "queued" || c.status === "failed" || c.status === "skipped") && (
                        <button
                          onClick={() => handleMintSingle(c.handle)}
                          className="rounded px-2 py-1 text-xs text-[#22c55e] hover:bg-[#22c55e]/10"
                          title={c.status === "pending" || c.status === "queued" ? "Mint now" : "Retry mint"}
                        >
                          {c.status === "pending" || c.status === "queued" ? "Mint" : "Retry"}
                        </button>
                      )}
                      <button
                        onClick={() => handleRefreshSingle(c.handle)}
                        disabled={refreshingHandle === c.handle}
                        className="rounded px-2 py-1 text-xs text-cyan-400 hover:bg-cyan-500/10 disabled:opacity-50"
                        title="Refresh followers"
                      >
                        {refreshingHandle === c.handle ? "..." : "⟳"}
                      </button>
                      <button
                        onClick={() => handleRemove(c.handle)}
                        className="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-500/10"
                        title="Remove"
                      >
                        Remove
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="text-xs text-[#4a4a5a]">
        Total: {candidates.length} candidate{candidates.length !== 1 ? "s" : ""}
      </div>
    </div>
  );
}

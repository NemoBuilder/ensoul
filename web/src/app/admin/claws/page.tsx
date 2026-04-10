"use client";

import { useState, useEffect, useCallback } from "react";
import {
  adminClawApi,
  type AdminClawListItem,
  type AdminClawStats,
} from "@/lib/admin-api";

// ── Stat card ──────────────────────────────────────────────────

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-4">
      <p className="text-xs text-[#64748b]">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${color}`}>{value.toLocaleString()}</p>
    </div>
  );
}

// ── Status badges ──────────────────────────────────────────────

function ClaimBadge({ status }: { status: string }) {
  if (status === "claimed") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-400 border border-emerald-500/30">
        ✅ Claimed
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400 border border-amber-500/30">
      ⏳ Pending
    </span>
  );
}

function ApprovalBadge({ approved }: { approved: boolean }) {
  if (approved) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-400 border border-emerald-500/30">
        ⛏️ Approved
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-400 border border-red-500/30">
      🚫 Not Approved
    </span>
  );
}

// ── Helpers ────────────────────────────────────────────────────

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

function shortAddr(addr: string): string {
  if (!addr || addr.length <= 12) return addr || "—";
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

// ── Main Page ──────────────────────────────────────────────────

export default function AdminClawsPage() {
  const [items, setItems] = useState<AdminClawListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<AdminClawStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [approvedFilter, setApprovedFilter] = useState("");
  const [sort, setSort] = useState("created_at");
  const [order, setOrder] = useState("desc");
  const [searchInput, setSearchInput] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const fetchClaws = useCallback(async () => {
    try {
      setLoading(true);
      const res = await adminClawApi.list({
        page,
        page_size: pageSize,
        search: search || undefined,
        status: statusFilter || undefined,
        mining_approved: approvedFilter || undefined,
        sort,
        order,
      });
      setItems(res.items || []);
      setTotal(res.total);
    } catch (err) {
      console.error("Failed to fetch claws:", err);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, search, statusFilter, approvedFilter, sort, order]);

  const fetchStats = useCallback(async () => {
    try {
      const s = await adminClawApi.stats();
      setStats(s);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    fetchClaws();
  }, [fetchClaws]);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  const handleApprove = async (id: string) => {
    setActionLoading(id);
    try {
      await adminClawApi.approve(id);
      await Promise.all([fetchClaws(), fetchStats()]);
    } catch (err) {
      console.error("Failed to approve:", err);
    } finally {
      setActionLoading(null);
    }
  };

  const handleReject = async (id: string) => {
    setActionLoading(id);
    try {
      await adminClawApi.reject(id);
      await Promise.all([fetchClaws(), fetchStats()]);
    } catch (err) {
      console.error("Failed to reject:", err);
    } finally {
      setActionLoading(null);
    }
  };

  const handleBatchApprove = async () => {
    if (selectedIds.size === 0) return;
    setActionLoading("batch");
    try {
      await adminClawApi.batchApprove(Array.from(selectedIds));
      setSelectedIds(new Set());
      await Promise.all([fetchClaws(), fetchStats()]);
    } catch (err) {
      console.error("Failed to batch approve:", err);
    } finally {
      setActionLoading(null);
    }
  };

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === items.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(items.map((i) => i.id)));
    }
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-6">
      {/* ── Stats Cards ──────────────────────────────────── */}
      {stats && (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Total Claws" value={stats.total_claws} color="text-[#e2e8f0]" />
          <StatCard label="Claimed" value={stats.claimed_claws} color="text-emerald-400" />
          <StatCard label="Mining Approved" value={stats.approved_claws} color="text-purple-400" />
          <StatCard label="Pending Approval" value={stats.pending_approval} color="text-amber-400" />
        </div>
      )}

      {/* ── Batch Actions ────────────────────────────────── */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 rounded-lg border border-[#8b5cf6]/30 bg-[#8b5cf6]/5 px-4 py-3">
          <span className="text-sm text-[#e2e8f0]">
            {selectedIds.size} selected
          </span>
          <button
            onClick={handleBatchApprove}
            disabled={actionLoading === "batch"}
            className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-emerald-500 disabled:opacity-50"
          >
            {actionLoading === "batch" ? "Approving..." : "✅ Approve Selected"}
          </button>
          <button
            onClick={() => setSelectedIds(new Set())}
            className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] transition-colors hover:bg-[#1e1e2e]"
          >
            Clear
          </button>
        </div>
      )}

      {/* ── Filters ──────────────────────────────────────── */}
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by name or wallet..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="flex-1 min-w-[200px] rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] placeholder-[#475569] outline-none focus:border-[#8b5cf6]"
        />
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        >
          <option value="">All Status</option>
          <option value="pending_claim">Pending Claim</option>
          <option value="claimed">Claimed</option>
        </select>
        <select
          value={approvedFilter}
          onChange={(e) => { setApprovedFilter(e.target.value); setPage(1); }}
          className="rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        >
          <option value="">All Approval</option>
          <option value="true">Approved</option>
          <option value="false">Not Approved</option>
        </select>
        <select
          value={`${sort}:${order}`}
          onChange={(e) => {
            const [s, o] = e.target.value.split(":");
            setSort(s);
            setOrder(o);
            setPage(1);
          }}
          className="rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        >
          <option value="created_at:desc">Created ↓</option>
          <option value="created_at:asc">Created ↑</option>
          <option value="name:asc">Name A-Z</option>
          <option value="name:desc">Name Z-A</option>
          <option value="total_submitted:desc">Submitted ↓</option>
          <option value="total_accepted:desc">Accepted ↓</option>
          <option value="earnings:desc">Earnings ↓</option>
        </select>
      </div>

      {/* ── Table ────────────────────────────────────────── */}
      <div className="overflow-x-auto rounded-xl border border-[#1e1e2e] bg-[#0d0d14]">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[#1e1e2e] text-left text-xs uppercase text-[#64748b]">
              <th className="px-4 py-3 w-10">
                <input
                  type="checkbox"
                  checked={items.length > 0 && selectedIds.size === items.length}
                  onChange={toggleSelectAll}
                  className="accent-[#8b5cf6]"
                />
              </th>
              <th className="px-4 py-3">Name</th>
              <th className="px-4 py-3">Wallet</th>
              <th className="px-4 py-3">Claim</th>
              <th className="px-4 py-3">Mining</th>
              <th className="px-4 py-3 text-right">Submitted</th>
              <th className="px-4 py-3 text-right">Accepted</th>
              <th className="px-4 py-3 text-right">Earnings</th>
              <th className="px-4 py-3">Created</th>
              <th className="px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={10} className="px-4 py-8 text-center text-[#64748b]">
                  Loading...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={10} className="px-4 py-8 text-center text-[#64748b]">
                  No claws found
                </td>
              </tr>
            ) : (
              items.map((claw) => (
                <tr
                  key={claw.id}
                  className="border-b border-[#1e1e2e]/50 transition-colors hover:bg-[#1e1e2e]/30"
                >
                  <td className="px-4 py-3">
                    <input
                      type="checkbox"
                      checked={selectedIds.has(claw.id)}
                      onChange={() => toggleSelect(claw.id)}
                      className="accent-[#8b5cf6]"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <div>
                      <span className="font-medium text-[#e2e8f0]">🦞 {claw.name}</span>
                      {claw.description && (
                        <p className="mt-0.5 text-xs text-[#64748b] truncate max-w-[200px]">
                          {claw.description}
                        </p>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <code className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-xs text-[#8b5cf6]">
                      {shortAddr(claw.wallet_addr)}
                    </code>
                  </td>
                  <td className="px-4 py-3">
                    <ClaimBadge status={claw.status} />
                  </td>
                  <td className="px-4 py-3">
                    <ApprovalBadge approved={claw.mining_approved} />
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">
                    {claw.total_submitted}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">
                    {claw.total_accepted}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">
                    {claw.earnings.toFixed(2)}
                  </td>
                  <td className="px-4 py-3 text-[#94a3b8]">
                    {timeAgo(claw.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      {claw.mining_approved ? (
                        <button
                          onClick={() => handleReject(claw.id)}
                          disabled={actionLoading === claw.id}
                          className="rounded-lg border border-red-500/30 px-2.5 py-1 text-xs text-red-400 transition-colors hover:bg-red-500/10 disabled:opacity-50"
                        >
                          Revoke
                        </button>
                      ) : (
                        <button
                          onClick={() => handleApprove(claw.id)}
                          disabled={actionLoading === claw.id}
                          className="rounded-lg bg-emerald-600 px-2.5 py-1 text-xs font-medium text-white transition-colors hover:bg-emerald-500 disabled:opacity-50"
                        >
                          Approve
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* ── Pagination ───────────────────────────────────── */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-[#94a3b8]">
          <span>
            Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
            >
              ← Prev
            </button>
            <span>
              Page {page} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
            >
              Next →
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

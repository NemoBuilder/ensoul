"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  adminUserApi,
  type AdminUserListItem,
  type AdminUserOverviewStats,
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

// ── Status badge ───────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  if (status === "banned") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-400 border border-red-500/30">
        🔴 Banned
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-medium text-emerald-400 border border-emerald-500/30">
      🟢 Active
    </span>
  );
}

function SubBadge({ tier, status }: { tier: string | null; status: string | null }) {
  if (!tier || !status || status !== "active") {
    if (status === "expired") {
      return <span className="text-xs text-yellow-400">⏳ Expired</span>;
    }
    return <span className="text-xs text-[#64748b]">Free</span>;
  }
  if (tier === "pro") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-purple-500/10 px-2 py-0.5 text-xs font-medium text-purple-400 border border-purple-500/30">
        ⭐ Pro
      </span>
    );
  }
  return <span className="text-xs text-[#64748b]">{tier}</span>;
}

// ── Time ago helper ────────────────────────────────────────────

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
  if (addr.length <= 12) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

// ── Main Page ──────────────────────────────────────────────────

export default function AdminUsersPage() {
  const router = useRouter();
  const [items, setItems] = useState<AdminUserListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<AdminUserOverviewStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [subFilter, setSubFilter] = useState("");
  const [sort, setSort] = useState("last_seen_at");
  const [order, setOrder] = useState("desc");
  const [searchInput, setSearchInput] = useState("");

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const res = await adminUserApi.list({
        page,
        page_size: pageSize,
        search: search || undefined,
        status: statusFilter || undefined,
        subscription: subFilter || undefined,
        sort,
        order,
      });
      setItems(res.items || []);
      setTotal(res.total);
    } catch (err) {
      console.error("Failed to fetch users:", err);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, search, statusFilter, subFilter, sort, order]);

  const fetchStats = useCallback(async () => {
    try {
      const s = await adminUserApi.stats();
      setStats(s);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-6">
      {/* ── Stats Cards ──────────────────────────────────── */}
      {stats && (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Total Users" value={stats.total_users} color="text-[#e2e8f0]" />
          <StatCard label="Pro Subscribers" value={stats.pro_subscribers} color="text-purple-400" />
          <StatCard label="Today New" value={stats.today_new_users} color="text-emerald-400" />
          <StatCard label="Today Active" value={stats.today_active_users} color="text-blue-400" />
        </div>
      )}

      {/* ── Filters ──────────────────────────────────────── */}
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by wallet address (0x...)"
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
          <option value="active">Active</option>
          <option value="banned">Banned</option>
        </select>
        <select
          value={subFilter}
          onChange={(e) => { setSubFilter(e.target.value); setPage(1); }}
          className="rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        >
          <option value="">All Subscription</option>
          <option value="pro">Pro</option>
          <option value="free">Free</option>
          <option value="expired">Expired</option>
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
          <option value="last_seen_at:desc">Last Seen ↓</option>
          <option value="last_seen_at:asc">Last Seen ↑</option>
          <option value="first_seen_at:desc">Registered ↓</option>
          <option value="first_seen_at:asc">Registered ↑</option>
          <option value="login_count:desc">Logins ↓</option>
          <option value="login_count:asc">Logins ↑</option>
        </select>
      </div>

      {/* ── Table ────────────────────────────────────────── */}
      <div className="overflow-x-auto rounded-xl border border-[#1e1e2e] bg-[#0d0d14]">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[#1e1e2e] text-left text-xs uppercase text-[#64748b]">
              <th className="px-4 py-3">Address</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Subscription</th>
              <th className="px-4 py-3">Last Seen</th>
              <th className="px-4 py-3 text-right">Snipes</th>
              <th className="px-4 py-3 text-right">Logins</th>
              <th className="px-4 py-3">Note</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-[#64748b]">
                  Loading...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-[#64748b]">
                  No users found
                </td>
              </tr>
            ) : (
              items.map((user) => (
                <tr
                  key={user.wallet_addr}
                  onClick={() => router.push(`/admin/users/${user.wallet_addr}`)}
                  className="cursor-pointer border-b border-[#1e1e2e]/50 transition-colors hover:bg-[#1e1e2e]/30"
                >
                  <td className="px-4 py-3">
                    <code className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-xs text-[#8b5cf6]">
                      {shortAddr(user.wallet_addr)}
                    </code>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={user.status} />
                  </td>
                  <td className="px-4 py-3">
                    <SubBadge tier={user.sub_tier} status={user.sub_status} />
                  </td>
                  <td className="px-4 py-3 text-[#94a3b8]">
                    {timeAgo(user.last_seen_at)}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">
                    {user.snipe_count}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">
                    {user.login_count}
                  </td>
                  <td className="px-4 py-3 max-w-[150px] truncate text-[#64748b]">
                    {user.note || "—"}
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

"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  adminUserApi,
  type AdminUserListItem,
  type AdminUserOverviewStats,
  type AuthType,
} from "@/lib/admin-api";

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-4">
      <p className="text-xs text-[#64748b]">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${color}`}>{value.toLocaleString()}</p>
    </div>
  );
}

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

function ProBadge({ isPro, expiresAt }: { isPro: boolean; expiresAt?: string | null }) {
  if (isPro) {
    return (
      <span
        className="inline-flex items-center gap-1 rounded-full bg-purple-500/10 px-2 py-0.5 text-xs font-medium text-purple-400 border border-purple-500/30"
        title={expiresAt ? `Expires ${new Date(expiresAt).toLocaleDateString()}` : ""}
      >
        ⭐ Pro
      </span>
    );
  }
  return <span className="text-xs text-[#64748b]">Free</span>;
}

function AuthTypeBadge({ type }: { type: AuthType }) {
  const map: Record<AuthType, { icon: string; label: string; cls: string }> = {
    email:   { icon: "✉️", label: "Email",   cls: "bg-blue-500/10 text-blue-400 border-blue-500/30" },
    wallet:  { icon: "🔁", label: "Wallet",  cls: "bg-amber-500/10 text-amber-400 border-amber-500/30" },
    linked:  { icon: "🔗", label: "Linked",  cls: "bg-emerald-500/10 text-emerald-400 border-emerald-500/30" },
    unknown: { icon: "❓", label: "Unknown", cls: "bg-[#1e1e2e] text-[#64748b] border-[#1e1e2e]" },
  };
  const { icon, label, cls } = map[type] ?? map.unknown;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${cls}`}>
      <span>{icon}</span> {label}
    </span>
  );
}

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

function IdentityCell({ user }: { user: AdminUserListItem }) {
  const hasEmail = !!user.email;
  const hasWallet = !!user.wallet_addr;
  if (hasEmail) {
    return (
      <div className="flex flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-[#e2e8f0]">
          {user.email}
          {user.email_verified && <span title="Verified" className="text-emerald-400 text-[10px]">✓</span>}
        </span>
        {hasWallet && (
          <code className="text-[10px] text-[#64748b]">{shortAddr(user.wallet_addr!)}</code>
        )}
      </div>
    );
  }
  if (hasWallet) {
    return (
      <code className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-xs text-[#8b5cf6]">
        {shortAddr(user.wallet_addr!)}
      </code>
    );
  }
  return <span className="text-xs text-[#475569]">—</span>;
}

export default function AdminUsersPage() {
  const router = useRouter();
  const [items, setItems] = useState<AdminUserListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<AdminUserOverviewStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [search, setSearch] = useState("");
  const [authTypeFilter, setAuthTypeFilter] = useState<string>("email");
  const [statusFilter, setStatusFilter] = useState("");
  const [subFilter, setSubFilter] = useState("");
  const [sort, setSort] = useState("last_seen_at");
  const [order, setOrder] = useState("desc");
  const [searchInput, setSearchInput] = useState("");

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
        auth_type: authTypeFilter || undefined,
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
  }, [page, pageSize, search, statusFilter, subFilter, authTypeFilter, sort, order]);

  const fetchStats = useCallback(async () => {
    try {
      const s = await adminUserApi.stats();
      setStats(s);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => { fetchUsers(); }, [fetchUsers]);
  useEffect(() => { fetchStats(); }, [fetchStats]);

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-6">
      {stats && (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatCard label="Total Users" value={stats.total_users} color="text-[#e2e8f0]" />
            <StatCard label="Pro Subscribers" value={stats.pro_subscribers} color="text-purple-400" />
            <StatCard label="Today New" value={stats.today_new_users} color="text-emerald-400" />
            <StatCard label="Today Active" value={stats.today_active_users} color="text-blue-400" />
          </div>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatCard label="✉️ Email Only" value={stats.email_only_users} color="text-blue-400" />
            <StatCard label="🔗 Linked" value={stats.linked_users} color="text-emerald-400" />
            <StatCard label="🔁 Wallet Only" value={stats.wallet_only_users} color="text-amber-400" />
            <StatCard label="🚫 Banned" value={stats.banned_users} color="text-red-400" />
          </div>
        </>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by email or wallet (0x...)"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="flex-1 min-w-[240px] rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] placeholder-[#475569] outline-none focus:border-[#8b5cf6]"
        />
        <select
          value={authTypeFilter}
          onChange={(e) => { setAuthTypeFilter(e.target.value); setPage(1); }}
          className="rounded-lg border border-[#1e1e2e] bg-[#0d0d14] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        >
          <option value="">All Types</option>
          <option value="email">✉️ Email</option>
          <option value="linked">🔗 Linked</option>
          <option value="wallet">🔁 Wallet</option>
        </select>
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

      <div className="overflow-x-auto rounded-xl border border-[#1e1e2e] bg-[#0d0d14]">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[#1e1e2e] text-left text-xs uppercase text-[#64748b]">
              <th className="px-4 py-3">Identity</th>
              <th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Pro</th>
              <th className="px-4 py-3">Last Seen</th>
              <th className="px-4 py-3 text-right">Snipes</th>
              <th className="px-4 py-3 text-right">Logins</th>
              <th className="px-4 py-3">Note</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={8} className="px-4 py-8 text-center text-[#64748b]">Loading...</td></tr>
            ) : items.length === 0 ? (
              <tr><td colSpan={8} className="px-4 py-8 text-center text-[#64748b]">No users found</td></tr>
            ) : (
              items.map((user) => (
                <tr
                  key={user.id}
                  onClick={() => router.push(`/admin/users/${user.id}`)}
                  className="cursor-pointer border-b border-[#1e1e2e]/50 transition-colors hover:bg-[#1e1e2e]/30"
                >
                  <td className="px-4 py-3"><IdentityCell user={user} /></td>
                  <td className="px-4 py-3"><AuthTypeBadge type={user.auth_type} /></td>
                  <td className="px-4 py-3"><StatusBadge status={user.status} /></td>
                  <td className="px-4 py-3"><ProBadge isPro={user.is_pro} expiresAt={user.pro_expires_at} /></td>
                  <td className="px-4 py-3 text-[#94a3b8]">{timeAgo(user.last_seen_at)}</td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">{user.snipe_count}</td>
                  <td className="px-4 py-3 text-right font-mono text-[#94a3b8]">{user.login_count}</td>
                  <td className="px-4 py-3 max-w-[150px] truncate text-[#64748b]">{user.note || "—"}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-[#94a3b8]">
          <span>Showing {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total}</span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
            >← Prev</button>
            <span>Page {page} / {totalPages}</span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 transition-colors hover:bg-[#1e1e2e] disabled:opacity-40"
            >Next →</button>
          </div>
        </div>
      )}
    </div>
  );
}

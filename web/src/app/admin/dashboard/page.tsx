"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import {
  adminTaxWalletApi,
  adminMiningApi,
  adminCandidatesApi,
  type TaxWalletStatus,
  type MiningPoolStatus,
  type MintCandidate,
} from "@/lib/admin-api";

// ── Stat Card ──────────────────────────────────────────────────

function StatCard({
  icon,
  label,
  value,
  sub,
  color = "text-[#e2e8f0]",
}: {
  icon: string;
  label: string;
  value: string | number;
  sub?: string;
  color?: string;
}) {
  return (
    <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
      <div className="mb-1 flex items-center gap-2 text-sm text-[#94a3b8]">
        <span>{icon}</span>
        <span>{label}</span>
      </div>
      <div className={`text-2xl font-bold ${color}`}>{value}</div>
      {sub && <div className="mt-1 text-xs text-[#4a4a5a]">{sub}</div>}
    </div>
  );
}

// ── Dashboard Page ─────────────────────────────────────────────

export default function DashboardPage() {
  const [taxWallet, setTaxWallet] = useState<TaxWalletStatus | null>(null);
  const [miningPool, setMiningPool] = useState<MiningPoolStatus | null>(null);
  const [recentCandidates, setRecentCandidates] = useState<MintCandidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const [tax, pool, cands] = await Promise.all([
          adminTaxWalletApi.status(),
          adminMiningApi.pool(),
          adminCandidatesApi.list(),
        ]);
        setTaxWallet(tax);
        setMiningPool(pool);
        setRecentCandidates(cands.candidates.slice(0, 5));
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : "Failed to load dashboard");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  if (loading) {
    return <div className="text-[#94a3b8]">Loading dashboard...</div>;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
        {error}
      </div>
    );
  }

  // Convert tax wallet balance from wei to BNB
  const taxBalanceBNB = taxWallet
    ? (Number(BigInt(taxWallet.balance_wei)) / 1e18).toFixed(4)
    : "0";

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon="💰"
          label="Tax Wallet Balance"
          value={`${taxBalanceBNB} BNB`}
          color="text-[#22c55e]"
        />
        <StatCard
          icon="🎯"
          label="Pending Candidates"
          value={taxWallet?.candidates.pending ?? 0}
          sub={`${taxWallet?.candidates.minted ?? 0} minted · ${taxWallet?.candidates.failed ?? 0} failed`}
          color="text-[#f59e0b]"
        />
        <StatCard
          icon="⛏️"
          label="Mining Pool"
          value={`${(miningPool?.balance ?? 0).toLocaleString()} $Ensoul`}
          sub={`${(miningPool?.total_released ?? 0).toLocaleString()} total released`}
          color="text-[#8b5cf6]"
        />
        <StatCard
          icon="📈"
          label="Daily Released"
          value={`${(miningPool?.daily_released ?? 0).toLocaleString()}`}
          sub={`limit: ${(miningPool?.daily_limit ?? 0).toLocaleString()}`}
          color="text-[#3b82f6]"
        />
      </div>

      {/* Quick actions */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-4 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          Quick Actions
        </h2>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/admin/candidates"
            className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa]"
          >
            Manage Candidates
          </Link>
          <Link
            href="/admin/tax-wallet"
            className="rounded-lg border border-[#1e1e2e] bg-[#1a1a2e] px-4 py-2 text-sm font-medium text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
          >
            Tax Wallet Operations
          </Link>
          <Link
            href="/admin/mining"
            className="rounded-lg border border-[#1e1e2e] bg-[#1a1a2e] px-4 py-2 text-sm font-medium text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
          >
            Deposit to Mining Pool
          </Link>
        </div>
      </div>

      {/* Recent candidates */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
            Recent Candidates
          </h2>
          <Link
            href="/admin/candidates"
            className="text-xs text-[#8b5cf6] hover:text-[#a78bfa]"
          >
            View all →
          </Link>
        </div>

        {recentCandidates.length === 0 ? (
          <p className="text-sm text-[#4a4a5a]">No candidates yet</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[#1e1e2e] text-left text-xs text-[#4a4a5a] uppercase">
                  <th className="pb-2 pr-4">Handle</th>
                  <th className="pb-2 pr-4">Priority</th>
                  <th className="pb-2 pr-4">Status</th>
                  <th className="pb-2">Reason</th>
                </tr>
              </thead>
              <tbody>
                {recentCandidates.map((c) => (
                  <tr key={c.id} className="border-b border-[#1e1e2e]/50">
                    <td className="py-2.5 pr-4 font-mono text-[#e2e8f0]">@{c.handle}</td>
                    <td className="py-2.5 pr-4 text-[#94a3b8]">{c.priority}</td>
                    <td className="py-2.5 pr-4">
                      <StatusBadge status={c.status} />
                    </td>
                    <td className="py-2.5 text-[#94a3b8] truncate max-w-[200px]">
                      {c.reason || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Status badge ───────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-yellow-500/10 text-yellow-400 border-yellow-500/30",
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

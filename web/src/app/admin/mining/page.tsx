"use client";

import { useState, useEffect } from "react";
import { adminMiningApi, type MiningPoolStatus } from "@/lib/admin-api";

export default function MiningPage() {
  const [pool, setPool] = useState<MiningPoolStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");

  // Deposit form
  const [depositAmount, setDepositAmount] = useState("");
  const [depositing, setDepositing] = useState(false);

  const loadPool = async () => {
    try {
      setLoading(true);
      const p = await adminMiningApi.pool();
      setPool(p);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPool();
  }, []);

  useEffect(() => {
    if (actionMsg) {
      const t = setTimeout(() => setActionMsg(""), 5000);
      return () => clearTimeout(t);
    }
  }, [actionMsg]);

  const handleDeposit = async (e: React.FormEvent) => {
    e.preventDefault();
    const amount = Number(depositAmount);
    if (!amount || amount <= 0) {
      setError("Amount must be positive");
      return;
    }
    setDepositing(true);
    setError("");
    try {
      const res = await adminMiningApi.deposit(amount);
      setActionMsg(res.message || `Deposited ${amount} $Ensoul`);
      setDepositAmount("");
      loadPool();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to deposit");
    } finally {
      setDepositing(false);
    }
  };

  if (loading) {
    return <div className="text-sm text-[#94a3b8]">Loading mining pool...</div>;
  }

  return (
    <div className="space-y-6">
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

      {/* Stats */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">⛏️ Pool Balance</div>
          <div className="text-2xl font-bold text-[#8b5cf6]">
            {(pool?.balance ?? 0).toLocaleString()}
          </div>
          <div className="mt-1 text-xs text-[#4a4a5a]">$Ensoul</div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">📥 Total Deposited</div>
          <div className="text-2xl font-bold text-[#22c55e]">
            {(pool?.total_deposited ?? 0).toLocaleString()}
          </div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">📤 Total Released</div>
          <div className="text-2xl font-bold text-[#f59e0b]">
            {(pool?.total_released ?? 0).toLocaleString()}
          </div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">📈 Daily Released</div>
          <div className="text-2xl font-bold text-[#3b82f6]">
            {(pool?.daily_released ?? 0).toLocaleString()}
          </div>
          <div className="mt-1 text-xs text-[#4a4a5a]">
            limit: {(pool?.daily_limit ?? 0).toLocaleString()} · remaining: {(pool?.daily_remaining ?? 0).toLocaleString()}
          </div>
        </div>
      </div>

      {/* Pool Status */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
            Pool Details
          </h2>
          <button
            onClick={loadPool}
            className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
          >
            ↻ Refresh
          </button>
        </div>

        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-[#94a3b8]">Status</span>
            <span className={pool?.paused ? "text-red-400" : "text-[#22c55e]"}>
              {pool?.paused ? "⏸ Paused" : "▶ Active"}
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-[#94a3b8]">Last Reset</span>
            <span className="text-[#e2e8f0]">
              {pool?.last_reset_at
                ? new Date(pool.last_reset_at).toLocaleString()
                : "—"}
            </span>
          </div>
        </div>
      </div>

      {/* Deposit form */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          Deposit to Mining Pool
        </h2>
        <p className="mb-4 text-sm text-[#4a4a5a]">
          Add $Ensoul tokens to the mining pool. This records the deposit amount in the database.
        </p>
        <form onSubmit={handleDeposit} className="flex gap-3">
          <input
            type="number"
            placeholder="Amount ($Ensoul)"
            value={depositAmount}
            onChange={(e) => setDepositAmount(e.target.value)}
            min="0"
            step="any"
            required
            className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6] max-w-xs"
          />
          <button
            type="submit"
            disabled={depositing}
            className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {depositing ? "Depositing..." : "Deposit"}
          </button>
        </form>
      </div>
    </div>
  );
}

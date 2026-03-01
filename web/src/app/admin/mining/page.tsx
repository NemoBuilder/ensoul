"use client";

import { useState, useEffect, useCallback } from "react";
import {
  adminMiningApi,
  type MiningPoolStatus,
  type FailedMiningReward,
} from "@/lib/admin-api";

export default function MiningPage() {
  const [pool, setPool] = useState<MiningPoolStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");

  // Deposit form
  const [depositAmount, setDepositAmount] = useState("");
  const [depositing, setDepositing] = useState(false);

  // Failed rewards
  const [failedRewards, setFailedRewards] = useState<FailedMiningReward[]>([]);
  const [maxRetries, setMaxRetries] = useState(5);
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [retryingAll, setRetryingAll] = useState(false);

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

  const loadFailedRewards = useCallback(async () => {
    try {
      const res = await adminMiningApi.failedRewards();
      setFailedRewards(res.rewards || []);
      setMaxRetries(res.max_retries || 5);
    } catch {
      // silent — non-critical
    }
  }, []);

  useEffect(() => {
    loadPool();
    loadFailedRewards();
  }, [loadFailedRewards]);

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

  const handleRetryOne = async (rewardId: string) => {
    setRetryingId(rewardId);
    setError("");
    try {
      const res = await adminMiningApi.retryReward(rewardId);
      setActionMsg(res.message || "Reward queued for retry");
      // Refresh after a short delay (let the async send start)
      setTimeout(() => {
        loadFailedRewards();
        loadPool();
      }, 2000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Retry failed");
    } finally {
      setRetryingId(null);
    }
  };

  const handleRetryAll = async () => {
    setRetryingAll(true);
    setError("");
    try {
      const res = await adminMiningApi.retryAll();
      setActionMsg(res.message || `Retried ${res.retried} rewards`);
      setTimeout(() => {
        loadFailedRewards();
        loadPool();
      }, 3000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Retry all failed");
    } finally {
      setRetryingAll(false);
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

      {/* Failed Rewards */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
            ❌ Failed Rewards ({failedRewards.length})
          </h2>
          <div className="flex gap-2">
            <button
              onClick={loadFailedRewards}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
            >
              ↻ Refresh
            </button>
            {failedRewards.length > 0 && (
              <button
                onClick={handleRetryAll}
                disabled={retryingAll}
                className="rounded-lg bg-[#f59e0b] px-3 py-1.5 text-xs font-medium text-black transition-colors hover:bg-[#fbbf24] disabled:opacity-50"
              >
                {retryingAll ? "Retrying..." : `⟳ Retry All (${failedRewards.filter(r => r.retry_count < maxRetries).length})`}
              </button>
            )}
          </div>
        </div>

        {failedRewards.length === 0 ? (
          <div className="rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-8 text-center text-[#4a4a5a]">
            <p className="text-lg">✅ No failed rewards</p>
            <p className="mt-1 text-xs">All mining rewards have been delivered successfully.</p>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="rounded-lg border border-yellow-500/20 bg-yellow-500/5 p-3 text-xs text-yellow-400">
              💡 Failed rewards have been refunded to the mining pool. Retrying will re-deduct from the pool and re-send on-chain.
              Auto-retry runs every 10 minutes (max {maxRetries} attempts).
            </div>

            {failedRewards.map((r) => {
              const retriesExhausted = r.retry_count >= maxRetries;
              return (
                <div
                  key={r.id}
                  className={`rounded-lg border p-4 ${
                    retriesExhausted
                      ? "border-red-500/30 bg-red-500/5"
                      : "border-[#1e1e2e] bg-[#0a0a0f]"
                  }`}
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0 flex-1">
                      {/* Amount + Claw */}
                      <div className="flex items-center gap-3 mb-2">
                        <span className="font-mono text-sm font-bold text-[#8b5cf6]">
                          {r.amount.toFixed(4)} $Ensoul
                        </span>
                        <span className="text-xs text-[#94a3b8]">→</span>
                        <span className="text-sm text-[#e2e8f0]">
                          {r.claw?.name || r.claw_id.slice(0, 8)}
                        </span>
                        {r.claw?.wallet_addr && (
                          <a
                            href={`https://bscscan.com/address/${r.claw.wallet_addr}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="font-mono text-xs text-[#4a4a5a] hover:text-[#8b5cf6]"
                          >
                            {r.claw.wallet_addr.slice(0, 6)}…{r.claw.wallet_addr.slice(-4)}
                          </a>
                        )}
                      </div>

                      {/* Error reason */}
                      {r.last_error && (
                        <div className="mb-2 rounded bg-red-500/10 px-2 py-1">
                          <p className="font-mono text-xs text-red-400 break-all">
                            {r.last_error}
                          </p>
                        </div>
                      )}

                      {/* Meta */}
                      <div className="flex flex-wrap gap-3 text-xs text-[#4a4a5a]">
                        <span>
                          Retries: <strong className={retriesExhausted ? "text-red-400" : "text-[#94a3b8]"}>
                            {r.retry_count}/{maxRetries}
                          </strong>
                        </span>
                        <span>Created: {new Date(r.created_at).toLocaleString()}</span>
                        {r.last_attempt_at && (
                          <span>Last attempt: {new Date(r.last_attempt_at).toLocaleString()}</span>
                        )}
                        {r.tx_hash && (
                          <a
                            href={`https://bscscan.com/tx/${r.tx_hash}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-[#8b5cf6] hover:underline"
                          >
                            Tx: {r.tx_hash.slice(0, 10)}…
                          </a>
                        )}
                        <span className="font-mono text-[#4a4a5a]">
                          ID: {r.id.slice(0, 8)}
                        </span>
                      </div>
                    </div>

                    {/* Retry button */}
                    <button
                      onClick={() => handleRetryOne(r.id)}
                      disabled={retryingId === r.id || retriesExhausted}
                      className={`shrink-0 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                        retriesExhausted
                          ? "border border-red-500/30 text-red-400 cursor-not-allowed opacity-50"
                          : "bg-[#8b5cf6] text-white hover:bg-[#a78bfa] disabled:opacity-50"
                      }`}
                      title={retriesExhausted ? `Max retries (${maxRetries}) exhausted` : "Retry this reward"}
                    >
                      {retryingId === r.id
                        ? "Retrying..."
                        : retriesExhausted
                        ? "Exhausted"
                        : "⟳ Retry"}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

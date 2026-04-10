"use client";

import { useState, useEffect, useCallback } from "react";
import { Link } from "@/i18n/navigation";
import { useAccount, useSignMessage } from "wagmi";
import {
  sessionApi,
  clawKeyApi,
  withdrawApi,
  ClawBindingInfo,
  Fragment,
  MiningReward,
  WithdrawStatus,
  WithdrawRecord,
} from "@/lib/api";
import { dimensionLabels, timeAgo } from "@/lib/utils";

export default function DashboardPage() {
  const { address, isConnected } = useAccount();
  const { signMessageAsync } = useSignMessage();

  const [sessionAddr, setSessionAddr] = useState("");
  const [loggingIn, setLoggingIn] = useState(false);
  const [checkingSession, setCheckingSession] = useState(true);

  const [claws, setClaws] = useState<ClawBindingInfo[]>([]);
  const [activeIdx, setActiveIdx] = useState(0);
  const [newKey, setNewKey] = useState("");
  const [adding, setAdding] = useState(false);
  const [overview, setOverview] = useState<{
    total_submitted: number;
    total_accepted: number;
    accept_rate: string;
    earnings: number;
  } | null>(null);
  const [clawWalletAddr, setClawWalletAddr] = useState("");
  const [miningRewards, setMiningRewards] = useState<MiningReward[]>([]);
  const [totalEarned, setTotalEarned] = useState(0);
  const [totalPending, setTotalPending] = useState(0);
  const [contributions, setContributions] = useState<Fragment[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [activeTab, setActiveTab] = useState<"contributions" | "rewards" | "withdraw">("contributions");

  // Withdraw state
  const [withdrawStatus, setWithdrawStatus] = useState<WithdrawStatus | null>(null);
  const [withdrawHistory, setWithdrawHistory] = useState<WithdrawRecord[]>([]);
  const [withdrawAmount, setWithdrawAmount] = useState("");
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawMsg, setWithdrawMsg] = useState("");
  const [checkingGas, setCheckingGas] = useState(false);

  // Check session on mount / when wallet changes
  useEffect(() => {
    if (!isConnected || !address) {
      setSessionAddr("");
      setClaws([]);
      setOverview(null);
      setContributions([]);
      setActiveIdx(0);
      setCheckingSession(false);
      return;
    }
    setCheckingSession(true);
    sessionApi
      .session()
      .then((res) => {
        if (res.address && res.address.toLowerCase() === address.toLowerCase()) {
          setSessionAddr(res.address);
        } else {
          setSessionAddr("");
        }
      })
      .catch(() => setSessionAddr(""))
      .finally(() => setCheckingSession(false));
  }, [address, isConnected]);

  // Load bound claws when session is established
  const loadBoundClaws = useCallback(async () => {
    try {
      const res = await clawKeyApi.list();
      const list: ClawBindingInfo[] = res.claws || [];
      setClaws(list);
      setActiveIdx(0);
      if (list.length > 0) {
        fetchDashboard(list[0].id);
      } else {
        setOverview(null);
        setContributions([]);
      }
    } catch {
      setClaws([]);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (sessionAddr) {
      loadBoundClaws();
    }
  }, [sessionAddr, loadBoundClaws]);

  const fetchDashboard = useCallback(async (bindingId: string) => {
    setLoading(true);
    setError("");
    try {
      const data = await clawKeyApi.dashboard(bindingId);
      setOverview(data.overview);
      setContributions(data.recent_contributions || []);
      setClawWalletAddr(data.wallet_addr || "");
      setMiningRewards(data.mining_rewards || []);
      setTotalEarned(data.total_earned || 0);
      setTotalPending(data.total_pending || 0);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load dashboard");
      setOverview(null);
      setContributions([]);
      setMiningRewards([]);
      setTotalEarned(0);
      setTotalPending(0);
    } finally {
      setLoading(false);
    }
  }, []);

  function switchClaw(idx: number) {
    setActiveIdx(idx);
    setError("");
    // Reset withdraw state when switching claws
    setWithdrawStatus(null);
    setWithdrawHistory([]);
    setWithdrawAmount("");
    setWithdrawMsg("");
    fetchDashboard(claws[idx].id);
    // If currently on withdraw tab, fetch new claw's withdraw data
    if (activeTab === "withdraw") {
      const cid = claws[idx]?.claw_id;
      if (cid) {
        setCheckingGas(true);
        Promise.all([
          withdrawApi.check(cid),
          withdrawApi.history(cid),
        ])
          .then(([st, hist]) => {
            setWithdrawStatus(st);
            setWithdrawHistory(hist.withdrawals || []);
          })
          .catch(() => {})
          .finally(() => setCheckingGas(false));
      }
    }
  }

  async function handleLogin() {
    if (!address) return;
    setLoggingIn(true);
    setError("");
    try {
      const message = `ensoul:login:${Date.now()}`;
      const signature = await signMessageAsync({ message });
      await sessionApi.login(address, signature, message);
      setSessionAddr(address);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoggingIn(false);
    }
  }

  async function handleAddClaw() {
    if (!newKey.trim()) return;
    setAdding(true);
    setError("");
    try {
      await clawKeyApi.bind(newKey.trim());
      setNewKey("");
      await loadBoundClaws();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Invalid API key");
    } finally {
      setAdding(false);
    }
  }

  async function removeClaw(idx: number) {
    const binding = claws[idx];
    if (!binding) return;
    try {
      await clawKeyApi.unbind(binding.id);
      const updated = claws.filter((_, i) => i !== idx);
      setClaws(updated);
      if (updated.length === 0) {
        setActiveIdx(0);
        setOverview(null);
        setContributions([]);
      } else {
        const newIdx = Math.min(activeIdx, updated.length - 1);
        setActiveIdx(newIdx);
        fetchDashboard(updated[newIdx].id);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to remove");
    }
  }

  // Wallet not connected — show gate
  if (!isConnected) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="mb-8">
          <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
            Claw Dashboard
          </h1>
          <p className="text-[#94a3b8]">
            Manage your Claws, track contributions, and view earnings.
          </p>
        </div>
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <div className="mb-4 text-5xl">🔒</div>
          <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">
            Wallet Required
          </h2>
          <p className="mb-2 text-[#94a3b8]">
            Connect your wallet to manage your Claws.
          </p>
          <p className="text-sm text-[#94a3b8]/70">
            Claw API keys are bound to your wallet address for security.
          </p>
        </div>
      </div>
    );
  }

  // Checking session
  if (checkingSession) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="mb-8">
          <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
            Claw Dashboard
          </h1>
        </div>
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
          Checking session...
        </div>
      </div>
    );
  }

  // Session not established — sign to continue
  if (!sessionAddr) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="mb-8">
          <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
            Claw Dashboard
          </h1>
          <p className="text-[#94a3b8]">
            Manage your Claws, track contributions, and view earnings.
          </p>
        </div>
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <div className="mb-4 text-5xl">✍️</div>
          <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">
            Sign to Continue
          </h2>
          <p className="mb-6 text-[#94a3b8]">
            Sign a message with your wallet to verify ownership and access your
            dashboard securely. No gas fees.
          </p>
          {error && (
            <p className="mb-4 text-sm text-red-400">{error}</p>
          )}
          <button
            onClick={handleLogin}
            disabled={loggingIn}
            className="rounded-lg bg-[#8b5cf6] px-8 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {loggingIn ? "Signing..." : "Sign & Login"}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <div className="mb-8">
        <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
          Claw Dashboard
        </h1>
        <p className="text-[#94a3b8]">
          Manage your Claws, track contributions, and view earnings.
        </p>
      </div>

      {/* Claw tabs + Add button */}
      <div className="mb-6">
        <div className="flex flex-wrap items-center gap-2">
          {claws.map((claw, idx) => (
            <div key={claw.id} className="group flex items-center">
              <button
                onClick={() => switchClaw(idx)}
                className={`rounded-l-lg px-4 py-2 text-sm font-medium transition-colors ${
                  idx === activeIdx
                    ? "bg-[#8b5cf6] text-white"
                    : "border border-[#1e1e2e] bg-[#14141f] text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                }`}
              >
                🦞 {claw.claw_name}
              </button>
              <button
                onClick={() => removeClaw(idx)}
                className={`rounded-r-lg px-2 py-2 text-sm transition-colors ${
                  idx === activeIdx
                    ? "bg-[#7c3aed] text-white/70 hover:text-white"
                    : "border border-l-0 border-[#1e1e2e] bg-[#14141f] text-[#94a3b8]/50 hover:text-red-400"
                }`}
                title="Remove this Claw"
              >
                ✕
              </button>
            </div>
          ))}
          <button
            onClick={() => setAdding(!adding)}
            className="rounded-lg border border-dashed border-[#1e1e2e] px-4 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#8b5cf6]"
          >
            + Add Claw
          </button>
        </div>

        {/* Add claw input */}
        {adding && (
          <div className="mt-3 flex gap-3">
            <input
              type="password"
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleAddClaw()}
              placeholder="Paste your Claw API key..."
              className="flex-1 rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-2.5 font-mono text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none focus:border-[#8b5cf6]"
              autoFocus
            />
            <button
              onClick={handleAddClaw}
              disabled={loading || !newKey.trim()}
              className="rounded-md bg-[#8b5cf6] px-5 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              Add
            </button>
            <button
              onClick={() => { setAdding(false); setNewKey(""); setError(""); }}
              className="rounded-md border border-[#1e1e2e] px-4 py-2.5 text-sm text-[#94a3b8] hover:text-[#e2e8f0]"
            >
              Cancel
            </button>
          </div>
        )}
      </div>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* No claws state */}
      {claws.length === 0 && !adding && (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
          <p className="mb-2 text-[#e2e8f0]">No Claws connected</p>
          <p className="mb-4 text-sm text-[#94a3b8]">
            Add your Claw API key to view your dashboard.{" "}
            <Link href="/claw" className="text-[#8b5cf6] hover:underline">
              Register a Claw
            </Link>
          </p>
          <button
            onClick={() => setAdding(true)}
            className="rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
          >
            + Add Claw
          </button>
        </div>
      )}

      {/* Dashboard content */}
      {claws.length > 0 && loading && (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
          Loading...
        </div>
      )}

      {claws.length > 0 && !loading && overview && (
        <>
          {/* Mining Approval Status */}
          {claws[activeIdx] && !claws[activeIdx].mining_approved && (
            <div className="mb-4 flex items-center gap-3 rounded-lg border border-amber-500/20 bg-amber-500/5 px-4 py-3">
              <span className="text-lg">⏳</span>
              <div>
                <p className="text-sm font-medium text-amber-400">Mining approval pending</p>
                <p className="text-xs text-amber-400/70">
                  Your Claw is awaiting admin approval. Once approved, you can start submitting fragments and earning $Ensoul.
                </p>
              </div>
            </div>
          )}
          {claws[activeIdx] && claws[activeIdx].mining_approved && (
            <div className="mb-4 flex items-center gap-3 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-4 py-3">
              <span className="text-lg">⛏️</span>
              <div>
                <p className="text-sm font-medium text-emerald-400">Mining approved</p>
                <p className="text-xs text-emerald-400/70">
                  Your Claw is fully activated — submit fragments and earn $Ensoul rewards!
                </p>
              </div>
            </div>
          )}

          {/* Claw Wallet Address */}
          {clawWalletAddr && (
            <div className="mb-4 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-[#94a3b8]">Claw Wallet</span>
                  <span className="font-mono text-sm text-[#e2e8f0]">{clawWalletAddr}</span>
                </div>
                <div className="flex items-center gap-3">
                  <a
                    href={`https://bscscan.com/address/${clawWalletAddr}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-xs text-[#8b5cf6] hover:underline"
                  >
                    View on BscScan →
                  </a>
                </div>
              </div>
              <p className="mt-1 text-xs text-[#94a3b8]/70">
                Mining rewards are sent here. Use the Withdraw tab to transfer to your wallet.
              </p>
            </div>
          )}

          {/* Overview cards */}
          <div className="mb-8 grid gap-4 sm:grid-cols-2 md:grid-cols-5">
            {[
              { label: "Submitted", value: overview.total_submitted, color: "text-[#e2e8f0]" },
              { label: "Accepted", value: overview.total_accepted, color: "text-[#e2e8f0]" },
              { label: "Accept Rate", value: overview.accept_rate, color: "text-[#e2e8f0]" },
              { label: "Total Earned", value: `${totalEarned.toFixed(2)} $Ensoul`, color: "text-[#22c55e]" },
              { label: "Pending", value: `${totalPending.toFixed(2)} $Ensoul`, color: "text-[#f59e0b]" },
            ].map((item) => (
              <div
                key={item.label}
                className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center"
              >
                <div className={`font-mono text-xl font-bold ${item.color}`}>
                  {item.value}
                </div>
                <div className="mt-1 text-xs text-[#94a3b8]">{item.label}</div>
              </div>
            ))}
          </div>

          {/* Quick actions */}
          <div className="mb-8 flex gap-3">
            <Link
              href="/explore"
              className="rounded-lg border border-[#1e1e2e] bg-[#14141f] px-4 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
            >
              Browse Souls →
            </Link>
            <Link
              href="/mining"
              className="rounded-lg border border-[#1e1e2e] bg-[#14141f] px-4 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
            >
              Mining Pool →
            </Link>
          </div>

          {/* Tab selector */}
          <div className="mb-4 flex gap-1 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-1">
            <button
              onClick={() => setActiveTab("contributions")}
              className={`flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "contributions"
                  ? "bg-[#8b5cf6] text-white"
                  : "text-[#94a3b8] hover:text-[#e2e8f0]"
              }`}
            >
              Recent Contributions
            </button>
            <button
              onClick={() => setActiveTab("rewards")}
              className={`flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "rewards"
                  ? "bg-[#8b5cf6] text-white"
                  : "text-[#94a3b8] hover:text-[#e2e8f0]"
              }`}
            >
              Mining Rewards ({miningRewards.length})
            </button>
            <button
              onClick={() => {
                setActiveTab("withdraw");
                const claw = claws[activeIdx];
                if (claw) {
                  setCheckingGas(true);
                  setWithdrawStatus(null);
                  setWithdrawHistory([]);
                  const cid = claw.claw_id;
                  Promise.all([
                    withdrawApi.check(cid),
                    withdrawApi.history(cid),
                  ])
                    .then(([st, hist]) => {
                      setWithdrawStatus(st);
                      setWithdrawHistory(hist.withdrawals || []);
                    })
                    .catch(() => {})
                    .finally(() => setCheckingGas(false));
                }
              }}
              className={`flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "withdraw"
                  ? "bg-[#8b5cf6] text-white"
                  : "text-[#94a3b8] hover:text-[#e2e8f0]"
              }`}
            >
              Withdraw
            </button>
          </div>

          {/* Tab content: Recent contributions */}
          {activeTab === "contributions" && (
          <div>
            {contributions.length === 0 ? (
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
                <p className="mb-2">No contributions yet</p>
                <p className="text-sm">
                  Start contributing fragments to souls via the API.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {contributions.map((c) => {
                  const statusColor = {
                    accepted: "text-green-400 bg-green-500/10",
                    pending: "text-yellow-400 bg-yellow-500/10",
                    rejected: "text-red-400 bg-red-500/10",
                  }[c.status];
                  return (
                    <div
                      key={c.id}
                      className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4"
                    >
                      <div className="mb-2 flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusColor}`}>
                            {c.status}
                          </span>
                          <span className="text-xs text-[#94a3b8]">
                            {dimensionLabels[c.dimension] || c.dimension}
                          </span>
                          {c.shell && (
                            <Link
                              href={`/soul/${c.shell.handle}`}
                              className="text-xs text-[#8b5cf6] hover:underline"
                            >
                              @{c.shell.handle}
                            </Link>
                          )}
                        </div>
                        <span className="text-xs text-[#94a3b8]">
                          {timeAgo(c.created_at)}
                        </span>
                      </div>
                      <p className="text-sm text-[#e2e8f0]">
                        {c.content || (
                          <span className="text-[#64748b] italic">🔒 Content protected</span>
                        )}
                      </p>
                      {c.reject_reason && (
                        <p className="mt-2 text-xs text-red-400">
                          Reason: {c.reject_reason}
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
          )}

          {/* Tab content: Mining rewards */}
          {activeTab === "rewards" && (
          <div>
            {miningRewards.length === 0 ? (
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
                <p className="mb-2">No mining rewards yet</p>
                <p className="text-sm">
                  Submit accepted fragments to earn $Ensoul tokens from the mining pool.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {miningRewards.map((r) => {
                  const statusMap: Record<string, { color: string; label: string }> = {
                    confirmed: { color: "text-green-400 bg-green-500/10", label: "✓ Confirmed" },
                    sent: { color: "text-blue-400 bg-blue-500/10", label: "⟳ Sent" },
                    pending: { color: "text-yellow-400 bg-yellow-500/10", label: "⏳ Pending" },
                    failed: { color: "text-red-400 bg-red-500/10", label: "✕ Failed" },
                  };
                  const st = statusMap[r.status] || { color: "text-[#94a3b8] bg-[#1e1e2e]", label: r.status };
                  return (
                    <div
                      key={r.id}
                      className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                          <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${st.color}`}>
                            {st.label}
                          </span>
                          <span className="font-mono text-sm font-bold text-[#8b5cf6]">
                            +{r.amount.toFixed(4)} $Ensoul
                          </span>
                        </div>
                        <span className="text-xs text-[#94a3b8]">
                          {timeAgo(r.created_at)}
                        </span>
                      </div>
                      {r.tx_hash && (
                        <div className="mt-2">
                          <a
                            href={`https://bscscan.com/tx/${r.tx_hash}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="font-mono text-xs text-[#8b5cf6] hover:underline"
                          >
                            Tx: {r.tx_hash.slice(0, 10)}…{r.tx_hash.slice(-8)}
                          </a>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
          )}

          {/* Tab content: Withdraw */}
          {activeTab === "withdraw" && (
          <div className="space-y-4">
            {checkingGas ? (
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
                Checking wallet status...
              </div>
            ) : withdrawStatus ? (
              <>
                {/* Withdraw status card */}
                <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
                  <h3 className="mb-4 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
                    Wallet Status
                  </h3>
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="rounded-md bg-[#0a0a0f] p-3">
                      <p className="text-xs text-[#94a3b8]">Withdrawable</p>
                      <p className="mt-1 font-mono text-lg font-bold text-[#22c55e]">
                        {withdrawStatus.withdrawable.toFixed(2)}
                      </p>
                      <p className="text-xs text-[#4a4a5a]">$Ensoul</p>
                    </div>
                    <div className="rounded-md bg-[#0a0a0f] p-3">
                      <p className="text-xs text-[#94a3b8]">On-chain Balance</p>
                      <p className="mt-1 font-mono text-lg font-bold text-[#8b5cf6]">
                        {withdrawStatus.token_balance.toFixed(2)}
                      </p>
                      <p className="text-xs text-[#4a4a5a]">$Ensoul</p>
                    </div>
                    <div className="rounded-md bg-[#0a0a0f] p-3">
                      <p className="text-xs text-[#94a3b8]">Gas (BNB)</p>
                      <p className={`mt-1 font-mono text-lg font-bold ${
                        withdrawStatus.has_gas ? "text-[#22c55e]" : "text-red-400"
                      }`}>
                        {withdrawStatus.bnb_balance.toFixed(6)}
                      </p>
                      <p className="text-xs text-[#4a4a5a]">
                        {withdrawStatus.has_gas ? "✅ Sufficient" : `⚠️ Need ≥ ${withdrawStatus.min_gas} BNB`}
                      </p>
                    </div>
                  </div>

                  {/* Gas warning + deposit instructions */}
                  {!withdrawStatus.has_gas && (
                    <div className="mt-4 rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-4">
                      <p className="mb-2 text-sm font-medium text-yellow-400">⛽ Gas Required</p>
                      <p className="mb-3 text-xs text-[#94a3b8]">
                        Your Claw wallet needs a small amount of BNB to pay for the transaction fee.
                        Send at least <strong className="text-[#e2e8f0]">{withdrawStatus.min_gas} BNB</strong> (~$0.15) to:
                      </p>
                      <div className="flex items-center gap-2 rounded-md bg-[#0a0a0f] p-3">
                        <span className="font-mono text-sm text-[#e2e8f0] break-all">
                          {withdrawStatus.claw_wallet}
                        </span>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(withdrawStatus.claw_wallet);
                            setWithdrawMsg("Address copied!");
                            setTimeout(() => setWithdrawMsg(""), 2000);
                          }}
                          className="shrink-0 rounded bg-[#1e1e2e] px-2 py-1 text-xs text-[#94a3b8] hover:text-[#e2e8f0]"
                        >
                          Copy
                        </button>
                      </div>
                      <p className="mt-2 text-xs text-[#4a4a5a]">
                        After sending BNB, click Refresh to update the status.
                      </p>
                      <button
                        onClick={() => {
                          setCheckingGas(true);
                          const cid = claws[activeIdx]?.claw_id;
                          if (cid) {
                            withdrawApi.check(cid)
                              .then(setWithdrawStatus)
                              .catch(() => {})
                              .finally(() => setCheckingGas(false));
                          }
                        }}
                        className="mt-3 rounded-lg border border-[#1e1e2e] px-4 py-2 text-xs text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                      >
                        ↻ Refresh Status
                      </button>
                    </div>
                  )}

                  {/* Withdraw form */}
                  {withdrawStatus.has_gas && (
                    <div className="mt-4">
                      {withdrawStatus.can_withdraw ? (
                        <div className="flex gap-3">
                          <input
                            type="number"
                            placeholder={`Amount (min ${withdrawStatus.min_amount})`}
                            value={withdrawAmount}
                            onChange={(e) => setWithdrawAmount(e.target.value)}
                            min={withdrawStatus.min_amount}
                            max={withdrawStatus.withdrawable}
                            step="any"
                            className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6] max-w-xs"
                          />
                          <button
                            onClick={() => setWithdrawAmount(Math.floor(withdrawStatus.withdrawable * 10000 / 10000).toString())}
                            className="rounded-lg border border-[#1e1e2e] px-3 py-2 text-xs text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                          >
                            Max
                          </button>
                          <button
                            onClick={async () => {
                              const amt = Number(withdrawAmount);
                              if (!amt || amt < withdrawStatus.min_amount) {
                                setError(`Minimum is ${withdrawStatus.min_amount} $Ensoul`);
                                return;
                              }
                              setWithdrawing(true);
                              setError("");
                              try {
                                const cid = claws[activeIdx]?.claw_id;
                                const res = await withdrawApi.withdraw(cid, amt);
                                setWithdrawMsg(res.message || "Withdrawal initiated!");
                                setWithdrawAmount("");
                                // Refresh status and history
                                setTimeout(async () => {
                                  const [st, hist] = await Promise.all([
                                    withdrawApi.check(cid),
                                    withdrawApi.history(cid),
                                  ]);
                                  setWithdrawStatus(st);
                                  setWithdrawHistory(hist.withdrawals || []);
                                  setWithdrawMsg("");
                                }, 3000);
                              } catch (err: unknown) {
                                setError(err instanceof Error ? err.message : "Withdrawal failed");
                              } finally {
                                setWithdrawing(false);
                              }
                            }}
                            disabled={withdrawing}
                            className="rounded-lg bg-[#22c55e] px-5 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#16a34a] disabled:opacity-50"
                          >
                            {withdrawing ? "Sending..." : "Withdraw"}
                          </button>
                        </div>
                      ) : (
                        <p className="text-sm text-[#f59e0b]">
                          ⚠️ {withdrawStatus.reason}
                        </p>
                      )}
                    </div>
                  )}
                </div>

                {/* Success / info message */}
                {withdrawMsg && (
                  <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-3 text-sm text-green-400">
                    {withdrawMsg}
                  </div>
                )}

                {/* Withdrawal history */}
                <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
                  <h3 className="mb-3 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
                    Withdrawal History
                  </h3>
                  {withdrawHistory.length === 0 ? (
                    <p className="text-sm text-[#4a4a5a]">No withdrawals yet.</p>
                  ) : (
                    <div className="space-y-2">
                      {withdrawHistory.map((w) => {
                        const stMap: Record<string, { color: string; label: string }> = {
                          confirmed: { color: "text-green-400 bg-green-500/10", label: "✓ Confirmed" },
                          sent: { color: "text-blue-400 bg-blue-500/10", label: "⟳ Sent" },
                          pending: { color: "text-yellow-400 bg-yellow-500/10", label: "⏳ Pending" },
                          failed: { color: "text-red-400 bg-red-500/10", label: "✕ Failed" },
                        };
                        const st = stMap[w.status] || { color: "text-[#94a3b8] bg-[#1e1e2e]", label: w.status };
                        return (
                          <div
                            key={w.id}
                            className="flex items-center justify-between rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3"
                          >
                            <div className="flex items-center gap-3">
                              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${st.color}`}>
                                {st.label}
                              </span>
                              <span className="font-mono text-sm font-bold text-[#e2e8f0]">
                                {w.amount.toFixed(4)} $Ensoul
                              </span>
                              {w.tx_hash && (
                                <a
                                  href={`https://bscscan.com/tx/${w.tx_hash}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="font-mono text-xs text-[#8b5cf6] hover:underline"
                                >
                                  {w.tx_hash.slice(0, 10)}…
                                </a>
                              )}
                            </div>
                            <span className="text-xs text-[#94a3b8]">{timeAgo(w.created_at)}</span>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </>
            ) : (
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center text-[#94a3b8]">
                Unable to load withdraw status.
              </div>
            )}
          </div>
          )}
        </>
      )}
    </div>
  );
}

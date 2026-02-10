"use client";

import { useState, useEffect, useCallback } from "react";
import { Link } from "@/i18n/navigation";
import { useAccount, useSignMessage } from "wagmi";
import {
  sessionApi,
  clawKeyApi,
  ClawBindingInfo,
  Fragment,
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
  const [contributions, setContributions] = useState<Fragment[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

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
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load dashboard");
      setOverview(null);
      setContributions([]);
    } finally {
      setLoading(false);
    }
  }, []);

  function switchClaw(idx: number) {
    setActiveIdx(idx);
    setError("");
    fetchDashboard(claws[idx].id);
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
          {/* Overview cards */}
          <div className="mb-8 grid gap-4 sm:grid-cols-4">
            {[
              { label: "Submitted", value: overview.total_submitted },
              { label: "Accepted", value: overview.total_accepted },
              { label: "Accept Rate", value: overview.accept_rate },
              { label: "Earnings", value: `${overview.earnings} BNB` },
            ].map((item) => (
              <div
                key={item.label}
                className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center"
              >
                <div className="font-mono text-2xl font-bold text-[#e2e8f0]">
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
          </div>

          {/* Recent contributions */}
          <div>
            <h3 className="mb-4 text-lg font-medium text-[#e2e8f0]">
              Recent Contributions
            </h3>
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
        </>
      )}
    </div>
  );
}

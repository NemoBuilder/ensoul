"use client";

import { useState, useEffect } from "react";
import { adminTaxWalletApi, type TaxWalletStatus } from "@/lib/admin-api";

export default function TaxWalletPage() {
  const [status, setStatus] = useState<TaxWalletStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actionMsg, setActionMsg] = useState("");
  const [minting, setMinting] = useState(false);

  // Single mint
  const [mintHandle, setMintHandle] = useState("");
  const [mintingSingle, setMintingSingle] = useState(false);

  const loadStatus = async () => {
    try {
      setLoading(true);
      const s = await adminTaxWalletApi.status();
      setStatus(s);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadStatus();
  }, []);

  useEffect(() => {
    if (actionMsg) {
      const t = setTimeout(() => setActionMsg(""), 5000);
      return () => clearTimeout(t);
    }
  }, [actionMsg]);

  const handleTriggerMint = async () => {
    if (!confirm("Trigger auto-mint for all pending candidates?")) return;
    setMinting(true);
    try {
      const res = await adminTaxWalletApi.triggerMint();
      setActionMsg(res.message);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to trigger mint");
    } finally {
      setMinting(false);
    }
  };

  const handleMintSingle = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!mintHandle.trim()) return;
    setMintingSingle(true);
    setError("");
    try {
      const res = await adminTaxWalletApi.mintSingle(mintHandle.trim());
      setActionMsg(res.message);
      setMintHandle("");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to mint");
    } finally {
      setMintingSingle(false);
    }
  };

  const balanceBNB = status
    ? (Number(BigInt(status.balance_wei)) / 1e18).toFixed(6)
    : "0";

  if (loading) {
    return <div className="text-sm text-[#94a3b8]">Loading tax wallet...</div>;
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

      {/* Balance & Stats */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">💰 BNB Balance</div>
          <div className="text-2xl font-bold text-[#22c55e]">{balanceBNB}</div>
          <div className="mt-1 text-xs text-[#4a4a5a] font-mono break-all">
            {status?.balance_wei} wei
          </div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">⏳ Pending</div>
          <div className="text-2xl font-bold text-[#f59e0b]">
            {status?.candidates.pending ?? 0}
          </div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">✅ Minted</div>
          <div className="text-2xl font-bold text-[#22c55e]">
            {status?.candidates.minted ?? 0}
          </div>
        </div>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-1 text-sm text-[#94a3b8]">❌ Failed</div>
          <div className="text-2xl font-bold text-[#ef4444]">
            {status?.candidates.failed ?? 0}
          </div>
        </div>
      </div>

      {/* Trigger auto-mint */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          Batch Mint (All Pending)
        </h2>
        <p className="mb-4 text-sm text-[#4a4a5a]">
          Triggers the auto-mint process for all pending candidates. The process runs in the background — check server logs for progress.
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={handleTriggerMint}
            disabled={minting || (status?.candidates.pending ?? 0) === 0}
            className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {minting ? "Triggering..." : `🚀 Mint ${status?.candidates.pending ?? 0} Pending`}
          </button>
          <button
            onClick={loadStatus}
            className="rounded-lg border border-[#1e1e2e] px-3 py-2 text-sm text-[#94a3b8] hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
          >
            ↻ Refresh
          </button>
        </div>
      </div>

      {/* Mint single handle */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          Mint Single Handle
        </h2>
        <p className="mb-4 text-sm text-[#4a4a5a]">
          Immediately mint a specific Twitter handle as a public Soul using the tax wallet. The handle doesn&apos;t need to be in the candidate list.
        </p>
        <form onSubmit={handleMintSingle} className="flex gap-3">
          <input
            type="text"
            placeholder="@handle"
            value={mintHandle}
            onChange={(e) => setMintHandle(e.target.value)}
            required
            className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6] max-w-xs"
          />
          <button
            type="submit"
            disabled={mintingSingle}
            className="rounded-lg bg-[#22c55e] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#16a34a] disabled:opacity-50"
          >
            {mintingSingle ? "Minting..." : "Mint Now"}
          </button>
        </form>
      </div>
    </div>
  );
}

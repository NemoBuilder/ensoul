"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { holderApi, type HolderDashboard } from "@/lib/api";

export default function HolderPage() {
  const t = useTranslations("Holder");
  const { isConnected } = useAccount();
  const [dashboard, setDashboard] = useState<HolderDashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [claiming, setClaiming] = useState(false);
  const [claimResult, setClaimResult] = useState("");
  const [error, setError] = useState("");

  const fetchDashboard = useCallback(async () => {
    try {
      const data = await holderApi.dashboard();
      setDashboard(data);
    } catch {
      // No data or not logged in
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isConnected) fetchDashboard();
    else setLoading(false);
  }, [isConnected, fetchDashboard]);

  async function handleClaim() {
    setClaiming(true);
    setError("");
    setClaimResult("");
    try {
      const result = await holderApi.claim();
      setClaimResult(t("claimSuccess", { txHash: result.tx_hash?.slice(0, 10) + "..." }));
      fetchDashboard();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Claim failed");
    } finally {
      setClaiming(false);
    }
  }

  if (!isConnected) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16 text-center">
        <h1 className="mb-4 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
        <p className="mb-6 text-[#94a3b8]">{t("connectPrompt")}</p>
        <ConnectButton />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      </div>
    );
  }

  if (!dashboard || (dashboard.shells?.length === 0 && dashboard.total_earned === 0)) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16 text-center">
        <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
        <p className="mb-6 text-[#94a3b8]">{t("noSouls")}</p>
        <Link href="/mint" className="text-[#8b5cf6] hover:underline">{t("mintFirst")}</Link>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {error && (
        <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">{error}</div>
      )}
      {claimResult && (
        <div className="mb-4 rounded-lg border border-green-500/30 bg-green-500/5 p-3 text-sm text-green-400">{claimResult}</div>
      )}

      {/* Summary Cards */}
      <div className="mb-8 grid gap-4 sm:grid-cols-3">
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
          <p className="text-xs text-[#94a3b8]">{t("totalEarned")}</p>
          <p className="mt-1 font-mono text-2xl font-bold text-[#8b5cf6]">
            {dashboard.total_earned.toFixed(4)} BNB
          </p>
        </div>
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
          <p className="text-xs text-[#94a3b8]">{t("pendingClaim")}</p>
          <p className="mt-1 font-mono text-2xl font-bold text-[#e2e8f0]">
            {dashboard.total_pending.toFixed(4)} BNB
          </p>
        </div>
        <div className="rounded-lg border border-[#8b5cf6]/20 bg-[#8b5cf6]/5 p-4">
          <button
            onClick={handleClaim}
            disabled={claiming || dashboard.total_pending <= 0}
            className="w-full rounded-lg bg-[#8b5cf6] py-3 text-sm font-bold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {claiming ? t("claiming") : t("claimRevenue")}
          </button>
        </div>
      </div>

      {/* Your Souls */}
      <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("yourSouls")}</h2>
      <div className="mb-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {dashboard.shells?.map((s) => (
          <Link
            key={s.handle}
            href={`/soul/${s.handle}` as "/soul/[handle]"}
            className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 transition-colors hover:border-[#8b5cf6]/30"
          >
            <div className="flex items-center gap-3">
              {s.avatar_url && (
                <img src={s.avatar_url} alt={s.handle} className="h-10 w-10 rounded-full" />
              )}
              <div>
                <p className="text-sm font-medium text-[#e2e8f0]">@{s.handle}</p>
                <p className="text-xs text-[#94a3b8]">{s.stage}</p>
              </div>
            </div>
            <p className="mt-2 text-xs text-[#94a3b8]">
              {t("usageCount", { count: s.current_usage })}
            </p>
          </Link>
        ))}
      </div>

      {/* Revenue History */}
      <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("revenueHistory")}</h2>
      {dashboard.recent_revenue?.length === 0 ? (
        <p className="text-sm text-[#94a3b8]">{t("noRevenue")}</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-[#1e1e2e]">
          <table className="w-full text-sm">
            <thead className="bg-[#14141f]">
              <tr className="text-left text-[#94a3b8]">
                <th className="px-4 py-3">{t("period")}</th>
                <th className="px-4 py-3">Soul</th>
                <th className="px-4 py-3">{t("amount")}</th>
                <th className="px-4 py-3">{t("status")}</th>
              </tr>
            </thead>
            <tbody>
              {dashboard.recent_revenue?.map((r) => (
                <tr key={r.id} className="border-t border-[#1e1e2e]">
                  <td className="px-4 py-3 text-[#e2e8f0]">{r.period}</td>
                  <td className="px-4 py-3 text-[#e2e8f0]">@{r.shell?.handle || r.shell_id}</td>
                  <td className="px-4 py-3 font-mono text-[#8b5cf6]">{r.amount.toFixed(4)} BNB</td>
                  <td className="px-4 py-3">
                    <span className={`rounded-full px-2 py-0.5 text-xs ${
                      r.status === "paid"
                        ? "bg-green-500/10 text-green-400"
                        : "bg-yellow-500/10 text-yellow-400"
                    }`}>
                      {r.status === "paid" ? t("paid") : t("pending")}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

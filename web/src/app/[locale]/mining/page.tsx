"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import {
  miningApi,
  sessionApi,
  clawKeyApi,
  type MiningPoolStatus,
  type FragmentDemand,
  type MiningReward,
  type ClawBindingInfo,
} from "@/lib/api";
import { dimensionLabels, timeAgo } from "@/lib/utils";

export default function MiningPage() {
  const t = useTranslations("Mining");
  const [pool, setPool] = useState<MiningPoolStatus | null>(null);
  const [demands, setDemands] = useState<FragmentDemand[]>([]);
  const [loading, setLoading] = useState(true);

  // Personal rewards state
  const [claws, setClaws] = useState<ClawBindingInfo[]>([]);
  const [rewards, setRewards] = useState<MiningReward[]>([]);
  const [totalEarned, setTotalEarned] = useState(0);
  const [totalPending, setTotalPending] = useState(0);
  const [hasSession, setHasSession] = useState(false);

  useEffect(() => {
    // Load pool + demands (public)
    Promise.all([miningApi.pool(), miningApi.demands()])
      .then(([p, d]) => {
        setPool(p);
        setDemands(d);
      })
      .catch(console.error)
      .finally(() => setLoading(false));

    // Try to load personal rewards (if logged in)
    sessionApi
      .session()
      .then(async () => {
        setHasSession(true);
        const res = await clawKeyApi.list();
        const list = res.claws || [];
        setClaws(list);
        if (list.length > 0) {
          // Fetch rewards for first claw
          const data = await miningApi.rewards(list[0].claw_id);
          setRewards(data.rewards || []);
          setTotalEarned(data.total_earned || 0);
          setTotalPending(data.total_pending || 0);
        }
      })
      .catch(() => {
        setHasSession(false);
      });
  }, []);

  if (loading) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="flex flex-col items-center gap-3 py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {/* Pool Status */}
      {pool && (
        <div className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-[#e2e8f0]">{t("poolStatus")}</h2>
            <span className={`rounded-full px-3 py-1 text-xs font-medium ${
              pool.paused ? "bg-red-500/10 text-red-400" : "bg-green-500/10 text-green-400"
            }`}>
              {pool.paused ? t("paused") : t("active")}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            {[
              { label: t("balance"), value: pool.balance.toLocaleString() },
              { label: t("dailyLimit"), value: pool.daily_limit.toLocaleString() },
              { label: t("dailyReleased"), value: pool.daily_released.toLocaleString() },
              { label: t("dailyRemaining"), value: pool.daily_remaining.toLocaleString() },
              { label: t("totalDeposited"), value: pool.total_deposited.toLocaleString() },
              { label: t("totalReleased"), value: pool.total_released.toLocaleString() },
            ].map((item) => (
              <div key={item.label} className="rounded-md bg-[#0a0a0f] p-3">
                <p className="text-xs text-[#94a3b8]">{item.label}</p>
                <p className="mt-1 font-mono text-lg font-bold text-[#8b5cf6]">{item.value}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* My Mining Rewards */}
      <div className="mb-8">
        <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("myRewards")}</h2>
        {!hasSession ? (
          <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
            <p className="mb-2 text-[#e2e8f0]">{t("loginToSee")}</p>
            <p className="mb-4 text-sm text-[#94a3b8]">{t("loginToSeeDesc")}</p>
            <Link
              href="/claw/dashboard"
              className="inline-block rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              {t("goToDashboard")}
            </Link>
          </div>
        ) : claws.length === 0 ? (
          <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
            <p className="mb-2 text-[#e2e8f0]">{t("noClaws")}</p>
            <p className="mb-4 text-sm text-[#94a3b8]">{t("noClawsDesc")}</p>
            <Link
              href="/claw"
              className="inline-block rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              {t("registerClaw")}
            </Link>
          </div>
        ) : (
          <>
            {/* Earnings summary */}
            <div className="mb-4 grid gap-4 sm:grid-cols-3">
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
                <div className="font-mono text-2xl font-bold text-[#22c55e]">
                  {totalEarned.toFixed(2)}
                </div>
                <div className="mt-1 text-xs text-[#94a3b8]">{t("earned")} ($Ensoul)</div>
              </div>
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
                <div className="font-mono text-2xl font-bold text-[#f59e0b]">
                  {totalPending.toFixed(2)}
                </div>
                <div className="mt-1 text-xs text-[#94a3b8]">{t("pending")} ($Ensoul)</div>
              </div>
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
                <div className="font-mono text-2xl font-bold text-[#e2e8f0]">
                  {rewards.length}
                </div>
                <div className="mt-1 text-xs text-[#94a3b8]">{t("totalRewards")}</div>
              </div>
            </div>

            {/* Claw wallet info */}
            {claws[0]?.wallet_addr && (
              <div className="mb-4 rounded-md border border-[#1e1e2e] bg-[#0a0a0f] p-3">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-[#94a3b8]">{t("clawWallet")}:</span>
                  <a
                    href={`https://bscscan.com/address/${claws[0].wallet_addr}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-mono text-xs text-[#8b5cf6] hover:underline"
                  >
                    {claws[0].wallet_addr}
                  </a>
                </div>
              </div>
            )}

            {/* Rewards list */}
            {rewards.length === 0 ? (
              <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6 text-center text-[#94a3b8]">
                <p>{t("noRewardsYet")}</p>
              </div>
            ) : (
              <div className="space-y-2">
                {rewards.slice(0, 10).map((r) => {
                  const statusMap: Record<string, { color: string; label: string }> = {
                    confirmed: { color: "text-green-400 bg-green-500/10", label: "✓" },
                    sent: { color: "text-blue-400 bg-blue-500/10", label: "⟳" },
                    pending: { color: "text-yellow-400 bg-yellow-500/10", label: "⏳" },
                    failed: { color: "text-red-400 bg-red-500/10", label: "✕" },
                  };
                  const st = statusMap[r.status] || { color: "text-[#94a3b8] bg-[#1e1e2e]", label: "?" };
                  return (
                    <div
                      key={r.id}
                      className="flex items-center justify-between rounded-lg border border-[#1e1e2e] bg-[#14141f] px-4 py-3"
                    >
                      <div className="flex items-center gap-3">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${st.color}`}>
                          {st.label}
                        </span>
                        <span className="font-mono text-sm font-bold text-[#8b5cf6]">
                          +{r.amount.toFixed(4)} $Ensoul
                        </span>
                        {r.tx_hash && (
                          <a
                            href={`https://bscscan.com/tx/${r.tx_hash}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="font-mono text-xs text-[#94a3b8] hover:text-[#8b5cf6]"
                          >
                            {r.tx_hash.slice(0, 8)}…
                          </a>
                        )}
                      </div>
                      <span className="text-xs text-[#94a3b8]">{timeAgo(r.created_at)}</span>
                    </div>
                  );
                })}
                {rewards.length > 10 && (
                  <div className="pt-2 text-center">
                    <Link
                      href="/claw/dashboard"
                      className="text-sm text-[#8b5cf6] hover:underline"
                    >
                      {t("viewAll")} →
                    </Link>
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>

      {/* Fragment Demands */}
      <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("demands")}</h2>
      {demands.length === 0 ? (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <p className="text-lg text-[#e2e8f0]">{t("noDemandsTitle")}</p>
          <p className="mt-2 text-sm text-[#94a3b8]">{t("noDemandsDesc")}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {demands.map((d) => (
            <div key={d.id} className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-sm font-medium text-[#e2e8f0]">
                    @{d.shell?.handle || d.shell_id}
                  </span>
                  <span className="rounded bg-[#8b5cf6]/10 px-2 py-0.5 text-xs text-[#8b5cf6]">
                    {dimensionLabels[d.dimension] || d.dimension}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="font-mono text-sm font-bold text-[#8b5cf6]">
                    {d.bounty} $Ensoul
                  </span>
                  <span className={`rounded-full px-2 py-0.5 text-xs ${
                    d.status === "open"
                      ? "bg-green-500/10 text-green-400"
                      : "bg-[#1e1e2e] text-[#94a3b8]"
                  }`}>
                    {d.status === "open" ? t("open") : t("filled")}
                  </span>
                </div>
              </div>
              {d.description && (
                <p className="mt-2 text-sm text-[#94a3b8]">{d.description}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

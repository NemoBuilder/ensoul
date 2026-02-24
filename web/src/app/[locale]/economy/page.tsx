"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import {
  economyApi,
  type EconomyOverview,
  type EconomyBuybackRecord,
  type EconomyRevenuePool,
} from "@/lib/api";

function fmt(n: number, decimals = 2): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(decimals) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(decimals) + "K";
  return n.toFixed(decimals);
}

function fmtBNB(n: number): string {
  return n.toFixed(4);
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

// ─── KPI Card ────────────────────────────────────────────────
function KPICard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
      <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">{label}</p>
      <p className="mt-2 font-mono text-2xl font-bold text-[#8b5cf6]">{value}</p>
      {sub && <p className="mt-1 text-xs text-[#64748b]">{sub}</p>}
    </div>
  );
}

// ─── Flow Diagram ────────────────────────────────────────────
function FlywheelDiagram({ t, split }: { t: ReturnType<typeof useTranslations>; split: EconomyOverview["split_config"] }) {
  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
      <h2 className="mb-6 text-lg font-semibold text-[#e2e8f0]">{t("flywheelTitle")}</h2>

      <div className="flex flex-col items-center gap-4">
        {/* Revenue Sources */}
        <div className="flex w-full justify-center gap-6">
          <div className="rounded-md border border-[#8b5cf6]/30 bg-[#8b5cf6]/10 px-4 py-2 text-center text-sm font-medium text-[#c4b5fd]">
            🪙 {t("mintRevenue")}
          </div>
          <div className="rounded-md border border-[#06b6d4]/30 bg-[#06b6d4]/10 px-4 py-2 text-center text-sm font-medium text-[#67e8f9]">
            📡 {t("subRevenue")}
          </div>
        </div>

        {/* Arrow down */}
        <div className="text-[#64748b]">▼</div>

        {/* Buyback Wallet */}
        <div className="rounded-lg border border-[#f59e0b]/30 bg-[#f59e0b]/10 px-6 py-3 text-center">
          <p className="text-sm font-semibold text-[#fbbf24]">💰 {t("buybackWallet")}</p>
          <p className="mt-1 text-xs text-[#d97706]">BNB → $Ensoul Swap</p>
        </div>

        {/* Arrow down */}
        <div className="text-[#64748b]">▼</div>

        {/* 3-way split */}
        <div className="grid w-full max-w-2xl grid-cols-3 gap-3">
          {/* Mining Pool */}
          <div className="rounded-lg border border-[#22c55e]/30 bg-[#22c55e]/10 p-3 text-center">
            <p className="text-xs text-[#86efac]">⛏️ {t("miningPool")}</p>
            <p className="mt-1 text-lg font-bold text-[#4ade80]">{split.mint_buyback_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("mintLabel")}</p>
            <p className="text-xs font-medium text-[#4ade80]">{split.sub_buyback_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("subLabel")}</p>
          </div>

          {/* Treasury */}
          <div className="rounded-lg border border-[#3b82f6]/30 bg-[#3b82f6]/10 p-3 text-center">
            <p className="text-xs text-[#93c5fd]">🏦 {t("treasury")}</p>
            <p className="mt-1 text-lg font-bold text-[#60a5fa]">{split.mint_treasury_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("mintLabel")}</p>
            <p className="text-xs font-medium text-[#60a5fa]">{split.sub_treasury_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("subLabel")}</p>
          </div>

          {/* Revenue Pool */}
          <div className="rounded-lg border border-[#a855f7]/30 bg-[#a855f7]/10 p-3 text-center">
            <p className="text-xs text-[#d8b4fe]">💎 {t("revenuePool")}</p>
            <p className="mt-1 text-lg font-bold text-[#c084fc]">{split.mint_revenue_pool_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("mintLabel")}</p>
            <p className="text-xs font-medium text-[#c084fc]">{split.sub_revenue_pool_pct}%</p>
            <p className="text-[10px] text-[#64748b]">{t("subLabel")}</p>
          </div>
        </div>

        {/* Arrow down */}
        <div className="flex w-full max-w-2xl justify-around text-[#64748b]">
          <span>▼</span>
          <span>▼</span>
          <span>▼</span>
        </div>

        {/* Outputs */}
        <div className="grid w-full max-w-2xl grid-cols-3 gap-3 text-center">
          <p className="text-xs text-[#94a3b8]">→ {t("clawRewards")}</p>
          <p className="text-xs text-[#94a3b8]">→ {t("coldStorage")}</p>
          <p className="text-xs text-[#94a3b8]">→ {t("holderClaims")}</p>
        </div>
      </div>
    </div>
  );
}

// ─── Main Page ───────────────────────────────────────────────
export default function EconomyPage() {
  const t = useTranslations("Economy");
  const [data, setData] = useState<EconomyOverview | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    economyApi
      .overview()
      .then(setData)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl px-4 pt-24 pb-16">
        <div className="flex flex-col items-center gap-3 py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="mx-auto max-w-6xl px-4 pt-24 pb-16 text-center">
        <p className="text-[#94a3b8]">{t("loadError")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {/* ── KPI Cards ── */}
      <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <KPICard
          label={t("totalSouls")}
          value={data.total_souls.toLocaleString()}
          sub={t("mintedNFTs")}
        />
        <KPICard
          label={t("totalBuyback")}
          value={fmt(data.buyback.total_token_bought) + " $Ensoul"}
          sub={fmtBNB(data.buyback.total_bnb_spent) + " BNB " + t("spent")}
        />
        <KPICard
          label={t("miningPoolBalance")}
          value={fmt(data.mining_pool.balance) + " $Ensoul"}
          sub={t("dailyLimit") + ": " + fmt(data.mining_pool.daily_limit)}
        />
        <KPICard
          label={t("activeSubscribers")}
          value={data.total_subscribers.toLocaleString()}
          sub={t("sniperUsers")}
        />
      </div>

      {/* ── Flywheel Diagram ── */}
      <div className="mb-8">
        <FlywheelDiagram t={t} split={data.split_config} />
      </div>

      {/* ── Two-column: Buyback History + Revenue Pools ── */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Buyback History */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("buybackHistory")}</h2>
          {(!data.buyback_history || data.buyback_history.length === 0) ? (
            <p className="py-8 text-center text-sm text-[#64748b]">{t("noRecords")}</p>
          ) : (
            <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
              {data.buyback_history.map((r: EconomyBuybackRecord) => (
                <div key={r.id} className="flex items-center justify-between rounded-md bg-[#0a0a0f] px-3 py-2">
                  <div>
                    <span className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${
                      r.source === "mint_revenue"
                        ? "bg-[#8b5cf6]/20 text-[#c4b5fd]"
                        : "bg-[#06b6d4]/20 text-[#67e8f9]"
                    }`}>
                      {r.source === "mint_revenue" ? t("mint") : t("subscription")}
                    </span>
                    <span className="ml-2 text-xs text-[#64748b]">{timeAgo(r.created_at)}</span>
                  </div>
                  <div className="text-right">
                    <p className="font-mono text-sm font-medium text-[#e2e8f0]">
                      {fmt(r.token_amount)} <span className="text-[#8b5cf6]">$Ensoul</span>
                    </p>
                    <p className="text-[10px] text-[#64748b]">{fmtBNB(r.bnb_amount)} BNB</p>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Buyback breakdown */}
          <div className="mt-4 grid grid-cols-2 gap-3 border-t border-[#1e1e2e] pt-4">
            <div className="text-center">
              <p className="text-xs text-[#94a3b8]">{t("fromMint")}</p>
              <p className="font-mono text-sm font-bold text-[#c4b5fd]">{fmt(data.buyback.mint_revenue_token)}</p>
              <p className="text-[10px] text-[#64748b]">{fmtBNB(data.buyback.mint_revenue_bnb)} BNB</p>
            </div>
            <div className="text-center">
              <p className="text-xs text-[#94a3b8]">{t("fromSubscription")}</p>
              <p className="font-mono text-sm font-bold text-[#67e8f9]">{fmt(data.buyback.sub_revenue_token)}</p>
              <p className="text-[10px] text-[#64748b]">{fmtBNB(data.buyback.sub_revenue_bnb)} BNB</p>
            </div>
          </div>
        </div>

        {/* Revenue Pools */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("revenuePoolHistory")}</h2>
          {(!data.revenue_pools || data.revenue_pools.length === 0) ? (
            <p className="py-8 text-center text-sm text-[#64748b]">{t("noRecords")}</p>
          ) : (
            <div className="space-y-2">
              {data.revenue_pools.map((rp: EconomyRevenuePool) => (
                <div key={rp.id} className="flex items-center justify-between rounded-md bg-[#0a0a0f] px-3 py-2">
                  <div>
                    <p className="font-mono text-sm font-medium text-[#e2e8f0]">{rp.period}</p>
                    <span className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${
                      rp.distributed
                        ? "bg-green-500/20 text-green-400"
                        : "bg-yellow-500/20 text-yellow-400"
                    }`}>
                      {rp.distributed ? t("distributed") : t("accumulating")}
                    </span>
                  </div>
                  <div className="text-right">
                    <p className="font-mono text-sm font-bold text-[#c084fc]">
                      {fmt(rp.pool_amount)} <span className="text-[#a855f7]">$Ensoul</span>
                    </p>
                    <p className="text-[10px] text-[#64748b]">
                      {t("totalRevenue")}: {fmt(rp.total_revenue)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* ── Mining Pool Details ── */}
      <div className="mt-6 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-[#e2e8f0]">{t("miningPoolDetails")}</h2>
          <span className={`rounded-full px-3 py-1 text-xs font-medium ${
            data.mining_pool.paused
              ? "bg-red-500/10 text-red-400"
              : "bg-green-500/10 text-green-400"
          }`}>
            {data.mining_pool.paused ? t("paused") : t("active")}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          {[
            { label: t("poolBalance"), value: fmt(data.mining_pool.balance) },
            { label: t("dailyLimit"), value: fmt(data.mining_pool.daily_limit) },
            { label: t("dailyReleased"), value: fmt(data.mining_pool.daily_released) },
            { label: t("dailyRemaining"), value: fmt(data.mining_pool.daily_remaining) },
            { label: t("totalDeposited"), value: fmt(data.mining_pool.total_deposited) },
            { label: t("totalReleased"), value: fmt(data.mining_pool.total_released) },
          ].map((item) => (
            <div key={item.label} className="rounded-md bg-[#0a0a0f] p-3">
              <p className="text-xs text-[#94a3b8]">{item.label}</p>
              <p className="mt-1 font-mono text-lg font-bold text-[#4ade80]">{item.value}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
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

function shortAddr(addr: string): string {
  if (!addr || addr.length < 10) return addr || "";
  return addr.slice(0, 6) + "..." + addr.slice(-4);
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

const BSCSCAN = "https://bscscan.com/address/";

// ─── KPI Card ────────────────────────────────────────────────
function KPICard({ label, value, sub, color = "text-[#8b5cf6]" }: {
  label: string; value: string; sub?: string; color?: string;
}) {
  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
      <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">{label}</p>
      <p className={`mt-2 font-mono text-2xl font-bold ${color}`}>{value}</p>
      {sub && <p className="mt-1 text-xs text-[#64748b]">{sub}</p>}
    </div>
  );
}

// ─── SVG Flow Diagram ────────────────────────────────────────
function FlywheelDiagram({ t, data }: { t: ReturnType<typeof useTranslations>; data: EconomyOverview }) {
  const split = data.split_config || { mint_buyback_pct: 60, mint_treasury_pct: 10, mint_revenue_pool_pct: 30, sub_buyback_pct: 40, sub_treasury_pct: 10, sub_revenue_pool_pct: 50 };
  const wallets = data.wallets || { buyback_bnb: 0, buyback_token: 0, buyback_addr: "", mining_pool_token: 0, mining_pool_addr: "", revenue_pool_token: 0, revenue_pool_addr: "", treasury_addr: "" };
  const last_buyback = data.last_buyback;

  // Compute last buyback split amounts for arrow labels
  const lastToken = last_buyback?.token_amount ?? 0;
  const isMintSource = last_buyback?.source === "mint_revenue";
  const buybackPct = isMintSource ? split.mint_buyback_pct : split.sub_buyback_pct;
  const treasuryPct = isMintSource ? split.mint_treasury_pct : split.sub_treasury_pct;
  const revPoolPct = isMintSource ? split.mint_revenue_pool_pct : split.sub_revenue_pool_pct;
  const lastMiningAmt = lastToken * buybackPct / (buybackPct + revPoolPct);
  const lastRevenueAmt = lastToken - lastMiningAmt;
  const lastTreasuryBNB = last_buyback ? last_buyback.bnb_amount * treasuryPct / 100 : 0;

  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
      <h2 className="mb-2 text-lg font-semibold text-[#e2e8f0]">{t("flywheelTitle")}</h2>
      {last_buyback && (
        <p className="mb-4 text-xs text-[#64748b]">
          {t("lastOperation")}: {timeAgo(last_buyback.created_at)} · {fmtBNB(last_buyback.bnb_amount)} BNB → {fmt(last_buyback.token_amount)} $Ensoul
        </p>
      )}

      <div className="relative mx-auto" style={{ maxWidth: 720, minHeight: 520 }}>
        <svg className="absolute inset-0 h-full w-full" viewBox="0 0 720 520" fill="none" xmlns="http://www.w3.org/2000/svg">
          {/* Mint → Buyback */}
          <path d="M 210 55 L 360 110" stroke="#8b5cf6" strokeWidth="2" strokeDasharray="6 3" markerEnd="url(#arrowPurple)" />
          {/* Sub → Buyback */}
          <path d="M 510 55 L 360 110" stroke="#06b6d4" strokeWidth="2" strokeDasharray="6 3" markerEnd="url(#arrowCyan)" />

          {/* Buyback → Mining Pool */}
          <path d="M 290 175 L 140 260" stroke="#22c55e" strokeWidth="2" markerEnd="url(#arrowGreen)" />
          <text x="175" y="215" fill="#86efac" fontSize="10" textAnchor="middle">
            {buybackPct}% · {fmt(lastMiningAmt)}
          </text>

          {/* Buyback → Treasury (BNB) */}
          <path d="M 360 175 L 360 260" stroke="#3b82f6" strokeWidth="2" markerEnd="url(#arrowBlue)" />
          <text x="400" y="225" fill="#93c5fd" fontSize="10" textAnchor="start">
            {treasuryPct}% · {fmtBNB(lastTreasuryBNB)} BNB
          </text>

          {/* Buyback → Revenue Pool */}
          <path d="M 430 175 L 580 260" stroke="#a855f7" strokeWidth="2" markerEnd="url(#arrowViolet)" />
          <text x="545" y="215" fill="#d8b4fe" fontSize="10" textAnchor="middle">
            {revPoolPct}% · {fmt(lastRevenueAmt)}
          </text>

          {/* Mining → Claw Rewards */}
          <path d="M 140 370 L 140 430" stroke="#22c55e" strokeWidth="1.5" strokeDasharray="4 2" markerEnd="url(#arrowGreen)" />
          {/* Treasury → Cold Storage */}
          <path d="M 360 370 L 360 430" stroke="#3b82f6" strokeWidth="1.5" strokeDasharray="4 2" markerEnd="url(#arrowBlue)" />
          {/* Revenue → Holder Claims */}
          <path d="M 580 370 L 580 430" stroke="#a855f7" strokeWidth="1.5" strokeDasharray="4 2" markerEnd="url(#arrowViolet)" />

          {/* Arrow markers */}
          <defs>
            <marker id="arrowPurple" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#8b5cf6" />
            </marker>
            <marker id="arrowCyan" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#06b6d4" />
            </marker>
            <marker id="arrowGreen" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#22c55e" />
            </marker>
            <marker id="arrowBlue" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#3b82f6" />
            </marker>
            <marker id="arrowViolet" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" fill="#a855f7" />
            </marker>
          </defs>
        </svg>

        {/* ── HTML Nodes (positioned absolutely over SVG) ── */}
        <div className="relative h-full" style={{ minHeight: 520 }}>

          {/* Row 1: Revenue Sources */}
          <div className="absolute left-[80px] top-[10px] w-[180px]">
            <div className="cursor-pointer rounded-lg border border-[#8b5cf6]/40 bg-[#8b5cf6]/10 px-4 py-3 text-center transition hover:border-[#8b5cf6]/70">
              <p className="text-sm font-semibold text-[#c4b5fd]">🪙 {t("mintRevenue")}</p>
              <p className="mt-1 font-mono text-xs text-[#8b5cf6]">{fmtBNB(data.buyback?.mint_revenue_bnb ?? 0)} BNB</p>
            </div>
          </div>

          <div className="absolute left-[460px] top-[10px] w-[180px]">
            <div className="cursor-pointer rounded-lg border border-[#06b6d4]/40 bg-[#06b6d4]/10 px-4 py-3 text-center transition hover:border-[#06b6d4]/70">
              <p className="text-sm font-semibold text-[#67e8f9]">📡 {t("subRevenue")}</p>
              <p className="mt-1 font-mono text-xs text-[#06b6d4]">{fmtBNB(data.buyback?.sub_revenue_bnb ?? 0)} BNB</p>
            </div>
          </div>

          {/* Row 2: Buyback Wallet */}
          <div className="absolute left-[220px] top-[100px] w-[280px]">
            <a href={wallets.buyback_addr ? BSCSCAN + wallets.buyback_addr : "#"} target="_blank" rel="noopener noreferrer"
              className="block rounded-lg border border-[#f59e0b]/40 bg-[#f59e0b]/10 px-4 py-3 text-center transition hover:border-[#f59e0b]/70">
              <p className="text-sm font-semibold text-[#fbbf24]">💰 {t("buybackWallet")}</p>
              <p className="mt-1 font-mono text-xs text-[#d97706]">
                {fmtBNB(wallets.buyback_bnb)} BNB · {fmt(wallets.buyback_token)} $Ensoul
              </p>
              {wallets.buyback_addr && (
                <p className="mt-0.5 text-[10px] text-[#92400e]">{shortAddr(wallets.buyback_addr)}</p>
              )}
            </a>
          </div>

          {/* Row 3: Three-way Split */}
          {/* Mining Pool */}
          <div className="absolute left-[20px] top-[260px] w-[220px]">
            <a href={wallets.mining_pool_addr ? BSCSCAN + wallets.mining_pool_addr : "#"} target="_blank" rel="noopener noreferrer"
              className="block rounded-lg border border-[#22c55e]/40 bg-[#22c55e]/10 p-3 text-center transition hover:border-[#22c55e]/70">
              <p className="text-xs font-medium text-[#86efac]">⛏️ {t("miningPool")}</p>
              <p className="mt-1 font-mono text-lg font-bold text-[#4ade80]">{fmt(wallets.mining_pool_token)}</p>
              <p className="text-[10px] text-[#64748b]">$Ensoul {t("onChain")}</p>
              <div className="mt-1 flex justify-center gap-3 text-[10px]">
                <span className="text-[#86efac]">{split.mint_buyback_pct}% {t("mintLabel")}</span>
                <span className="text-[#86efac]">{split.sub_buyback_pct}% {t("subLabel")}</span>
              </div>
              {wallets.mining_pool_addr && (
                <p className="mt-0.5 text-[10px] text-[#4d7c0f]">{shortAddr(wallets.mining_pool_addr)}</p>
              )}
            </a>
          </div>

          {/* Treasury */}
          <div className="absolute left-[250px] top-[260px] w-[220px]">
            <a href={wallets.treasury_addr ? BSCSCAN + wallets.treasury_addr : "#"} target="_blank" rel="noopener noreferrer"
              className="block rounded-lg border border-[#3b82f6]/40 bg-[#3b82f6]/10 p-3 text-center transition hover:border-[#3b82f6]/70">
              <p className="text-xs font-medium text-[#93c5fd]">🏦 {t("treasury")}</p>
              <p className="mt-1 font-mono text-lg font-bold text-[#60a5fa]">BNB</p>
              <p className="text-[10px] text-[#64748b]">{t("coldStorage")}</p>
              <div className="mt-1 flex justify-center gap-3 text-[10px]">
                <span className="text-[#93c5fd]">{split.mint_treasury_pct}% {t("mintLabel")}</span>
                <span className="text-[#93c5fd]">{split.sub_treasury_pct}% {t("subLabel")}</span>
              </div>
              {wallets.treasury_addr && (
                <p className="mt-0.5 text-[10px] text-[#1e40af]">{shortAddr(wallets.treasury_addr)}</p>
              )}
            </a>
          </div>

          {/* Revenue Pool */}
          <div className="absolute left-[480px] top-[260px] w-[220px]">
            <a href={wallets.revenue_pool_addr ? BSCSCAN + wallets.revenue_pool_addr : "#"} target="_blank" rel="noopener noreferrer"
              className="block rounded-lg border border-[#a855f7]/40 bg-[#a855f7]/10 p-3 text-center transition hover:border-[#a855f7]/70">
              <p className="text-xs font-medium text-[#d8b4fe]">💎 {t("revenuePool")}</p>
              <p className="mt-1 font-mono text-lg font-bold text-[#c084fc]">{fmt(wallets.revenue_pool_token)}</p>
              <p className="text-[10px] text-[#64748b]">$Ensoul {t("onChain")}</p>
              <div className="mt-1 flex justify-center gap-3 text-[10px]">
                <span className="text-[#d8b4fe]">{split.mint_revenue_pool_pct}% {t("mintLabel")}</span>
                <span className="text-[#d8b4fe]">{split.sub_revenue_pool_pct}% {t("subLabel")}</span>
              </div>
              {wallets.revenue_pool_addr && (
                <p className="mt-0.5 text-[10px] text-[#6b21a8]">{shortAddr(wallets.revenue_pool_addr)}</p>
              )}
            </a>
          </div>

          {/* Row 4: Outputs */}
          <div className="absolute left-[40px] top-[440px] w-[180px] text-center">
            <Link href="/mining" className="text-xs font-medium text-[#4ade80] underline decoration-dotted hover:text-[#86efac]">
              → {t("clawRewards")}
            </Link>
            <p className="mt-0.5 text-[10px] text-[#64748b]">{t("dailyRelease")}: {fmt(data.mining_pool?.daily_limit ?? 0)}/d</p>
          </div>
          <div className="absolute left-[270px] top-[440px] w-[180px] text-center">
            <p className="text-xs font-medium text-[#60a5fa]">→ {t("coldStorage")}</p>
            <p className="mt-0.5 text-[10px] text-[#64748b]">{t("teamReserve")}</p>
          </div>
          <div className="absolute left-[500px] top-[440px] w-[180px] text-center">
            <Link href="/holder" className="text-xs font-medium text-[#c084fc] underline decoration-dotted hover:text-[#d8b4fe]">
              → {t("holderClaims")}
            </Link>
            <p className="mt-0.5 text-[10px] text-[#64748b]">{t("monthlyClaim")}</p>
          </div>
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
      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <KPICard
          label={t("totalBuyback")}
          value={fmt(data.buyback?.total_token_bought ?? 0) + " $Ensoul"}
          sub={fmtBNB(data.buyback?.total_bnb_spent ?? 0) + " BNB " + t("spent")}
        />
        <KPICard
          label={t("miningPoolBalance")}
          value={fmt(data.mining_pool?.balance ?? 0) + " $Ensoul"}
          sub={t("dailyLimit") + ": " + fmt(data.mining_pool?.daily_limit ?? 0)}
          color="text-[#4ade80]"
        />
        <KPICard
          label={t("revenuePoolBalance")}
          value={fmt(data.wallets?.revenue_pool_token ?? 0) + " $Ensoul"}
          sub={t("monthlyClaim")}
          color="text-[#c084fc]"
        />
      </div>

      {/* ── Flywheel Diagram ── */}
      <div className="mb-8">
        <FlywheelDiagram t={t} data={data} />
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
            data.mining_pool?.paused
              ? "bg-red-500/10 text-red-400"
              : "bg-green-500/10 text-green-400"
          }`}>
            {data.mining_pool?.paused ? t("paused") : t("active")}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          {[
            { label: t("poolBalance"), value: fmt(data.mining_pool?.balance ?? 0) },
            { label: t("dailyLimit"), value: fmt(data.mining_pool?.daily_limit ?? 0) },
            { label: t("dailyReleased"), value: fmt(data.mining_pool?.daily_released ?? 0) },
            { label: t("dailyRemaining"), value: fmt(data.mining_pool?.daily_remaining ?? 0) },
            { label: t("totalDeposited"), value: fmt(data.mining_pool?.total_deposited ?? 0) },
            { label: t("totalReleased"), value: fmt(data.mining_pool?.total_released ?? 0) },
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

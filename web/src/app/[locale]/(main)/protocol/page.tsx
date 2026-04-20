"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { economyApi, type EconomyOverview } from "@/lib/api";

function fmt(n: number, decimals = 2): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(decimals) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(decimals) + "K";
  return n.toFixed(decimals);
}

const protocolLinks = [
  {
    key: "economy",
    href: "/economy" as const,
    icon: "📊",
    color: "border-[#06b6d4]/30 hover:border-[#06b6d4]",
  },
  {
    key: "mint",
    href: "/mint" as const,
    icon: "🧬",
    color: "border-[#8b5cf6]/30 hover:border-[#8b5cf6]",
  },
  {
    key: "mining",
    href: "/mining" as const,
    icon: "⛏️",
    color: "border-[#22c55e]/30 hover:border-[#22c55e]",
  },
  {
    key: "claw",
    href: "/claw" as const,
    icon: "🦞",
    color: "border-[#f59e0b]/30 hover:border-[#f59e0b]",
  },
  {
    key: "leaderboard",
    href: "/leaderboard" as const,
    icon: "🏆",
    color: "border-[#06b6d4]/30 hover:border-[#06b6d4]",
  },
  {
    key: "holder",
    href: "/holder" as const,
    icon: "💰",
    color: "border-[#a855f7]/30 hover:border-[#a855f7]",
  },
];

export default function ProtocolPage() {
  const t = useTranslations("Protocol");
  const [overview, setOverview] = useState<EconomyOverview | null>(null);

  useEffect(() => {
    economyApi.overview().then(setOverview).catch(() => {});
  }, []);

  return (
    <div className="min-h-screen bg-[#0a0a0f] pt-24 pb-16">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="mb-12 text-center">
          <h1 className="text-3xl font-bold text-[#e2e8f0] sm:text-4xl">
            {t("title")}
          </h1>
          <p className="mt-3 text-base text-[#94a3b8]">
            {t("subtitle")}
          </p>
        </div>

        {/* Economy Overview KPIs */}
        {overview && (
          <div className="mb-12 grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
              <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">
                {t("totalBuyback")}
              </p>
              <p className="mt-2 font-mono text-xl font-bold text-[#8b5cf6]">
                {fmt(overview.buyback.total_token_bought)}
              </p>
              <p className="mt-1 text-xs text-[#64748b]">$ENSOUL</p>
            </div>
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
              <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">
                {t("miningPool")}
              </p>
              <p className="mt-2 font-mono text-xl font-bold text-[#22c55e]">
                {fmt(overview.wallets?.mining_pool_token ?? 0)}
              </p>
              <p className="mt-1 text-xs text-[#64748b]">$ENSOUL</p>
            </div>
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
              <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">
                {t("revenuePool")}
              </p>
              <p className="mt-2 font-mono text-xl font-bold text-[#a855f7]">
                {fmt(overview.wallets?.revenue_pool_token ?? 0)}
              </p>
              <p className="mt-1 text-xs text-[#64748b]">$ENSOUL</p>
            </div>
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5">
              <p className="text-xs font-medium uppercase tracking-wider text-[#94a3b8]">
                {t("totalBuybackBNB")}
              </p>
              <p className="mt-2 font-mono text-xl font-bold text-[#06b6d4]">
                {overview.buyback.total_bnb_spent.toFixed(4)}
              </p>
              <p className="mt-1 text-xs text-[#64748b]">BNB</p>
            </div>
          </div>
        )}

        {/* Protocol Feature Cards */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {protocolLinks.map((item) => (
            <Link
              key={item.key}
              href={item.href}
              className={`group rounded-lg border bg-[#14141f] p-6 transition-all ${item.color}`}
            >
              <div className="mb-3 text-2xl">{item.icon}</div>
              <h3 className="text-lg font-semibold text-[#e2e8f0] group-hover:text-white">
                {t(`${item.key}Title`)}
              </h3>
              <p className="mt-2 text-sm text-[#94a3b8]">
                {t(`${item.key}Desc`)}
              </p>
              <span className="mt-4 inline-block text-sm text-[#8b5cf6] group-hover:underline">
                {t("enter")} →
              </span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

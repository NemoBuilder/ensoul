"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { miningApi, type MiningPoolStatus, type FragmentDemand } from "@/lib/api";
import { dimensionLabels } from "@/lib/utils";

export default function MiningPage() {
  const t = useTranslations("Mining");
  const [pool, setPool] = useState<MiningPoolStatus | null>(null);
  const [demands, setDemands] = useState<FragmentDemand[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([miningApi.pool(), miningApi.demands()])
      .then(([p, d]) => {
        setPool(p);
        setDemands(Array.isArray(d) ? d : []);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
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

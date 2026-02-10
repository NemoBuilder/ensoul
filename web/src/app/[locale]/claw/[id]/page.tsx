"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import Image from "next/image";
import { clawApi, ClawProfile, ClawDimStat, ClawShellContrib, Fragment as FragmentType } from "@/lib/api";
import { timeAgo } from "@/lib/utils";

const DIMENSION_LABELS: Record<string, string> = {
  belief: "Belief",
  memory: "Memory",
  personality: "Personality",
  skill: "Skill",
  social: "Social",
  goal: "Goal",
};

const DIMENSION_COLORS: Record<string, string> = {
  belief: "#f59e0b",
  memory: "#3b82f6",
  personality: "#8b5cf6",
  skill: "#10b981",
  social: "#ec4899",
  goal: "#ef4444",
};

export default function ClawProfilePage() {
  const params = useParams();
  const id = params.id as string;
  const t = useTranslations("ClawProfile");

  const [profile, setProfile] = useState<ClawProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    clawApi
      .profile(id)
      .then((res) => {
        setProfile(res);
        setError(false);
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center pt-24">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  if (error || !profile) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center px-4 pt-24">
        <p className="mb-2 text-lg text-[#e2e8f0]">{t("notFound")}</p>
        <p className="mb-6 text-sm text-[#94a3b8]">{t("notFoundDesc")}</p>
        <Link href="/claw" className="text-[#8b5cf6] hover:underline">
          {t("backToClaws")}
        </Link>
      </div>
    );
  }

  const { claw, dimension_stats, shell_contributions, recent_accepted } = profile;

  return (
    <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
      {/* Header */}
      <div className="mb-8 rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6">
        <div className="flex items-start gap-4">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-[#1e1e2e] text-3xl">
            🦞
          </div>
          <div className="flex-1">
            <h1 className="text-2xl font-bold text-[#e2e8f0]">{claw.name}</h1>
            {claw.description && (
              <p className="mt-1 text-sm text-[#94a3b8]">{claw.description}</p>
            )}
            <div className="mt-2 flex items-center gap-3">
              <span
                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                  claw.status === "claimed"
                    ? "bg-green-500/10 text-green-400"
                    : "bg-yellow-500/10 text-yellow-400"
                }`}
              >
                {claw.status === "claimed" ? t("claimed") : t("pendingClaim")}
              </span>
              <span className="text-xs text-[#64748b]">
                {t("joined")} {timeAgo(claw.created_at)}
              </span>
            </div>
          </div>
        </div>

        {/* Stats Grid */}
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
          <StatCard label={t("submitted")} value={claw.total_submitted} />
          <StatCard label={t("accepted")} value={claw.total_accepted} color="text-green-400" />
          <StatCard label={t("acceptRate")} value={claw.accept_rate} />
          <StatCard label={t("earnings")} value={`${claw.earnings}`} color="text-[#f59e0b]" />
        </div>
      </div>

      {/* Dimension Breakdown */}
      {dimension_stats && dimension_stats.length > 0 && (
        <div className="mb-8 rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-sm font-medium text-[#94a3b8]">{t("dimensionBreakdown")}</h2>
          <div className="space-y-3">
            {dimension_stats.map((stat: ClawDimStat) => {
              const pct = stat.Total > 0 ? (stat.Accepted / stat.Total) * 100 : 0;
              const color = DIMENSION_COLORS[stat.Dimension.toLowerCase()] || "#8b5cf6";
              return (
                <div key={stat.Dimension}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="text-sm text-[#e2e8f0]">
                      {DIMENSION_LABELS[stat.Dimension.toLowerCase()] || stat.Dimension}
                    </span>
                    <span className="text-xs text-[#94a3b8]">
                      {stat.Accepted}/{stat.Total} {t("accepted").toLowerCase()}
                    </span>
                  </div>
                  <div className="h-2 overflow-hidden rounded-full bg-[#1e1e2e]">
                    <div
                      className="h-full rounded-full transition-all"
                      style={{ width: `${pct}%`, backgroundColor: color }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Shells Contributed To */}
      {shell_contributions && shell_contributions.length > 0 && (
        <div className="mb-8 rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-sm font-medium text-[#94a3b8]">{t("shellsContributed")}</h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {shell_contributions.map((sc: ClawShellContrib) => (
              <Link
                key={sc.ShellID}
                href={`/soul/${sc.Handle}`}
                className="flex items-center gap-3 rounded-lg border border-[#1e1e2e] p-3 transition-colors hover:border-[#8b5cf6]/30 hover:bg-[#1e1e2e]/50"
              >
                {sc.AvatarURL ? (
                  <Image
                    src={sc.AvatarURL}
                    alt={sc.Handle}
                    width={40}
                    height={40}
                    className="rounded-full"
                  />
                ) : (
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[#1e1e2e] text-sm text-[#94a3b8]">
                    👤
                  </div>
                )}
                <div className="flex-1 overflow-hidden">
                  <div className="truncate text-sm font-medium text-[#e2e8f0]">
                    {sc.DisplayName || `@${sc.Handle}`}
                  </div>
                  <div className="text-xs text-[#94a3b8]">@{sc.Handle}</div>
                </div>
                <div className="text-right">
                  <div className="text-sm font-medium text-green-400">{sc.AcceptedCount}</div>
                  <div className="text-xs text-[#64748b]">/ {sc.FragCount}</div>
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Recent Accepted Fragments */}
      {recent_accepted && recent_accepted.length > 0 && (
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-sm font-medium text-[#94a3b8]">{t("recentAccepted")}</h2>
          <div className="space-y-3">
            {recent_accepted.map((frag: FragmentType) => (
              <div
                key={frag.id}
                className="rounded-lg border border-[#1e1e2e] p-4 transition-colors hover:bg-[#1e1e2e]/30"
              >
                <div className="mb-2 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span
                      className="inline-block rounded px-1.5 py-0.5 text-xs font-medium"
                      style={{
                        backgroundColor: `${DIMENSION_COLORS[frag.dimension?.toLowerCase()] || "#8b5cf6"}20`,
                        color: DIMENSION_COLORS[frag.dimension?.toLowerCase()] || "#8b5cf6",
                      }}
                    >
                      {DIMENSION_LABELS[frag.dimension?.toLowerCase()] || frag.dimension}
                    </span>
                    {frag.shell && (
                      <Link
                        href={`/soul/${frag.shell.handle}`}
                        className="text-xs text-[#8b5cf6] hover:underline"
                      >
                        @{frag.shell.handle}
                      </Link>
                    )}
                  </div>
                  <span className="text-xs text-[#64748b]">{timeAgo(frag.created_at)}</span>
                </div>
                <p className="line-clamp-3 text-sm text-[#cbd5e1]">
                  {frag.content || (
                    <span className="flex items-center gap-1.5 text-[#64748b] italic">
                      🔒 <span className="font-mono text-xs">{frag.content_hash ? `SHA256:${frag.content_hash.slice(0, 12)}…` : "protected"}</span>
                    </span>
                  )}
                </p>
                <div className="mt-2 flex items-center gap-3">
                  <span className="text-xs text-[#64748b]">
                    {t("confidence")}: {Math.round(frag.confidence * 100)}%
                  </span>
                  {frag.tx_hash && (
                    <span className="text-xs text-green-400/60">✓ {t("onchain")}</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Back Link */}
      <div className="mt-8 text-center">
        <Link href="/leaderboard" className="text-sm text-[#94a3b8] hover:text-[#8b5cf6] hover:underline">
          ← {t("backToLeaderboard")}
        </Link>
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  color = "text-[#e2e8f0]",
}: {
  label: string;
  value: string | number;
  color?: string;
}) {
  return (
    <div className="rounded-lg border border-[#1e1e2e] bg-[#0a0a14] p-3 text-center">
      <div className={`text-xl font-bold ${color}`}>{value}</div>
      <div className="text-xs text-[#64748b]">{label}</div>
    </div>
  );
}

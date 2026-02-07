"use client";

import { useState, useEffect, use } from "react";
import Link from "next/link";
import Image from "next/image";
import { clawApi, ClawProfile, ClawDimStat, ClawShellContrib, Fragment } from "@/lib/api";
import { dimensionLabels, timeAgo } from "@/lib/utils";

export default function ClawProfilePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const [profile, setProfile] = useState<ClawProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    clawApi
      .profile(id)
      .then(setProfile)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center pt-16">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  if (error || !profile) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16 text-center">
        <h2 className="mb-2 text-2xl font-bold text-[#e2e8f0]">Claw Not Found</h2>
        <p className="mb-6 text-[#94a3b8]">{error || "This claw does not exist."}</p>
        <Link href="/explore" className="text-[#8b5cf6] hover:underline">← Back to Explore</Link>
      </div>
    );
  }

  const { claw, dimension_stats, shell_contributions, recent_accepted } = profile;

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      {/* Header */}
      <div className="mb-8 flex flex-col gap-6 sm:flex-row sm:items-start">
        {/* Claw avatar */}
        <div className="flex h-20 w-20 flex-shrink-0 items-center justify-center rounded-xl border-2 border-[#8b5cf6]/50 bg-[#1e1e2e] text-3xl">
          🦞
        </div>

        <div className="flex-1">
          <h1 className="mb-1 text-2xl font-bold text-[#e2e8f0]">{claw.name}</h1>
          <p className="mb-3 text-sm text-[#94a3b8]">{claw.description || "No description"}</p>
          <div className="flex flex-wrap items-center gap-3 text-xs text-[#94a3b8]">
            <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
              claw.status === "claimed"
                ? "bg-green-500/10 text-green-400"
                : "bg-yellow-500/10 text-yellow-400"
            }`}>
              {claw.status}
            </span>
            <span>Joined {timeAgo(claw.created_at)}</span>
          </div>
        </div>
      </div>

      {/* Stats bar */}
      <div className="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {[
          { label: "Submitted", value: claw.total_submitted },
          { label: "Accepted", value: claw.total_accepted },
          { label: "Accept Rate", value: claw.accept_rate },
          { label: "Earnings", value: `${claw.earnings.toFixed(4)} BNB` },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
            <div className="text-lg font-bold text-[#e2e8f0]">{s.value}</div>
            <div className="text-xs text-[#94a3b8]">{s.label}</div>
          </div>
        ))}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Dimension breakdown */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Dimension Breakdown</h3>
          {dimension_stats && dimension_stats.length > 0 ? (
            <div className="space-y-3">
              {dimension_stats.map((d: ClawDimStat) => {
                const rate = d.Total > 0 ? Math.round((d.Accepted / d.Total) * 100) : 0;
                return (
                  <div key={d.Dimension}>
                    <div className="mb-1 flex items-center justify-between text-sm">
                      <span className="text-[#e2e8f0]">
                        {dimensionLabels[d.Dimension] || d.Dimension}
                      </span>
                      <span className="text-xs text-[#94a3b8]">
                        {d.Accepted}/{d.Total} accepted ({rate}%)
                      </span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-[#1e1e2e]">
                      <div
                        className="h-full rounded-full bg-[#8b5cf6] transition-all"
                        style={{ width: `${rate}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <p className="text-sm text-[#94a3b8]">No contributions yet.</p>
          )}
        </div>

        {/* Souls contributed to */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Souls Contributed To</h3>
          {shell_contributions && shell_contributions.length > 0 ? (
            <div className="space-y-3">
              {shell_contributions.map((s: ClawShellContrib) => (
                <Link
                  key={s.ShellID}
                  href={`/soul/${s.Handle}`}
                  className="flex items-center gap-3 rounded-lg border border-[#1e1e2e] p-3 transition-colors hover:border-[#8b5cf6]/30 hover:bg-[#1e1e2e]/50"
                >
                  <div className="relative h-8 w-8 flex-shrink-0 overflow-hidden rounded-full bg-[#1e1e2e]">
                    {s.AvatarURL ? (
                      <Image src={s.AvatarURL} alt={s.Handle} fill className="object-cover" unoptimized />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-xs text-[#94a3b8]">
                        {s.Handle?.[0]?.toUpperCase() || "?"}
                      </div>
                    )}
                  </div>
                  <div className="flex-1">
                    <span className="text-sm font-medium text-[#e2e8f0]">
                      {s.DisplayName || `@${s.Handle}`}
                    </span>
                  </div>
                  <div className="text-right text-xs text-[#94a3b8]">
                    <span className="text-green-400">{s.AcceptedCount}</span>/{s.FragCount} accepted
                  </div>
                </Link>
              ))}
            </div>
          ) : (
            <p className="text-sm text-[#94a3b8]">No contributions yet.</p>
          )}
        </div>
      </div>

      {/* Recent accepted fragments */}
      <div className="mt-8">
        <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Recent Accepted Fragments</h3>
        {recent_accepted && recent_accepted.length > 0 ? (
          <div className="space-y-3">
            {recent_accepted.map((f: Fragment) => (
              <div key={f.id} className="rounded-lg border border-green-500/20 bg-green-500/5 p-4">
                <div className="mb-2 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="rounded-full bg-green-500/10 px-2 py-0.5 text-xs font-medium text-green-400">
                      accepted
                    </span>
                    <span className="text-xs text-[#94a3b8]">
                      {dimensionLabels[f.dimension] || f.dimension}
                    </span>
                    {f.shell && (
                      <Link href={`/soul/${f.shell.handle}`} className="text-xs text-[#8b5cf6] hover:underline">
                        @{f.shell.handle}
                      </Link>
                    )}
                    {f.confidence > 0 && (
                      <span className="text-xs text-[#94a3b8]">
                        · {Math.round(f.confidence * 100)}%
                      </span>
                    )}
                  </div>
                  <span className="text-xs text-[#94a3b8]">{timeAgo(f.created_at)}</span>
                </div>
                <p className="text-sm leading-relaxed text-[#e2e8f0]">{f.content}</p>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
            <p className="text-sm text-[#94a3b8]">No accepted fragments yet.</p>
          </div>
        )}
      </div>
    </div>
  );
}

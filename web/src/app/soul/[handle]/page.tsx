"use client";

import { useState, useEffect, use } from "react";
import Image from "next/image";
import Link from "next/link";
import { shellApi, fragmentApi, Shell, Fragment, Ensouling, ShellContributor } from "@/lib/api";
import { stageConfig, dimensionLabels, timeAgo, truncateAddr, calcCompletion, Stage } from "@/lib/utils";
import RadarChart from "@/components/RadarChart";

type Tab = "fragments" | "dimensions" | "history";

export default function SoulPage({
  params,
}: {
  params: Promise<{ handle: string }>;
}) {
  const { handle } = use(params);
  const [shell, setShell] = useState<Shell | null>(null);
  const [fragments, setFragments] = useState<Fragment[]>([]);
  const [history, setHistory] = useState<Ensouling[]>([]);
  const [contributors, setContributors] = useState<ShellContributor[]>([]);
  const [tab, setTab] = useState<Tab>("fragments");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [imgErr, setImgErr] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const s = await shellApi.get(handle);
        setShell(s);
        // Load fragments and history in parallel
        const [fragRes, hist, contribs] = await Promise.all([
          fragmentApi.list({ handle, limit: 50 }),
          shellApi.getHistory(handle).catch(() => []),
          shellApi.getContributors(handle).catch(() => ({ contributors: [] })),
        ]);
        setFragments(fragRes.fragments || []);
        setHistory(hist || []);
        setContributors(contribs.contributors || []);
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : "Failed to load soul");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [handle]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center pt-16">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  if (error || !shell) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16 text-center">
        <h2 className="mb-2 text-2xl font-bold text-[#e2e8f0]">Soul Not Found</h2>
        <p className="mb-6 text-[#94a3b8]">{error || "This soul does not exist."}</p>
        <Link href="/explore" className="text-[#8b5cf6] hover:underline">
          ← Back to Explore
        </Link>
      </div>
    );
  }

  const stage = stageConfig[shell.stage as Stage] || stageConfig.embryo;
  const completion = calcCompletion(shell.dimensions || {});
  const dims = shell.dimensions || {};

  const acceptedFrags = fragments.filter((f) => f.status === "accepted");
  const pendingFrags = fragments.filter((f) => f.status === "pending");
  const rejectedFrags = fragments.filter((f) => f.status === "rejected");

  const tabs: { key: Tab; label: string; count?: number }[] = [
    { key: "fragments", label: "Fragments", count: fragments.length },
    { key: "dimensions", label: "Dimensions", count: Object.keys(dims).length },
    { key: "history", label: "Evolution", count: history.length },
  ];

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      {/* Header */}
      <div className="mb-8 flex flex-col gap-6 sm:flex-row sm:items-start">
        {/* Avatar */}
        <div className={`relative h-24 w-24 flex-shrink-0 overflow-hidden rounded-full border-2 ${stage.borderClass}`}>
          {shell.avatar_url && !imgErr ? (
            <Image
              src={shell.avatar_url}
              alt={shell.handle}
              fill
              className="object-cover"
              onError={() => setImgErr(true)}
              unoptimized
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center bg-[#1e1e2e] text-2xl text-[#94a3b8]">
              {shell.handle[0]?.toUpperCase() || "?"}
            </div>
          )}
        </div>

        {/* Info */}
        <div className="flex-1">
          <div className="mb-1 flex items-center gap-3">
            <h1 className="text-2xl font-bold text-[#e2e8f0]">
              {shell.display_name || `@${shell.handle}`}
            </h1>
            <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${stage.bgClass} ${stage.textClass}`}>
              {stage.label}
            </span>
          </div>
          <a
            href={`https://x.com/${shell.handle}`}
            target="_blank"
            rel="noopener noreferrer"
            className="mb-2 inline-flex items-center gap-1 text-sm text-[#94a3b8] hover:text-[#8b5cf6] transition-colors"
          >
            <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            @{shell.handle}
          </a>
          <p className="mb-4 text-sm leading-relaxed text-[#94a3b8]">
            {shell.seed_summary || "No summary yet."}
          </p>
          <div className="flex flex-wrap items-center gap-4 text-xs text-[#94a3b8]">
            <span>DNA v{shell.dna_version}</span>
            <span>·</span>
            <span>Owner: {truncateAddr(shell.owner_addr)}</span>
            <span>·</span>
            <span>Minted {timeAgo(shell.created_at)}</span>
            {shell.agent_id != null && (
              <>
                <span>·</span>
                <span className="font-mono text-[#8b5cf6]">Token #{shell.agent_id}</span>
                <span>·</span>
                <a
                  href={`https://bscscan.com/nft/0x8004a169fb4a3325136eb29fa0ceb6d2e539a432/${shell.agent_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#8b5cf6] hover:underline"
                >
                  BscScan ↗
                </a>
                <a
                  href={`https://www.8004scan.io/agents/bsc/${shell.agent_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[#8b5cf6] hover:underline"
                >
                  8004Scan ↗
                </a>
              </>
            )}
          </div>
        </div>

        {/* Chat button */}
        <Link
          href={`/soul/${shell.handle}/chat`}
          className="flex-shrink-0 rounded-lg bg-[#8b5cf6] px-5 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
        >
          Chat with Soul
        </Link>
      </div>

      {/* Stats bar */}
      <div className="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-5">
        {[
          { label: "Completion", value: `${completion}%` },
          { label: "Fragments", value: shell.total_frags },
          { label: "Accepted", value: shell.accepted_frags },
          { label: "Claws", value: shell.total_claws },
          { label: "Chats", value: shell.total_chats },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-3 text-center">
            <div className="text-lg font-bold text-[#e2e8f0]">{s.value}</div>
            <div className="text-xs text-[#94a3b8]">{s.label}</div>
          </div>
        ))}
      </div>

      {/* Radar + Soul Prompt side by side */}
      <div className="mb-8 grid gap-6 lg:grid-cols-2">
        {/* Radar chart */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Soul Dimensions</h3>
          <RadarChart dimensions={dims} size={280} />
        </div>

        {/* Soul prompt */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Soul Identity</h3>
          {shell.soul_prompt ? (
            <div className="space-y-4">
              {/* Key dimension summaries */}
              {Object.entries(dims).filter(([, d]) => d.summary).length > 0 ? (
                Object.entries(dims)
                  .filter(([, d]) => d.summary)
                  .sort(([, a], [, b]) => b.score - a.score)
                  .map(([key, d]) => (
                    <div key={key}>
                      <div className="mb-1 flex items-center gap-2">
                        <span className="text-sm">
                          {key === "personality" ? "🧠" : key === "knowledge" ? "📚" : key === "stance" ? "🎯" : key === "style" ? "✍️" : key === "relationship" ? "🤝" : "⏳"}
                        </span>
                        <span className="text-xs font-medium text-[#8b5cf6]">
                          {dimensionLabels[key] || key}
                        </span>
                        <span className="text-xs text-[#94a3b8]">· {d.score}%</span>
                      </div>
                      <p className="text-xs leading-relaxed text-[#e2e8f0]/80">
                        {d.summary}
                      </p>
                    </div>
                  ))
              ) : (
                <div className="space-y-3">
                  <p className="text-sm leading-relaxed text-[#e2e8f0]">
                    {shell.seed_summary || "This soul is still forming its identity..."}
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <span className="rounded-full bg-[#8b5cf6]/10 px-2.5 py-1 text-xs text-[#a78bfa]">
                      DNA v{shell.dna_version}
                    </span>
                    <span className="rounded-full bg-[#8b5cf6]/10 px-2.5 py-1 text-xs text-[#a78bfa]">
                      {shell.accepted_frags} fragments absorbed
                    </span>
                    <span className={`rounded-full px-2.5 py-1 text-xs ${stage.bgClass} ${stage.textClass}`}>
                      {stage.label}
                    </span>
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-[#f59e0b]">
                <span>🥚</span>
                <span className="text-sm font-medium">Embryo Stage</span>
              </div>
              <p className="text-xs leading-relaxed text-[#94a3b8]">
                This soul hasn&apos;t awakened yet. As more fragments are contributed and absorbed, 
                a unique identity will emerge — personality, knowledge, communication style, 
                and worldview will gradually crystallize.
              </p>
              <div className="rounded-lg border border-dashed border-[#1e1e2e] p-3 text-center">
                <p className="text-xs text-[#94a3b8]">
                  Need <span className="text-[#8b5cf6]">10 accepted fragments</span> to trigger first Ensouling
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Top Contributors */}
      {contributors.length > 0 && (
        <div className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">Top Contributors</h3>
          <div className="flex flex-wrap gap-3">
            {contributors.map((c, i) => (
              <Link
                key={c.claw_id}
                href={`/claw/${c.claw_id}`}
                className="flex items-center gap-2 rounded-lg border border-[#1e1e2e] px-3 py-2 transition-colors hover:border-[#8b5cf6]/30 hover:bg-[#1e1e2e]/50"
              >
                <span className="text-sm">
                  {i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : "🦞"}
                </span>
                <span className="text-sm font-medium text-[#e2e8f0]">{c.name}</span>
                <span className="text-xs text-[#94a3b8]">
                  {c.accepted_frags} accepted
                </span>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="mb-6 flex gap-1 border-b border-[#1e1e2e]">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`border-b-2 px-4 py-2.5 text-sm font-medium transition-colors ${
              tab === t.key
                ? "border-[#8b5cf6] text-[#8b5cf6]"
                : "border-transparent text-[#94a3b8] hover:text-[#e2e8f0]"
            }`}
          >
            {t.label}
            {t.count !== undefined && (
              <span className="ml-1.5 text-xs opacity-60">({t.count})</span>
            )}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {tab === "fragments" && (
        <FragmentFeed
          accepted={acceptedFrags}
          pending={pendingFrags}
          rejected={rejectedFrags}
        />
      )}
      {tab === "dimensions" && <DimensionDetail dims={dims} />}
      {tab === "history" && <HistoryTimeline history={history} />}
    </div>
  );
}

// --- Fragment Feed ---
function FragmentFeed({
  accepted,
  pending,
  rejected,
}: {
  accepted: Fragment[];
  pending: Fragment[];
  rejected: Fragment[];
}) {
  const all = [...accepted, ...pending, ...rejected].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );

  if (all.length === 0) {
    return (
      <div className="py-12 text-center text-[#94a3b8]">
        <p className="mb-1 text-lg">No fragments yet</p>
        <p className="text-sm">Be the first Claw to contribute a fragment to this soul.</p>
      </div>
    );
  }

  const statusStyles = {
    accepted: "border-green-500/30 bg-green-500/5",
    pending: "border-yellow-500/30 bg-yellow-500/5",
    rejected: "border-red-500/30 bg-red-500/5",
  };

  const statusBadge = {
    accepted: "text-green-400 bg-green-500/10",
    pending: "text-yellow-400 bg-yellow-500/10",
    rejected: "text-red-400 bg-red-500/10",
  };

  return (
    <div className="space-y-3">
      {all.map((f) => (
        <div
          key={f.id}
          className={`rounded-lg border p-4 ${statusStyles[f.status]}`}
        >
          <div className="mb-2 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusBadge[f.status]}`}>
                {f.status}
              </span>
              <span className="text-xs text-[#94a3b8]">
                {dimensionLabels[f.dimension] || f.dimension}
              </span>
              {f.claw && f.claw.name && (
                <Link href={`/claw/${f.claw.id}`} className="text-xs text-[#8b5cf6] hover:underline">
                  🦞 {f.claw.name}
                </Link>
              )}
              {f.confidence > 0 && (
                <span className="text-xs text-[#94a3b8]">
                  · {Math.round(f.confidence * 100)}% confidence
                </span>
              )}
            </div>
            <span className="text-xs text-[#94a3b8]">{timeAgo(f.created_at)}</span>
          </div>
          <p className="text-sm leading-relaxed text-[#e2e8f0]">{f.content}</p>
          {f.reject_reason && (
            <p className="mt-2 text-xs text-red-400">Reason: {f.reject_reason}</p>
          )}
        </div>
      ))}
    </div>
  );
}

// --- Dimension Detail ---
function DimensionDetail({
  dims,
}: {
  dims: Record<string, { score: number; summary: string }>;
}) {
  const entries = Object.entries(dims);

  if (entries.length === 0) {
    return (
      <div className="py-12 text-center text-[#94a3b8]">
        <p>No dimension data available yet.</p>
      </div>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {entries.map(([key, dim]) => (
        <div
          key={key}
          className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5"
        >
          <div className="mb-3 flex items-center justify-between">
            <h4 className="font-medium text-[#e2e8f0]">
              {dimensionLabels[key] || key}
            </h4>
            <span className="font-mono text-sm font-bold text-[#8b5cf6]">
              {dim.score}
            </span>
          </div>
          {/* Score bar */}
          <div className="mb-3 h-1.5 overflow-hidden rounded-full bg-[#1e1e2e]">
            <div
              className="h-full rounded-full bg-[#8b5cf6] transition-all"
              style={{ width: `${Math.min(dim.score, 100)}%` }}
            />
          </div>
          <p className="text-sm leading-relaxed text-[#94a3b8]">
            {dim.summary || "No analysis yet."}
          </p>
        </div>
      ))}
    </div>
  );
}

// --- History Timeline ---
function HistoryTimeline({ history }: { history: Ensouling[] }) {
  if (history.length === 0) {
    return (
      <div className="py-12 text-center text-[#94a3b8]">
        <p className="mb-1 text-lg">No evolution yet</p>
        <p className="text-sm">
          Ensouling happens when enough quality fragments accumulate.
        </p>
      </div>
    );
  }

  return (
    <div className="relative space-y-6 pl-6">
      {/* Vertical line */}
      <div className="absolute top-0 bottom-0 left-2 w-px bg-[#1e1e2e]" />

      {history.map((e) => (
        <div key={e.id} className="relative">
          {/* Dot */}
          <div className="absolute -left-4 top-1.5 h-3 w-3 rounded-full border-2 border-[#8b5cf6] bg-[#0a0a0f]" />
          <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
            <div className="mb-2 flex items-center justify-between">
              <span className="font-mono text-sm font-medium text-[#8b5cf6]">
                v{e.version_from} → v{e.version_to}
              </span>
              <span className="text-xs text-[#94a3b8]">{timeAgo(e.created_at)}</span>
            </div>
            <p className="mb-1 text-xs text-[#94a3b8]">
              {e.frags_merged} fragments merged
            </p>
            <p className="text-sm leading-relaxed text-[#e2e8f0]">
              {e.summary_diff || "Evolution completed."}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

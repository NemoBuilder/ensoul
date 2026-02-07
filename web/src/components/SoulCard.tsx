"use client";

import Link from "next/link";
import { stageConfig, type Stage, calcCompletion } from "@/lib/utils";
import type { Shell } from "@/lib/api";

interface SoulCardProps {
  soul: Shell;
}

export default function SoulCard({ soul }: SoulCardProps) {
  const stage = stageConfig[soul.stage as Stage] || stageConfig.embryo;
  const completion = calcCompletion(soul.dimensions || {});

  return (
    <Link href={`/soul/${soul.handle}`}>
      <div className="group relative overflow-hidden rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5 transition-all hover:border-[#8b5cf6]/40 hover:shadow-lg hover:shadow-[#8b5cf6]/5">
        {/* Avatar + Handle */}
        <div className="mb-4 flex items-center gap-3">
          <div className="relative h-12 w-12 overflow-hidden rounded-full bg-[#1e1e2e]">
            {soul.avatar_url ? (
              <img
                src={soul.avatar_url}
                alt={`@${soul.handle}`}
                className="h-full w-full object-cover"
              />
            ) : (
              <div className="flex h-full w-full items-center justify-center text-lg text-[#94a3b8]">
                {soul.handle?.charAt(0)?.toUpperCase() || "?"}
              </div>
            )}
          </div>
          <div className="min-w-0 flex-1">
            <div className="truncate font-semibold text-[#e2e8f0] group-hover:text-[#8b5cf6] transition-colors">
              @{soul.handle}
            </div>
            {soul.display_name && soul.display_name !== soul.handle && (
              <div className="truncate text-xs text-[#94a3b8]">
                {soul.display_name}
              </div>
            )}
          </div>
        </div>

        {/* Progress bar */}
        <div className="mb-3">
          <div className="mb-1 flex items-center justify-between">
            <span className="text-xs text-[#94a3b8]">Completion</span>
            <span className="font-mono text-xs text-[#94a3b8]">
              {completion}%
            </span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-[#1e1e2e]">
            <div
              className="h-full rounded-full transition-all duration-500"
              style={{
                width: `${Math.min(completion, 100)}%`,
                backgroundColor: stage.color,
              }}
            />
          </div>
        </div>

        {/* Stats row */}
        <div className="mb-3 flex items-center gap-4 text-xs text-[#94a3b8]">
          <span title="Total fragments">
            📄 {soul.total_frags}
          </span>
          <span title="Contributing claws">
            🦞 {soul.total_claws}
          </span>
          <span title="DNA version" className="font-mono">
            v{soul.dna_version}
          </span>
        </div>

        {/* Stage badge */}
        <div className="flex items-center justify-between">
          <span
            className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${stage.borderClass} ${stage.bgClass} ${stage.textClass}`}
          >
            <span
              className="h-1.5 w-1.5 rounded-full"
              style={{ backgroundColor: stage.color }}
            />
            {stage.label}
          </span>
          <span className="text-xs text-[#94a3b8]/60 opacity-0 transition-opacity group-hover:opacity-100">
            View Soul →
          </span>
        </div>
      </div>
    </Link>
  );
}

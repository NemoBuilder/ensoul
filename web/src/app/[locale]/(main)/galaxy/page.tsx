"use client";

// V4 Galaxy explorer. Public, paginated by stage filter.
// Deliberately self-contained: no i18n keys yet — strings are hardcoded EN
// and will be lifted into messages/ once the V4 copy is finalised.

import Link from "next/link";
import { useEffect, useState } from "react";
import { galaxyApi, type Galaxy } from "@/lib/api";

const STAGES: { key: string; label: string }[] = [
  { key: "", label: "All" },
  { key: "embryo", label: "Embryo" },
  { key: "growing", label: "Growing" },
  { key: "mature", label: "Mature" },
  { key: "raising", label: "Raising" },
  { key: "graduated", label: "Graduated" },
];

export default function GalaxyListPage() {
  const [galaxies, setGalaxies] = useState<Galaxy[]>([]);
  const [stage, setStage] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    galaxyApi
      .list({ stage: stage || undefined })
      .then((res) => {
        if (!cancelled) setGalaxies(res.galaxies || []);
      })
      .catch((e: Error) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [stage]);

  return (
    <div className="mx-auto max-w-7xl px-4 pt-24 pb-16">
      <div className="mb-8 flex items-end justify-between">
        <div>
          <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">Galaxies</h1>
          <p className="text-[#94a3b8]">
            Knowledge graphs grown atom by atom. Any topic can become a galaxy.
          </p>
        </div>
        <Link
          href="./galaxy/apply"
          className="rounded-md border border-[#8b5cf6] bg-[#8b5cf6]/10 px-4 py-2 text-sm text-[#8b5cf6] hover:bg-[#8b5cf6]/20"
        >
          Propose a galaxy
        </Link>
      </div>

      <div className="mb-8 flex flex-wrap gap-3">
        {STAGES.map((s) => (
          <button
            key={s.key || "all"}
            onClick={() => setStage(s.key)}
            className={`rounded-md border px-4 py-2 text-sm transition-colors ${
              stage === s.key
                ? "border-[#8b5cf6] bg-[#8b5cf6]/10 text-[#8b5cf6]"
                : "border-[#1e1e2e] text-[#94a3b8] hover:border-[#8b5cf6]/50 hover:text-[#e2e8f0]"
            }`}
          >
            {s.label}
          </button>
        ))}
      </div>

      {error ? (
        <div className="rounded-md border border-red-500/30 bg-red-500/10 p-4 text-red-300">
          {error}
        </div>
      ) : loading ? (
        <div className="text-[#94a3b8]">Loading…</div>
      ) : galaxies.length === 0 ? (
        <div className="text-[#94a3b8]">No galaxies yet. Be the first to propose one.</div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {galaxies.map((g) => (
            <GalaxyCard key={g.id} g={g} />
          ))}
        </div>
      )}
    </div>
  );
}

function GalaxyCard({ g }: { g: Galaxy }) {
  return (
    <Link
      href={`./galaxy/${g.slug}`}
      className="block rounded-lg border border-[#1e1e2e] bg-[#0a0a14] p-5 transition-colors hover:border-[#8b5cf6]/60"
    >
      <div className="mb-2 flex items-center justify-between">
        <h2 className="truncate text-lg font-semibold text-[#e2e8f0]">{g.title}</h2>
        <span className="rounded bg-[#8b5cf6]/10 px-2 py-0.5 text-xs text-[#8b5cf6]">
          {g.stage}
        </span>
      </div>
      {g.subtitle && (
        <p className="mb-3 line-clamp-2 text-sm text-[#94a3b8]">{g.subtitle}</p>
      )}
      <div className="grid grid-cols-3 gap-2 text-xs text-[#64748b]">
        <Stat label="Atoms" value={g.atom_count} />
        <Stat label="Contribs" value={g.contrib_count} />
        <Stat label="Maturity" value={g.maturity_score.toFixed(0)} />
      </div>
    </Link>
  );
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="text-center">
      <div className="text-base font-semibold text-[#e2e8f0]">{value}</div>
      <div>{label}</div>
    </div>
  );
}

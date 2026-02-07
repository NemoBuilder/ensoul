"use client";

import { useEffect, useState, useCallback } from "react";
import { shellApi, type Shell } from "@/lib/api";
import SoulCard from "@/components/SoulCard";

const stages = [
  { key: "", label: "All" },
  { key: "embryo", label: "Embryo" },
  { key: "growing", label: "Growing" },
  { key: "mature", label: "Mature" },
  { key: "evolving", label: "Evolving" },
];

const sortOptions = [
  { key: "newest", label: "Newest" },
  { key: "most_fragments", label: "Most Fragments" },
  { key: "hot", label: "Hot" },
];

export default function ExplorePage() {
  const [souls, setSouls] = useState<Shell[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [stage, setStage] = useState("");
  const [sort, setSort] = useState("newest");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const limit = 12;

  const fetchSouls = useCallback(async () => {
    setLoading(true);
    try {
      const result = await shellApi.list({ stage, sort, search, page, limit });
      setSouls(result.shells || []);
      setTotal(result.total);
    } catch {
      setSouls([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [stage, sort, search, page]);

  useEffect(() => {
    fetchSouls();
  }, [fetchSouls]);

  // Reset page when filters change
  useEffect(() => {
    setPage(1);
  }, [stage, sort, search]);

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="mx-auto max-w-7xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
        Explore Souls
      </h1>
      <p className="mb-8 text-[#94a3b8]">
        Browse all minted souls. Filter by stage, search by handle.
      </p>

      {/* Filter bar */}
      <div className="mb-8 flex flex-wrap items-center gap-3">
        {stages.map((s) => (
          <button
            key={s.key}
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

        <select
          value={sort}
          onChange={(e) => setSort(e.target.value)}
          className="rounded-md border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#94a3b8] outline-none focus:border-[#8b5cf6]"
        >
          {sortOptions.map((o) => (
            <option key={o.key} value={o.key}>
              {o.label}
            </option>
          ))}
        </select>

        <input
          type="text"
          placeholder="Search by handle..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="ml-auto rounded-md border border-[#1e1e2e] bg-[#14141f] px-4 py-2 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none focus:border-[#8b5cf6]"
        />
      </div>

      {/* Results */}
      {loading ? (
        <div className="flex min-h-[40vh] items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      ) : souls.length === 0 ? (
        <div className="flex min-h-[40vh] flex-col items-center justify-center text-[#94a3b8]">
          <p className="text-lg">No souls found</p>
          <p className="mt-2 text-sm">
            {search
              ? `No results for "${search}". Try a different search.`
              : "Be the first to mint a shell!"}
          </p>
        </div>
      ) : (
        <>
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {souls.map((soul) => (
              <SoulCard key={soul.id} soul={soul} />
            ))}
          </div>

          {totalPages > 1 && (
            <div className="mt-8 flex items-center justify-center gap-2">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="rounded-md border border-[#1e1e2e] px-3 py-1.5 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#8b5cf6] disabled:opacity-30"
              >
                ← Prev
              </button>
              <span className="px-3 text-sm text-[#94a3b8]">
                Page {page} of {totalPages}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
                className="rounded-md border border-[#1e1e2e] px-3 py-1.5 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#8b5cf6] disabled:opacity-30"
              >
                Next →
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

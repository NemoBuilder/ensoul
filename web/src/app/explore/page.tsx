"use client";

import { useEffect, useState, useCallback, useRef } from "react";
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
  const [loadingMore, setLoadingMore] = useState(false);
  const [stage, setStage] = useState("");
  const [sort, setSort] = useState("newest");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const limit = 12;
  const sentinelRef = useRef<HTMLDivElement>(null);

  const hasMore = souls.length < total;

  // Fetch first page (reset)
  const fetchSouls = useCallback(async () => {
    setLoading(true);
    try {
      const result = await shellApi.list({ stage, sort, search, page: 1, limit });
      setSouls(result.shells || []);
      setTotal(result.total);
      setPage(1);
    } catch {
      setSouls([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [stage, sort, search]);

  // Fetch next page (append)
  const loadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    const nextPage = page + 1;
    setLoadingMore(true);
    try {
      const result = await shellApi.list({ stage, sort, search, page: nextPage, limit });
      setSouls((prev) => [...prev, ...(result.shells || [])]);
      setTotal(result.total);
      setPage(nextPage);
    } catch {
      // silently fail, user can scroll again
    } finally {
      setLoadingMore(false);
    }
  }, [stage, sort, search, page, loadingMore, hasMore]);

  // Reset on filter change
  useEffect(() => {
    fetchSouls();
  }, [fetchSouls]);

  // IntersectionObserver for infinite scroll
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !loading && !loadingMore) {
          loadMore();
        }
      },
      { rootMargin: "200px" } // trigger 200px before reaching bottom
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, [hasMore, loading, loadingMore, loadMore]);

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

          {/* Sentinel + loading indicator */}
          <div ref={sentinelRef} className="mt-8 flex justify-center py-4">
            {loadingMore && (
              <div className="flex items-center gap-2 text-sm text-[#94a3b8]">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
                Loading more...
              </div>
            )}
            {!hasMore && souls.length > 0 && (
              <p className="text-sm text-[#94a3b8]/50">
                All {total} souls loaded
              </p>
            )}
          </div>
        </>
      )}
    </div>
  );
}

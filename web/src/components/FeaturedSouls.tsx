"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { shellApi, Shell } from "@/lib/api";
import SoulCard from "@/components/SoulCard";

// Fetches and displays the top souls on the landing page.
export default function FeaturedSouls() {
  const [shells, setShells] = useState<Shell[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    shellApi
      .list({ sort: "hot", limit: 6 })
      .then((res) => setShells(res.shells || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  if (shells.length === 0) {
    return (
      <p className="text-center text-[#94a3b8]">
        Souls will appear here once minted. Be the first to{" "}
        <Link href="/mint" className="text-[#8b5cf6] hover:underline">
          mint a shell
        </Link>
        .
      </p>
    );
  }

  return (
    <div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {shells.map((s) => (
          <SoulCard key={s.id} soul={s} />
        ))}
      </div>
      {shells.length >= 6 && (
        <div className="mt-8 text-center">
          <Link
            href="/explore"
            className="rounded-lg border border-[#1e1e2e] px-6 py-2.5 text-sm font-medium text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#8b5cf6]"
          >
            View All Souls →
          </Link>
        </div>
      )}
    </div>
  );
}

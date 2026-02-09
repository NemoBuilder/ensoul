"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { statsApi, type GlobalStats } from "@/lib/api";

export default function StatsBar() {
  const t = useTranslations("Stats");
  const [stats, setStats] = useState<GlobalStats>({
    souls: 0,
    fragments: 0,
    claws: 0,
    chats: 0,
  });

  useEffect(() => {
    statsApi
      .global()
      .then(setStats)
      .catch(() => {
        setStats({ souls: 0, fragments: 0, claws: 0, chats: 0 });
      });
  }, []);

  const items = [
    { label: t("souls"), value: stats.souls },
    { label: t("fragments"), value: stats.fragments },
    { label: t("claws"), value: stats.claws },
    { label: t("chats"), value: stats.chats },
  ];

  return (
    <section className="border-y border-[#1e1e2e] bg-[#14141f]/50 py-8">
      <div className="mx-auto flex max-w-4xl items-center justify-around px-4">
        {items.map((item) => (
          <div key={item.label} className="text-center">
            <div className="font-mono text-2xl font-bold text-[#8b5cf6]">
              {item.value.toLocaleString()}
            </div>
            <div className="mt-1 text-xs text-[#94a3b8]">{item.label}</div>
          </div>
        ))}
      </div>
    </section>
  );
}

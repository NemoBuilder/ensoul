"use client";

import { useTranslations } from "next-intl";
import type { VibeWriteTag } from "@/lib/api";

interface TagCloudFilterProps {
  tags: VibeWriteTag[];
  selectedTagIds: string[];
  onToggleTag: (tagId: string) => void;
  locale?: string;
}

export default function TagCloudFilter({
  tags,
  selectedTagIds,
  onToggleTag,
  locale = "en",
}: TagCloudFilterProps) {
  const t = useTranslations("VibeWrite");

  // Group tags by category
  const grouped: Record<string, VibeWriteTag[]> = {};
  for (const tag of tags) {
    const cat = tag.category || "custom";
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(tag);
  }

  // Sort groups: ecosystem → track → custom
  const categoryOrder = ["ecosystem", "track", "custom"];
  const sortedCategories = Object.keys(grouped).sort(
    (a, b) => (categoryOrder.indexOf(a) ?? 99) - (categoryOrder.indexOf(b) ?? 99)
  );

  const categoryLabels: Record<string, string> = {
    ecosystem: `🔗 ${t("ecosystem")}`,
    track: `🔥 ${t("track")}`,
    custom: `⚙️ ${t("custom")}`,
  };

  function getTagDisplayName(tag: VibeWriteTag) {
    if (locale === "en" && tag.name_en) return tag.name_en;
    return tag.name || tag.name_en || tag.id;
  }

  return (
    <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f]/80 backdrop-blur-sm p-4">
      {sortedCategories.map((cat) => (
        <div key={cat} className="mb-3 last:mb-0">
          <div className="mb-2 text-xs font-medium text-[#64748b] uppercase tracking-wider">
            {categoryLabels[cat] || cat}
          </div>
          <div className="flex flex-wrap gap-2">
            {grouped[cat]
              .sort((a, b) => a.sort_order - b.sort_order)
              .map((tag) => {
                const isSelected = selectedTagIds.includes(tag.id);
                return (
                  <button
                    key={tag.id}
                    onClick={() => onToggleTag(tag.id)}
                    className={`
                      inline-flex items-center gap-1.5 rounded-full px-3 py-1.5
                      text-sm font-medium transition-all duration-150
                      border cursor-pointer select-none
                      ${
                        isSelected
                          ? "border-[#8b5cf6]/60 bg-[#8b5cf6]/15 text-[#c4b5fd] shadow-[0_0_8px_rgba(139,92,246,0.15)]"
                          : "border-[#1e1e2e] bg-[#0a0a0f] text-[#64748b] hover:border-[#334155] hover:text-[#94a3b8]"
                      }
                    `}
                  >
                    <span className="text-base leading-none">{tag.icon}</span>
                    <span>{getTagDisplayName(tag)}</span>
                    {isSelected && (
                      <span className="ml-0.5 text-[10px] text-[#8b5cf6]">✓</span>
                    )}
                  </button>
                );
              })}
          </div>
        </div>
      ))}
    </div>
  );
}

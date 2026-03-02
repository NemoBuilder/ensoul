"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import type { SniperReply, ReplyVariant, SubscriptionStatus } from "@/lib/api";

interface SniperPanelProps {
  reply: SniperReply | null;
  loading: boolean;
  error: string | null;
  subscription: SubscriptionStatus | null;
  onRegenerate: () => void;
  onCollapse: () => void;
  tweetUrl: string;
}

const STYLE_CONFIG: Record<string, { icon: string; labelKey: string }> = {
  insightful: { icon: "💡", labelKey: "insightful" },
  witty: { icon: "😄", labelKey: "witty" },
  supportive: { icon: "💪", labelKey: "supportive" },
};

export default function SniperPanel({
  reply,
  loading,
  error,
  subscription,
  onRegenerate,
  onCollapse,
}: SniperPanelProps) {
  const t = useTranslations("Sniper");
  const [copiedIdx, setCopiedIdx] = useState<number | null>(null);

  async function copyToClipboard(text: string, idx: number) {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedIdx(idx);
      setTimeout(() => setCopiedIdx(null), 2000);
    } catch {
      // fallback
    }
  }

  function openTwitterReply(replyText: string) {
    // Encode the reply text and open Twitter intent
    const tweetId = reply?.tweet_id || "";
    const url = `https://twitter.com/intent/tweet?in_reply_to=${tweetId}&text=${encodeURIComponent(replyText)}`;
    window.open(url, "_blank");
  }

  if (loading) {
    return (
      <div className="p-4 text-center">
        <div className="inline-flex items-center gap-2 text-sm text-[#94a3b8]">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
          {t("loading")}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4">
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">
          {error}
        </div>
        <div className="mt-2 flex justify-end gap-2">
          <button onClick={onRegenerate} className="text-xs text-[#8b5cf6] hover:underline">
            {t("regenerate")}
          </button>
          <button onClick={onCollapse} className="text-xs text-[#64748b] hover:underline">
            {t("collapsePanel")}
          </button>
        </div>
      </div>
    );
  }

  if (!reply) return null;

  const variants: ReplyVariant[] = reply.replies || [];

  return (
    <div className="p-4">
      <div className="mb-3 flex items-center justify-between">
        <h4 className="text-sm font-semibold text-[#e2e8f0]">🎯 {t("snipePanel")}</h4>
        <button
          onClick={onCollapse}
          className="text-xs text-[#64748b] hover:text-[#94a3b8] transition-colors"
        >
          {t("collapsePanel")} ↑
        </button>
      </div>

      <div className="space-y-3">
        {variants.map((variant, idx) => {
          const config = STYLE_CONFIG[variant.style] || { icon: "📝", labelKey: variant.style };
          return (
            <div
              key={idx}
              className="rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-3"
            >
              <div className="mb-2 text-xs font-medium text-[#8b5cf6]">
                {config.icon} {t(config.labelKey as "insightful" | "witty" | "supportive")}
              </div>
              <p className="text-sm leading-relaxed text-[#cbd5e1] whitespace-pre-wrap">
                {variant.content}
              </p>
              <div className="mt-2 flex items-center gap-2">
                <button
                  onClick={() => copyToClipboard(variant.content, idx)}
                  className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] transition-colors"
                >
                  📋 {copiedIdx === idx ? t("copied") : t("copy")}
                </button>
                <button
                  onClick={() => openTwitterReply(variant.content)}
                  className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] transition-colors"
                >
                  ↗ {t("openTwitterReply")}
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {/* Meta info */}
      <div className="mt-3 flex items-center justify-between text-xs text-[#64748b]">
        <div className="flex items-center gap-3">
          {reply.used_soul && reply.author_handle ? (
            <span>🧬 {t("usedSoul", { handle: reply.author_handle })}</span>
          ) : (
            <span>⚙️ {t("usedGeneric")}</span>
          )}
        </div>
        <div className="flex items-center gap-3">
          {subscription && (
            <span>
              📊 {t("todayUsage", {
                used: subscription.daily_snipes ?? 0,
                limit: subscription.daily_limit ?? 50,
              })}
            </span>
          )}
        </div>
      </div>

      {/* Actions */}
      <div className="mt-3 flex items-center gap-3">
        <button
          onClick={onRegenerate}
          className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium
            bg-[#1e1e2e] text-[#94a3b8] hover:bg-[#2a2a3e] transition-colors"
        >
          🔄 {t("regenerate")}
        </button>
      </div>
    </div>
  );
}

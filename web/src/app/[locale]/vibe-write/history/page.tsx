"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { vibeWriteApi, type VibeWriteReply } from "@/lib/api";

function timeAgo(dateStr: string, t: ReturnType<typeof useTranslations>) {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = Math.floor((now - then) / 1000);
  if (diff < 60) return t("justNow");
  if (diff < 3600) return t("minutesAgo", { count: Math.floor(diff / 60) });
  if (diff < 86400) return t("hoursAgo", { count: Math.floor(diff / 3600) });
  return t("daysAgo", { count: Math.floor(diff / 86400) });
}

export default function VibeWriteHistoryPage() {
  const t = useTranslations("VibeWrite");
  const { isConnected } = useAccount();
  const [replies, setReplies] = useState<VibeWriteReply[]>([]);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState<string | null>(null);

  const fetchReplies = useCallback(async () => {
    try {
      const data = await vibeWriteApi.getReplies();
      setReplies(data.replies || []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isConnected) fetchReplies();
    else setLoading(false);
  }, [isConnected, fetchReplies]);

  function copyText(text: string, id: string) {
    navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  }

  if (!isConnected) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16 text-center">
        <p className="mb-4 text-[#94a3b8]">{t("connectToSubscribe")}</p>
        <ConnectButton />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[#e2e8f0]">{t("historyTitle")}</h1>
          <p className="mt-1 text-sm text-[#94a3b8]">{t("historyDesc")}</p>
        </div>
        <Link
          href="/vibe-write"
          className="rounded-lg border border-[#1e1e2e] bg-[#14141f] px-4 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
        >
          {t("backToFeed")}
        </Link>
      </div>

      {/* Loading */}
      {loading && (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      )}

      {/* Empty state */}
      {!loading && replies.length === 0 && (
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <div className="mx-auto mb-4 text-5xl">🎯</div>
          <h2 className="mb-2 text-lg font-semibold text-[#e2e8f0]">{t("noHistoryYet")}</h2>
          <p className="mb-6 text-sm text-[#94a3b8]">{t("noHistoryDesc")}</p>
          <Link
            href="/vibe-write"
            className="inline-block rounded-lg bg-[#8b5cf6] px-6 py-2 text-sm font-medium text-white transition-colors hover:bg-[#a78bfa]"
          >
            {t("backToFeed")}
          </Link>
        </div>
      )}

      {/* Reply list */}
      {!loading && replies.length > 0 && (
        <div className="space-y-4">
          {replies.map((r) => (
            <div
              key={r.id}
              className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5 transition-colors hover:border-[#8b5cf6]/30"
            >
              {/* Tweet info header */}
              <div className="mb-3 flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="mb-1 flex items-center gap-2">
                    <span className="text-sm font-medium text-[#8b5cf6]">
                      @{r.author_handle}
                    </span>
                    {r.used_soul && (
                      <span className="rounded-full bg-[#8b5cf6]/10 px-2 py-0.5 text-xs text-[#8b5cf6]">
                        🧬 Soul
                      </span>
                    )}
                    <span className="text-xs text-[#94a3b8]">
                      {t("snipedAt", { time: timeAgo(r.created_at, t) })}
                    </span>
                  </div>
                  <p className="text-sm leading-relaxed text-[#cbd5e1]">
                    {r.tweet_text}
                  </p>
                </div>
                {r.tweet_url && (
                  <a
                    href={r.tweet_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="shrink-0 rounded-md border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                  >
                    {t("viewOnTwitter")}
                  </a>
                )}
              </div>

              {/* Generated replies */}
              {r.replies && r.replies.length > 0 && (
                <div className="space-y-2 border-t border-[#1e1e2e] pt-3">
                  {r.replies.map((v, i) => (
                    <div
                      key={i}
                      className="group flex items-start gap-3 rounded-lg bg-[#0a0a0f] p-3"
                    >
                      <span className="shrink-0 rounded-md bg-[#8b5cf6]/10 px-2 py-1 text-xs font-medium text-[#8b5cf6]">
                        {v.style === "insightful"
                          ? t("insightful")
                          : v.style === "witty"
                            ? t("witty")
                            : t("supportive")}
                      </span>
                      <p className="min-w-0 flex-1 text-sm leading-relaxed text-[#e2e8f0]">
                        {v.content}
                      </p>
                      <button
                        onClick={() => copyText(v.content, `${r.id}-${i}`)}
                        className="shrink-0 rounded-md border border-[#1e1e2e] px-2 py-1 text-xs text-[#94a3b8] opacity-0 transition-all hover:border-[#8b5cf6] hover:text-[#e2e8f0] group-hover:opacity-100"
                      >
                        {copied === `${r.id}-${i}` ? t("copied") : t("copy")}
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

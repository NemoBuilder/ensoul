"use client";

import { useState, useRef, useEffect } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import type { TweetCard as TweetCardType, SniperTag } from "@/lib/api";

interface TweetCardProps {
  tweet: TweetCardType;
  tags: SniperTag[];
  onSnipe: (tweet: TweetCardType) => void;
  onMute: (handle: string) => void;
  isSnipeActive: boolean;
  children?: React.ReactNode; // SniperPanel slot
}

function formatTimeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = Math.floor((now - then) / 1000);

  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

export default function TweetCard({
  tweet,
  tags,
  onSnipe,
  onMute,
  isSnipeActive,
  children,
}: TweetCardProps) {
  const t = useTranslations("Sniper");
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    if (menuOpen) document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [menuOpen]);

  // Find the first matching tag for badge display
  const primaryTag = tags.find((tg) => tweet.tags.includes(tg.id));

  return (
    <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] transition-colors hover:border-[#2a2a3e]">
      <div className="p-4">
        {/* Tag badge row */}
        {primaryTag && (
          <div className="mb-2 flex items-center gap-2">
            <span className="text-base">{primaryTag.icon}</span>
            <span className="text-xs font-medium text-[#64748b]">
              {primaryTag.name_en || primaryTag.name}
            </span>
          </div>
        )}

        {/* Author row */}
        <div className="flex items-center gap-3">
          {tweet.author.avatar ? (
            <Image
              src={tweet.author.avatar}
              alt={tweet.author.handle}
              width={40}
              height={40}
              className="h-10 w-10 rounded-full bg-[#1e1e2e]"
              unoptimized
            />
          ) : (
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[#1e1e2e] text-[#64748b] text-sm font-bold">
              {tweet.author.name?.[0] || "@"}
            </div>
          )}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="font-semibold text-[#e2e8f0] truncate">
                {tweet.author.name}
              </span>
              {tweet.author.verified && (
                <svg className="h-4 w-4 text-[#1d9bf0] flex-shrink-0" viewBox="0 0 22 22" fill="currentColor">
                  <path d="M20.396 11c-.018-.646-.215-1.275-.57-1.816-.354-.54-.852-.972-1.438-1.246.223-.607.27-1.264.14-1.897-.131-.634-.437-1.218-.882-1.687-.47-.445-1.053-.75-1.687-.882-.633-.13-1.29-.083-1.897.14-.273-.587-.704-1.086-1.245-1.44S11.647 1.62 11 1.604c-.646.017-1.273.213-1.813.568s-.969.853-1.24 1.44c-.608-.223-1.267-.272-1.902-.14-.635.13-1.22.436-1.69.882-.445.47-.749 1.055-.878 1.69-.13.633-.08 1.29.144 1.896-.587.274-1.087.705-1.443 1.245-.356.54-.555 1.17-.574 1.817.02.647.218 1.276.574 1.817.356.54.856.972 1.443 1.245-.224.606-.274 1.263-.144 1.896.13.636.433 1.221.878 1.69.47.446 1.055.752 1.69.883.635.13 1.294.083 1.902-.143.272.587.702 1.086 1.24 1.44.54.354 1.167.551 1.813.568.647-.016 1.276-.213 1.817-.567s.972-.854 1.245-1.44c.604.225 1.261.272 1.894.143.636-.13 1.22-.434 1.69-.88.445-.47.749-1.055.88-1.69.13-.634.085-1.29-.138-1.897.586-.274 1.084-.705 1.439-1.246.354-.54.551-1.17.569-1.816zM9.662 14.85l-3.429-3.428 1.293-1.302 2.072 2.072 4.4-4.794 1.347 1.246z" />
                </svg>
              )}
              {tweet.has_soul && (
                <span className="text-sm flex-shrink-0" title={t("soulBadge")}>🧬</span>
              )}
            </div>
            <div className="flex items-center gap-1.5 text-xs text-[#64748b]">
              <span>@{tweet.author.handle}</span>
              <span>·</span>
              <span>{formatTimeAgo(tweet.created_at)}</span>
            </div>
          </div>
          {/* Open original link */}
          <a
            href={tweet.tweet_url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex-shrink-0 text-[#64748b] hover:text-[#94a3b8] transition-colors"
            title={t("openOriginal")}
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M14 3h7m0 0v7m0-7L10 14" />
            </svg>
          </a>
        </div>

        {/* Tweet text */}
        <div className="mt-3 text-sm leading-relaxed text-[#cbd5e1] whitespace-pre-wrap break-words">
          {tweet.text}
        </div>

        {/* Stats */}
        <div className="mt-3 flex items-center gap-5 text-xs text-[#64748b]">
          <span className="flex items-center gap-1">
            💬 {formatCount(tweet.stats.replies)}
          </span>
          <span className="flex items-center gap-1">
            🔁 {formatCount(tweet.stats.retweets)}
          </span>
          <span className="flex items-center gap-1">
            ❤️ {formatCount(tweet.stats.likes)}
          </span>
          {tweet.stats.views > 0 && (
            <span className="flex items-center gap-1">
              👁 {formatCount(tweet.stats.views)}
            </span>
          )}
        </div>

        {/* Action bar */}
        <div className="mt-3 flex items-center gap-2 border-t border-[#1e1e2e] pt-3">
          <button
            onClick={() => onSnipe(tweet)}
            className={`
              inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium
              transition-all duration-150
              ${
                isSnipeActive
                  ? "bg-[#8b5cf6] text-white"
                  : "bg-[#8b5cf6]/10 text-[#c4b5fd] hover:bg-[#8b5cf6]/20"
              }
            `}
          >
            🎯 {t("snipe")}
          </button>

          <a
            href={tweet.tweet_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium
              bg-[#1e1e2e] text-[#94a3b8] hover:bg-[#2a2a3e] transition-colors"
          >
            🔗 {t("openOriginal")}
          </a>

          {/* More menu */}
          <div className="relative ml-auto" ref={menuRef}>
            <button
              onClick={() => setMenuOpen(!menuOpen)}
              className="rounded-lg p-1.5 text-[#64748b] hover:bg-[#1e1e2e] hover:text-[#94a3b8] transition-colors"
            >
              <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                <path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" />
              </svg>
            </button>
            {menuOpen && (
              <div className="absolute right-0 top-full mt-1 z-10 w-48 rounded-lg border border-[#1e1e2e] bg-[#14141f] shadow-xl">
                <button
                  onClick={() => {
                    onMute(tweet.author.handle);
                    setMenuOpen(false);
                  }}
                  className="w-full px-4 py-2.5 text-left text-sm text-[#f87171] hover:bg-[#1e1e2e] rounded-lg transition-colors"
                >
                  🔇 {t("muteAccount", { handle: tweet.author.handle })}
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Snipe panel (inline expanded) */}
      {children && (
        <div className="border-t border-[#1e1e2e]">
          {children}
        </div>
      )}
    </div>
  );
}

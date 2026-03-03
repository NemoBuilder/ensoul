"use client";

import { useTranslations } from "next-intl";
import type { TweetCard as TweetCardType, SniperTag, SniperReply, SubscriptionStatus } from "@/lib/api";
import TweetCard from "./TweetCard";
import SniperPanel from "./SniperPanel";

interface TweetFeedProps {
  tweets: TweetCardType[];
  tags: SniperTag[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  onLoadMore: () => void;
  // Snipe state
  activeSnipeTweetId: string | null;
  snipeLoading: boolean;
  snipeError: string | null;
  snipeResult: SniperReply | null;
  subscription: SubscriptionStatus | null;
  onSnipe: (tweet: TweetCardType) => void;
  onRegenerate: () => void;
  onCollapseSnipe: () => void;
  onMute: (handle: string) => void;
}

export default function TweetFeed({
  tweets,
  tags,
  loading,
  loadingMore,
  hasMore,
  onLoadMore,
  activeSnipeTweetId,
  snipeLoading,
  snipeError,
  snipeResult,
  subscription,
  onSnipe,
  onRegenerate,
  onCollapseSnipe,
  onMute,
}: TweetFeedProps) {
  const t = useTranslations("Sniper");

  if (loading) {
    return (
      <div className="space-y-3">
        {/* Skeleton tweet cards */}
        {Array.from({ length: 4 }).map((_, i) => (
          <div
            key={i}
            className="rounded-xl border border-[#1e1e2e] bg-[#12121a] p-4 animate-pulse"
          >
            {/* Author row */}
            <div className="flex items-center gap-3 mb-3">
              <div className="h-10 w-10 rounded-full bg-[#1e1e2e]" />
              <div className="flex-1">
                <div className="h-4 w-28 rounded bg-[#1e1e2e] mb-1.5" />
                <div className="h-3 w-20 rounded bg-[#1e1e2e]" />
              </div>
              <div className="h-3 w-16 rounded bg-[#1e1e2e]" />
            </div>
            {/* Text lines */}
            <div className="space-y-2 mb-3">
              <div className="h-3.5 w-full rounded bg-[#1e1e2e]" />
              <div className="h-3.5 w-5/6 rounded bg-[#1e1e2e]" />
              <div className="h-3.5 w-3/4 rounded bg-[#1e1e2e]" />
            </div>
            {/* Stats row */}
            <div className="flex items-center gap-6">
              <div className="h-3 w-12 rounded bg-[#1e1e2e]" />
              <div className="h-3 w-12 rounded bg-[#1e1e2e]" />
              <div className="h-3 w-12 rounded bg-[#1e1e2e]" />
              <div className="h-3 w-12 rounded bg-[#1e1e2e]" />
            </div>
          </div>
        ))}
        <p className="text-center text-xs text-[#4a4a5a] pt-2">{t("loadingFeed")}</p>
      </div>
    );
  }

  if (tweets.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <div className="text-4xl mb-3">📡</div>
        <h3 className="text-lg font-semibold text-[#e2e8f0]">{t("noTweets")}</h3>
        <p className="mt-1 text-sm text-[#64748b]">{t("noTweetsDesc")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {tweets.map((tweet) => {
        const isSnipeActive = activeSnipeTweetId === tweet.id;
        return (
          <TweetCard
            key={tweet.id}
            tweet={tweet}
            tags={tags}
            onSnipe={onSnipe}
            onMute={onMute}
            isSnipeActive={isSnipeActive}
          >
            {isSnipeActive && (
              <SniperPanel
                reply={snipeResult}
                loading={snipeLoading}
                error={snipeError}
                subscription={subscription}
                onRegenerate={onRegenerate}
                onCollapse={onCollapseSnipe}
                tweetUrl={tweet.tweet_url}
              />
            )}
          </TweetCard>
        );
      })}

      {/* Load more */}
      {hasMore && (
        <div className="flex justify-center py-4">
          <button
            onClick={onLoadMore}
            disabled={loadingMore}
            className="rounded-lg px-6 py-2 text-sm font-medium text-[#94a3b8]
              border border-[#1e1e2e] hover:border-[#334155] hover:text-[#e2e8f0]
              disabled:opacity-50 transition-colors"
          >
            {loadingMore ? t("loadingMore") : t("loadMore")}
          </button>
        </div>
      )}
    </div>
  );
}

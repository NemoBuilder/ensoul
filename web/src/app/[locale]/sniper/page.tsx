"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslations } from "next-intl";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { Link } from "@/i18n/navigation";
import {
  sniperApi,
  type SniperTag,
  type TweetCard as TweetCardType,
  type SniperReply,
  type SubscriptionStatus,
} from "@/lib/api";

import TagCloudFilter from "@/components/sniper/TagCloudFilter";
import TweetFeed from "@/components/sniper/TweetFeed";
import SubscribeModal from "@/components/sniper/SubscribeModal";
import { useSniperSSE, type SSEStatus } from "@/components/sniper/useSniperSSE";

export default function SniperPage() {
  const t = useTranslations("Sniper");
  const { isConnected } = useAccount();

  // Tags
  const [tags, setTags] = useState<SniperTag[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);
  const [tagsLoaded, setTagsLoaded] = useState(false);

  // Feed
  const [tweets, setTweets] = useState<TweetCardType[]>([]);
  const [feedLoading, setFeedLoading] = useState(false);
  const [feedLoadingMore, setFeedLoadingMore] = useState(false);
  const [cursor, setCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);

  // SSE new tweets
  const [newTweets, setNewTweets] = useState<TweetCardType[]>([]);

  // Muted
  const [mutedHandles, setMutedHandles] = useState<Set<string>>(new Set());

  // Subscription
  const [subscription, setSubscription] = useState<SubscriptionStatus | null>(null);
  const [showSubscribeModal, setShowSubscribeModal] = useState(false);

  // Snipe state
  const [activeSnipeTweetId, setActiveSnipeTweetId] = useState<string | null>(null);
  const [snipeLoading, setSnipeLoading] = useState(false);
  const [snipeError, setSnipeError] = useState<string | null>(null);
  const [snipeResult, setSnipeResult] = useState<SniperReply | null>(null);
  const activeTweetRef = useRef<TweetCardType | null>(null);

  // ── Init: load tags, user prefs, subscription ──────────────────────
  useEffect(() => {
    async function init() {
      try {
        // Always fetch tags first (needed for defaults)
        const tagData = await sniperApi.getTags();
        setTags(tagData.tags || []);

        if (isConnected) {
          // Fetch user prefs, subscription, and muted list in parallel
          const [userTagsRes, subRes, mutedRes] = await Promise.allSettled([
            sniperApi.getUserTags(),
            sniperApi.getSubscription(),
            sniperApi.getMuted(),
          ]);

          // User tag preferences
          let userTagIds: string[] | null = null;
          if (userTagsRes.status === "fulfilled") {
            const val = userTagsRes.value;
            if (val.tag_ids && val.tag_ids.length > 0) {
              userTagIds = val.tag_ids;
            }
          }

          // Subscription
          if (subRes.status === "fulfilled") {
            setSubscription(subRes.value);
          }

          // Muted handles
          if (mutedRes.status === "fulfilled") {
            setMutedHandles(new Set(mutedRes.value.handles || []));
          }

          setSelectedTagIds(userTagIds || tagData.defaults || []);
        } else {
          setSelectedTagIds(tagData.defaults || []);
        }

        setTagsLoaded(true);
      } catch {
        // Tag loading failed
        setTagsLoaded(true);
      }
    }
    init();
  }, [isConnected]);

  // ── Load feed when selected tags change ───────────────────────────
  const loadFeed = useCallback(
    async (tagIds: string[], append = false, pageCursor?: string) => {
      if (tagIds.length === 0) {
        if (!append) {
          setTweets([]);
          setCursor(null);
          setHasMore(false);
        }
        return;
      }

      if (append) {
        setFeedLoadingMore(true);
      } else {
        setFeedLoading(true);
      }

      try {
        const result = await sniperApi.getFeed(tagIds, pageCursor);
        const filtered = (result.tweets || []).filter(
          (tw) => !mutedHandles.has(tw.author.handle)
        );

        if (append) {
          setTweets((prev) => {
            const existingIds = new Set(prev.map((t) => t.id));
            const newOnly = filtered.filter((t) => !existingIds.has(t.id));
            return [...prev, ...newOnly];
          });
        } else {
          setTweets(filtered);
        }

        setCursor(result.next_cursor || null);
        setHasMore(!!result.next_cursor);
      } catch {
        // Feed load failed
      } finally {
        setFeedLoading(false);
        setFeedLoadingMore(false);
      }
    },
    [mutedHandles]
  );

  useEffect(() => {
    if (tagsLoaded && selectedTagIds.length > 0) {
      loadFeed(selectedTagIds);
      setNewTweets([]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTagIds, tagsLoaded]);

  // ── SSE ─────────────────────────────────────────────────────────────
  const handleNewTweets = useCallback(
    (_tagId: string, incoming: TweetCardType[]) => {
      const filtered = incoming.filter(
        (tw) => !mutedHandles.has(tw.author.handle)
      );
      if (filtered.length === 0) return;
      setNewTweets((prev) => {
        const existingIds = new Set([...prev.map((t) => t.id), ...tweets.map((t) => t.id)]);
        const newOnly = filtered.filter((t) => !existingIds.has(t.id));
        return [...newOnly, ...prev];
      });
    },
    [mutedHandles, tweets]
  );

  const { status: sseStatus } = useSniperSSE({
    tagIds: selectedTagIds,
    enabled: tagsLoaded && selectedTagIds.length > 0,
    onNewTweets: handleNewTweets,
  });

  // ── Handlers ───────────────────────────────────────────────────────
  function handleToggleTag(tagId: string) {
    setSelectedTagIds((prev) => {
      const next = prev.includes(tagId)
        ? prev.filter((id) => id !== tagId)
        : [...prev, tagId];

      if (isConnected) {
        sniperApi.updateUserTags(next).catch(() => {});
      }
      return next;
    });
  }

  function handleFlushNewTweets() {
    setTweets((prev) => {
      const existingIds = new Set(prev.map((t) => t.id));
      const toAdd = newTweets.filter((t) => !existingIds.has(t.id));
      return [...toAdd, ...prev];
    });
    setNewTweets([]);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function handleLoadMore() {
    if (cursor) {
      loadFeed(selectedTagIds, true, cursor);
    }
  }

  async function handleSnipe(tweet: TweetCardType) {
    // Toggle off if same tweet
    if (activeSnipeTweetId === tweet.id) {
      setActiveSnipeTweetId(null);
      setSnipeResult(null);
      setSnipeError(null);
      return;
    }

    // Check auth
    if (!isConnected) {
      setActiveSnipeTweetId(tweet.id);
      setSnipeError(t("loginToSnipe"));
      activeTweetRef.current = tweet;
      return;
    }

    // Check subscription
    if (!subscription?.active || subscription?.tier === "free") {
      activeTweetRef.current = tweet;
      setShowSubscribeModal(true);
      return;
    }

    // Do snipe
    activeTweetRef.current = tweet;
    setActiveSnipeTweetId(tweet.id);
    setSnipeLoading(true);
    setSnipeError(null);
    setSnipeResult(null);

    try {
      const result = await sniperApi.snipe(
        tweet.id,
        tweet.text,
        tweet.author.handle,
        tweet.tags[0] || ""
      );
      setSnipeResult(result);
      // Refresh subscription to update daily count
      try {
        const sub = await sniperApi.getSubscription();
        setSubscription(sub);
      } catch {}
    } catch (err: unknown) {
      setSnipeError(err instanceof Error ? err.message : "Snipe failed");
    } finally {
      setSnipeLoading(false);
    }
  }

  function handleRegenerate() {
    if (activeTweetRef.current) {
      handleSnipe(activeTweetRef.current);
    }
  }

  function handleCollapseSnipe() {
    setActiveSnipeTweetId(null);
    setSnipeResult(null);
    setSnipeError(null);
    activeTweetRef.current = null;
  }

  async function handleMute(handle: string) {
    setMutedHandles((prev) => new Set([...prev, handle]));
    setTweets((prev) => prev.filter((t) => t.author.handle !== handle));
    setNewTweets((prev) => prev.filter((t) => t.author.handle !== handle));

    if (isConnected) {
      try {
        await sniperApi.muteAccount(handle);
      } catch {
        setMutedHandles((prev) => {
          const next = new Set(prev);
          next.delete(handle);
          return next;
        });
      }
    }
  }

  function handleSubscribeSuccess() {
    setShowSubscribeModal(false);
    sniperApi.getSubscription().then(setSubscription).catch(() => {});
    if (activeTweetRef.current) {
      handleSnipe(activeTweetRef.current);
    }
  }

  // ── SSE status badge ──────────────────────────────────────────────
  function SSEBadge({ status }: { status: SSEStatus }) {
    const colors: Record<SSEStatus, string> = {
      connected: "bg-green-500",
      reconnecting: "bg-yellow-500 animate-pulse",
      disconnected: "bg-red-500",
    };
    const labels: Record<SSEStatus, string> = {
      connected: t("sseConnected"),
      reconnecting: t("sseReconnecting"),
      disconnected: t("sseDisconnected"),
    };
    return (
      <div className="flex items-center gap-1.5">
        <div className={`h-2 w-2 rounded-full ${colors[status]}`} />
        <span className="text-xs text-[#64748b]">{labels[status]}</span>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-4 pt-20 pb-16">
      {/* Header */}
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[#e2e8f0]">{t("title")}</h1>
          <p className="mt-1 text-sm text-[#64748b]">{t("subtitle")}</p>
        </div>
        <div className="flex items-center gap-3">
          {isConnected && (
            <Link
              href="/sniper/settings"
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs font-medium text-[#94a3b8]
                hover:border-[#334155] hover:text-[#e2e8f0] transition-colors"
            >
              ⚙️ {t("settings")}
            </Link>
          )}
          {!isConnected && <ConnectButton />}
        </div>
      </div>

      {/* SSE status + subscription badge */}
      <div className="mb-4 flex items-center justify-between">
        <SSEBadge status={sseStatus} />
        <div className="flex items-center gap-3">
          {subscription?.active ? (
            <span className="rounded-full bg-[#8b5cf6]/10 px-3 py-1 text-xs font-medium text-[#c4b5fd]">
              {t("proPlan")} · {t("todayUsage", {
                used: subscription.daily_snipes ?? 0,
                limit: subscription.daily_limit ?? 50,
              })}
            </span>
          ) : isConnected ? (
            <span className="rounded-full bg-[#1e1e2e] px-3 py-1 text-xs font-medium text-[#64748b]">
              {t("freePlan")} · {t("feedOnly")}
            </span>
          ) : null}
        </div>
      </div>

      {/* Tag cloud filter */}
      <div className="mb-4">
        <TagCloudFilter
          tags={tags}
          selectedTagIds={selectedTagIds}
          onToggleTag={handleToggleTag}
        />
      </div>

      {/* New tweets banner */}
      {newTweets.length > 0 && (
        <button
          onClick={handleFlushNewTweets}
          className="mb-3 w-full rounded-lg bg-[#8b5cf6]/10 border border-[#8b5cf6]/30
            py-2.5 text-sm font-medium text-[#c4b5fd] hover:bg-[#8b5cf6]/15 transition-colors"
        >
          {t("newTweets", { count: newTweets.length })}
        </button>
      )}

      {/* Tweet feed */}
      <TweetFeed
        tweets={tweets}
        tags={tags}
        loading={feedLoading}
        loadingMore={feedLoadingMore}
        hasMore={hasMore}
        onLoadMore={handleLoadMore}
        activeSnipeTweetId={activeSnipeTweetId}
        snipeLoading={snipeLoading}
        snipeError={snipeError}
        snipeResult={snipeResult}
        subscription={subscription}
        onSnipe={handleSnipe}
        onRegenerate={handleRegenerate}
        onCollapseSnipe={handleCollapseSnipe}
        onMute={handleMute}
      />

      {/* Subscribe modal */}
      <SubscribeModal
        open={showSubscribeModal}
        onClose={() => setShowSubscribeModal(false)}
        onSuccess={handleSubscribeSuccess}
      />
    </div>
  );
}

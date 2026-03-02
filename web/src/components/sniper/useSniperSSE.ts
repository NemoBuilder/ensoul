"use client";

import { useEffect, useRef, useCallback, useState, useMemo } from "react";
import type { TweetCard } from "@/lib/api";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8990";

export type SSEStatus = "connected" | "reconnecting" | "disconnected";

interface UseSSEOptions {
  tagIds: string[];
  enabled: boolean;
  onNewTweets: (tagId: string, tweets: TweetCard[]) => void;
}

export function useSniperSSE({ tagIds, enabled, onNewTweets }: UseSSEOptions) {
  const [innerStatus, setInnerStatus] = useState<SSEStatus>("disconnected");
  const [lastHeartbeat, setLastHeartbeat] = useState<number>(0);
  const esRef = useRef<EventSource | null>(null);
  const onNewTweetsRef = useRef(onNewTweets);
  useEffect(() => {
    onNewTweetsRef.current = onNewTweets;
  }, [onNewTweets]);

  const connect = useCallback(() => {
    if (tagIds.length === 0) return;

    // Close existing connection
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }

    const url = `${API_BASE}/api/sniper/feed/stream?tag_ids=${tagIds.join(",")}`;
    const es = new EventSource(url);
    esRef.current = es;

    es.onopen = () => {
      setInnerStatus("connected");
    };

    es.addEventListener("new_tweets", (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.tag_id && data.tweets) {
          onNewTweetsRef.current(data.tag_id, data.tweets);
        }
      } catch {
        // Ignore parse errors
      }
    });

    es.addEventListener("heartbeat", (e) => {
      try {
        const data = JSON.parse(e.data);
        setLastHeartbeat(data.ts || Date.now() / 1000);
      } catch {
        setLastHeartbeat(Date.now() / 1000);
      }
    });

    es.onerror = () => {
      setInnerStatus("reconnecting");
      // EventSource has built-in auto-reconnect
    };
  }, [tagIds]);

  useEffect(() => {
    if (!enabled || tagIds.length === 0) {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      return;
    }

    connect();

    return () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };
  }, [enabled, connect, tagIds]);

  // Derive effective status: if SSE is disabled, always report disconnected
  const isActive = enabled && tagIds.length > 0;
  const status: SSEStatus = useMemo(
    () => (isActive ? innerStatus : "disconnected"),
    [isActive, innerStatus]
  );

  return { status, lastHeartbeat };
}

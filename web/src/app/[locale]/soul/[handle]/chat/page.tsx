"use client";

import { useState, useRef, useEffect, use, useCallback } from "react";
import { Link } from "@/i18n/navigation";
import {
  chatApi,
  shellApi,
  sessionApi,
  Shell,
  ChatSession,
  ChatSessionMessage,
} from "@/lib/api";
import { stageConfig, Stage } from "@/lib/utils";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface DisplayMessage {
  role: "user" | "assistant";
  content: string;
}

const GUEST_MAX_ROUNDS = 5;

export default function ChatPage({
  params,
}: {
  params: Promise<{ handle: string }>;
}) {
  const { handle } = use(params);
  const [shell, setShell] = useState<Shell | null>(null);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");

  // Session state
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [tier, setTier] = useState<"guest" | "free" | "paid">("guest");
  const [rounds, setRounds] = useState(0);
  const [walletAddr, setWalletAddr] = useState<string | null>(null);
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [initLoading, setInitLoading] = useState(true);

  // Session history sidebar (logged-in users)
  const [sessionHistory, setSessionHistory] = useState<ChatSession[]>([]);
  const [showHistory, setShowHistory] = useState(true);

  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const loginChecked = useRef(false);

  // Check login state, then initialize chat session
  useEffect(() => {
    let cancelled = false;

    async function init() {
      // Step 1: Check login state
      let loggedIn = false;
      try {
        const res = await sessionApi.session();
        if (!cancelled) {
          setWalletAddr(res.address);
          setIsLoggedIn(true);
          loggedIn = true;
        }
      } catch {
        if (!cancelled) {
          setWalletAddr(null);
          setIsLoggedIn(false);
        }
      }

      if (cancelled) return;
      loginChecked.current = true;

      // Step 2: If logged in, try to resume the most recent session
      if (loggedIn) {
        try {
          const histRes = await chatApi.listSessions(handle);
          const sessions = histRes.sessions || [];
          if (!cancelled) setSessionHistory(sessions);

          if (!cancelled && sessions.length > 0) {
            // Resume the most recent session (already sorted by updated_at DESC)
            const latest = sessions[0];
            const full = await chatApi.getSession(latest.id);
            if (!cancelled) {
              setSessionId(full.id);
              setTier(full.tier as "guest" | "free" | "paid");
              setRounds(full.rounds);
              const msgs: DisplayMessage[] = (full.messages || []).map(
                (m: ChatSessionMessage) => ({
                  role: m.role as "user" | "assistant",
                  content: m.content,
                })
              );
              setMessages(msgs);
              setInitLoading(false);
              return;
            }
          }
        } catch {
          // Fall through to create new session
        }
      }

      // Step 3: No existing session found (or guest) — create a new one
      if (!cancelled) {
        try {
          const res = await chatApi.createSession(handle);
          if (!cancelled) {
            setSessionId(res.session_id);
            setTier(res.tier as "guest" | "free" | "paid");
            setRounds(0);
          }
        } catch (err: unknown) {
          if (!cancelled) {
            setError(
              err instanceof Error
                ? err.message
                : "Failed to create chat session"
            );
          }
        } finally {
          if (!cancelled) setInitLoading(false);
        }
      }
    }

    init();
    return () => {
      cancelled = true;
    };
  }, [handle]);

  // Load shell info
  useEffect(() => {
    shellApi.get(handle).then(setShell).catch(() => {});
  }, [handle]);

  // Reload session history (for sidebar refresh after new session etc.)
  const loadHistory = useCallback(async () => {
    if (!isLoggedIn) return;
    try {
      const res = await chatApi.listSessions(handle);
      setSessionHistory(res.sessions || []);
    } catch {
      // Silently fail
    }
  }, [isLoggedIn, handle]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  // Resume an existing session
  async function resumeSession(id: string) {
    try {
      const session = await chatApi.getSession(id);
      setSessionId(session.id);
      setTier(session.tier as "guest" | "free" | "paid");
      setRounds(session.rounds);
      // Load messages
      const msgs: DisplayMessage[] = (session.messages || []).map(
        (m: ChatSessionMessage) => ({
          role: m.role,
          content: m.content,
        })
      );
      setMessages(msgs);
      setShowHistory(false);
      setError("");
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to load session"
      );
    }
  }

  // Delete a session
  async function deleteSession(id: string) {
    try {
      await chatApi.deleteSession(id);
      setSessionHistory((prev) => prev.filter((s) => s.id !== id));
      // If deleting current session, start a new one
      if (id === sessionId) {
        startNewSession();
      }
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to delete session"
      );
    }
  }

  // Start a brand new session
  async function startNewSession() {
    try {
      setMessages([]);
      setError("");
      const res = await chatApi.createSession(handle);
      setSessionId(res.session_id);
      setTier(res.tier as "guest" | "free" | "paid");
      setRounds(0);
      loadHistory();
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to create session"
      );
    }
  }

  // Send message with SSE streaming
  async function sendMessage() {
    const text = input.trim();
    if (!text || streaming || !sessionId) return;

    // Check guest round limit on frontend too
    if (tier === "guest" && rounds >= GUEST_MAX_ROUNDS) {
      setError(
        `Guest limit reached (${GUEST_MAX_ROUNDS} rounds). Connect wallet & sign in for unlimited chat!`
      );
      return;
    }

    setInput("");
    setError("");
    setStreaming(true);

    // Add user message
    const userMsg: DisplayMessage = { role: "user", content: text };
    setMessages((prev) => [...prev, userMsg]);

    // Add empty assistant message to stream into
    setMessages((prev) => [...prev, { role: "assistant", content: "" }]);

    try {
      const res = await chatApi.sendMessage(sessionId, text);

      if (!res.ok) {
        const errData = await res
          .json()
          .catch(() => ({ error: "Chat failed" }));
        throw new Error(errData.error || `HTTP ${res.status}`);
      }

      const reader = res.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) throw new Error("No response stream");

      setRounds((prev) => prev + 1);

      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (
            line.startsWith("event:done") ||
            line.startsWith("event: done")
          ) {
            break;
          }
          if (line.startsWith("data:")) {
            const raw = line.startsWith("data: ")
              ? line.slice(6)
              : line.slice(5);
            if (raw === "[DONE]" || raw === "") continue;
            // JSON-decode the SSE data to restore newlines
            let data: string;
            try {
              data = JSON.parse(raw);
            } catch {
              data = raw;
            }
            // Append chunk to last assistant message
            setMessages((prev) => {
              const updated = [...prev];
              const last = updated[updated.length - 1];
              if (last && last.role === "assistant") {
                updated[updated.length - 1] = {
                  ...last,
                  content: last.content + data,
                };
              }
              return updated;
            });
          }
        }
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Chat failed");
      // Remove empty assistant message on error
      setMessages((prev) => {
        const last = prev[prev.length - 1];
        if (last && last.role === "assistant" && !last.content) {
          return prev.slice(0, -1);
        }
        return prev;
      });
    } finally {
      setStreaming(false);
    }
  }

  // Auto-focus textarea when streaming ends or input clears
  useEffect(() => {
    if (!streaming && inputRef.current) {
      inputRef.current.focus();
      // Reset textarea height after send
      inputRef.current.style.height = "auto";
    }
  }, [streaming]);

  const stage = shell
    ? stageConfig[(shell.stage as Stage)] || stageConfig.embryo
    : stageConfig.embryo;

  const guestLimitReached = tier === "guest" && rounds >= GUEST_MAX_ROUNDS;

  return (
    <div className="flex h-screen pt-16">
      {/* Session history sidebar — logged-in users only */}
      {isLoggedIn && showHistory && (
        <div className="flex w-72 flex-col border-r border-[#1e1e2e] bg-[#0a0a0f]">
          <div className="flex items-center justify-between border-b border-[#1e1e2e] px-4 py-3">
            <span className="text-sm font-medium text-[#e2e8f0]">
              Chat History
            </span>
            <button
              onClick={startNewSession}
              className="rounded bg-[#8b5cf6] px-2 py-1 text-xs text-white hover:bg-[#a78bfa]"
            >
              + New
            </button>
          </div>
          <div className="flex-1 overflow-y-auto">
            {sessionHistory.length === 0 ? (
              <p className="px-4 py-6 text-center text-xs text-[#94a3b8]">
                No previous chats
              </p>
            ) : (
              sessionHistory.map((s) => (
                <div
                  key={s.id}
                  className={`group flex cursor-pointer items-center gap-2 border-b border-[#1e1e2e] px-4 py-3 hover:bg-[#14141f] ${
                    s.id === sessionId ? "bg-[#14141f]" : ""
                  }`}
                  onClick={() => resumeSession(s.id)}
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-[#e2e8f0]">
                      {s.title || "Untitled"}
                    </p>
                    <p className="text-xs text-[#94a3b8]">
                      {s.rounds} rounds · {s.tier}
                    </p>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      deleteSession(s.id);
                    }}
                    className="hidden text-xs text-red-400 hover:text-red-300 group-hover:block"
                  >
                    ✕
                  </button>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* Main chat area */}
      <div className="flex flex-1 flex-col">
        {/* Chat header */}
        <div className="border-b border-[#1e1e2e] bg-[#14141f] px-4 py-3">
          <div className="mx-auto flex max-w-3xl items-center justify-between">
            <div className="flex items-center gap-3">
              {isLoggedIn && (
                <button
                  onClick={() => setShowHistory(!showHistory)}
                  className="text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
                  title="Chat history"
                >
                  ☰
                </button>
              )}
              <Link
                href={`/soul/${handle}`}
                className="text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
              >
                ←
              </Link>
              <div>
                <h2 className="font-medium text-[#e2e8f0]">
                  Chat with @{handle}
                </h2>
                {shell && (
                  <span className={`text-xs ${stage.textClass}`}>
                    {stage.label} · DNA v{shell.dna_version}
                  </span>
                )}
              </div>
            </div>
            <div className="flex items-center gap-3">
              {/* Round counter */}
              {tier === "guest" && (
                <span className="rounded-full bg-[#1e1e2e] px-2 py-0.5 text-xs text-[#f59e0b]">
                  {rounds}/{GUEST_MAX_ROUNDS} rounds
                </span>
              )}
              {tier === "free" && (
                <span className="rounded-full bg-[#1e1e2e] px-2 py-0.5 text-xs text-[#10b981]">
                  ∞ unlimited
                </span>
              )}
              <div className="flex items-center gap-2">
                <div
                  className={`h-2 w-2 rounded-full ${
                    shell?.stage === "embryo"
                      ? "bg-gray-500"
                      : "bg-green-500"
                  }`}
                />
                <span className="text-xs text-[#94a3b8]">
                  {shell?.stage === "embryo" ? "Embryo" : "Online"}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Messages area */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-6">
          <div className="mx-auto max-w-3xl space-y-6">
            {initLoading ? (
              <div className="py-20 text-center">
                <div className="mb-4 text-4xl animate-pulse">⏳</div>
                <p className="text-sm text-[#94a3b8]">
                  Connecting to @{handle}&apos;s soul...
                </p>
              </div>
            ) : messages.length === 0 ? (
              <div className="py-20 text-center">
                <div className="mb-4 text-4xl">💬</div>
                <h3 className="mb-2 text-lg font-medium text-[#e2e8f0]">
                  Start a Conversation
                </h3>
                <p className="text-sm text-[#94a3b8]">
                  {shell?.stage === "embryo"
                    ? "This soul is still an embryo. Conversations may be limited."
                    : `Talk to @${handle}'s digital soul. It responds based on collected personality fragments.`}
                </p>
                {tier === "guest" && (
                  <p className="mt-3 text-xs text-[#f59e0b]">
                    🔓 Guest mode: {GUEST_MAX_ROUNDS} free rounds. Sign in for
                    unlimited chat & saved history.
                  </p>
                )}
              </div>
            ) : null}

            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex ${
                  msg.role === "user" ? "justify-end" : "justify-start"
                }`}
              >
                {msg.role === "user" ? (
                  <div className="max-w-[80%] rounded-2xl rounded-br-sm bg-[#8b5cf6] px-4 py-3 text-sm leading-relaxed text-white whitespace-pre-wrap">
                    {msg.content}
                  </div>
                ) : (
                  <div className="max-w-[85%] rounded-2xl rounded-bl-sm border border-[#1e1e2e] bg-[#14141f] px-5 py-4 text-sm text-[#e2e8f0]">
                    {msg.content ? (
                      <div className="chat-markdown">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>
                          {msg.content}
                        </ReactMarkdown>
                      </div>
                    ) : (
                      <span className="inline-flex gap-1">
                        <span className="animate-pulse">●</span>
                        <span
                          className="animate-pulse"
                          style={{ animationDelay: "0.2s" }}
                        >
                          ●
                        </span>
                        <span
                          className="animate-pulse"
                          style={{ animationDelay: "0.4s" }}
                        >
                          ●
                        </span>
                      </span>
                    )}
                  </div>
                )}
              </div>
            ))}

            {/* Guest limit reached banner */}
            {guestLimitReached && !streaming && (
              <div className="mx-auto mt-6 max-w-md rounded-lg border border-[#f59e0b]/30 bg-[#f59e0b]/5 p-4 text-center">
                <p className="mb-2 text-sm font-medium text-[#f59e0b]">
                  🔒 Guest round limit reached
                </p>
                <p className="mb-3 text-xs text-[#94a3b8]">
                  Connect your wallet and sign in to unlock unlimited
                  conversations and saved chat history.
                </p>
                <Link
                  href={`/soul/${handle}`}
                  className="inline-block rounded bg-[#8b5cf6] px-4 py-2 text-xs font-semibold text-white hover:bg-[#a78bfa]"
                >
                  Back to Soul Profile
                </Link>
              </div>
            )}
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="border-t border-red-500/30 bg-red-500/5 px-4 py-2 text-center text-xs text-red-400">
            {error}
          </div>
        )}

        {/* Input area */}
        <div className="border-t border-[#1e1e2e] bg-[#14141f] px-4 py-4 pb-6">
          <div className="mx-auto max-w-3xl">
            <div className="relative rounded-xl border border-[#1e1e2e] bg-[#0a0a0f] focus-within:border-[#8b5cf6] transition-colors">
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => {
                  setInput(e.target.value);
                  // Auto-resize textarea
                  const el = e.target;
                  el.style.height = "auto";
                  el.style.height = Math.min(el.scrollHeight, 200) + "px";
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    sendMessage();
                  }
                }}
                placeholder={
                  guestLimitReached
                    ? "Sign in to continue chatting..."
                    : `Message @${handle}...`
                }
                rows={3}
                className="w-full resize-none rounded-xl bg-transparent px-4 pt-3 pb-12 text-sm leading-relaxed text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none"
                disabled={streaming || guestLimitReached || initLoading}
              />
              <div className="absolute right-3 bottom-3 flex items-center gap-2">
                {tier === "guest" && !guestLimitReached && rounds > 0 && (
                  <span className="text-xs text-[#94a3b8]">
                    {GUEST_MAX_ROUNDS - rounds} left
                  </span>
                )}
                <button
                  onClick={sendMessage}
                  disabled={
                    streaming ||
                    !input.trim() ||
                    guestLimitReached ||
                    initLoading
                  }
                  className="flex h-8 w-8 items-center justify-center rounded-lg bg-[#8b5cf6] text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-40"
                >
                  {streaming ? (
                    <span className="block h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                  ) : (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 24 24"
                      fill="currentColor"
                      className="h-4 w-4"
                    >
                      <path d="M3.478 2.404a.75.75 0 0 0-.926.941l2.432 7.905H13.5a.75.75 0 0 1 0 1.5H4.984l-2.432 7.905a.75.75 0 0 0 .926.94 60.519 60.519 0 0 0 18.445-8.986.75.75 0 0 0 0-1.218A60.517 60.517 0 0 0 3.478 2.404Z" />
                    </svg>
                  )}
                </button>
              </div>
            </div>
            <p className="mt-2 text-center text-xs text-[#94a3b8]/60">
              Enter to send · Shift+Enter for new line
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

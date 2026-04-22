"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import {
  workspaceApi,
  emailAuthApi,
  ApiError,
  type Workspace,
  type VibeMemory,
  type VibeChatItem,
  type VibeChatMsg,
  type EmailSessionInfo,
} from "@/lib/api";
import UpgradeModal from "@/components/UpgradeModal";
import LoginModal from "@/components/LoginModal";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

// Memory category config
const MEMORY_CATEGORIES = [
  { key: "profile" as const, icon: "👤", color: "#8b5cf6" },
  { key: "knowledge" as const, icon: "📚", color: "#3b82f6" },
  { key: "network" as const, icon: "🤝", color: "#10b981" },
  { key: "archive" as const, icon: "📂", color: "#f59e0b" },
  { key: "rules" as const, icon: "📏", color: "#ef4444" },
] as const;

// Parse memory suggestion from AI message
interface MemorySuggestion {
  category: string;
  content: string;
  reason: string;
}
function parseMemorySuggestion(text: string): { body: string; suggestion: MemorySuggestion | null } {
  const match = text.match(/:::memory-suggest\s*\n([\s\S]*?):::/);
  if (!match) return { body: text, suggestion: null };
  const block = match[1];
  const catMatch = block.match(/category:\s*(.+)/);
  const contentMatch = block.match(/content:\s*(.+)/);
  const reasonMatch = block.match(/reason:\s*(.+)/);
  if (!catMatch || !contentMatch) return { body: text, suggestion: null };
  return {
    body: text.replace(match[0], "").trim(),
    suggestion: {
      category: catMatch[1].trim(),
      content: contentMatch[1].trim(),
      reason: reasonMatch?.[1]?.trim() || "",
    },
  };
}

export default function VibeWritePage() {
  const t = useTranslations("VibeWrite2");

  // Auth
  const [user, setUser] = useState<EmailSessionInfo | null>(null);
  const [authChecked, setAuthChecked] = useState(false);

  // Workspace
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [activeWsId, setActiveWsId] = useState<string | null>(null);
  const [wsDropdownOpen, setWsDropdownOpen] = useState(false);

  // Chats
  const [chats, setChats] = useState<VibeChatItem[]>([]);
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [messages, setMessages] = useState<VibeChatMsg[]>([]);

  // Memories (right panel)
  const [memories, setMemories] = useState<VibeMemory[]>([]);

  // UI state
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [sendingElapsed, setSendingElapsed] = useState(0);
  const sendingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [rightPanelOpen, setRightPanelOpen] = useState(true);
  const [rightTab, setRightTab] = useState<"memory" | "context">("memory");
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeReason, setUpgradeReason] = useState<"credits" | "workspace" | "memory" | "feature">("feature");
  const [currentView, setCurrentView] = useState<"chat" | "memory">("chat");
  const [soulEnhancedByMsg, setSoulEnhancedByMsg] = useState<Record<string, string[]>>({});
  const [memoryCatsByMsg, setMemoryCatsByMsg] = useState<Record<string, string[]>>({});
  const [methodologySlugsByMsg, setMethodologySlugsByMsg] = useState<Record<string, string[]>>({});
  const [outputLangsByMsg, setOutputLangsByMsg] = useState<Record<string, string[]>>({});
  type VariantItem = { idx: number; content: string; recommended?: boolean; reason?: string; lang?: string };
  const [variantsByMsg, setVariantsByMsg] = useState<Record<string, VariantItem[]>>({});
  const [archivedMsgs, setArchivedMsgs] = useState<Set<string>>(new Set());
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [setupName, setSetupName] = useState("");
  const [setupHandle, setSetupHandle] = useState("");
  const [setupCreating, setSetupCreating] = useState(false);
  const [hasMoreMessages, setHasMoreMessages] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const messagesScrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const workspacesReqSeq = useRef(0);
  const chatsReqSeq = useRef(0);
  const memoriesReqSeq = useRef(0);
  const messagesReqSeq = useRef(0);
  // Set of chat ids that were just created locally inside `handleSend`.
  // The messages useEffect skips its initial fetch for these so it does
  // not duplicate the optimistic user bubble or clobber the streaming
  // assistant placeholder.
  const skipNextMessagesFetch = useRef<Set<string>>(new Set());

  const activeWs = workspaces.find((ws) => ws.id === activeWsId);

  function showUpgrade(reason: "credits" | "workspace" | "memory" | "feature") {
    setUpgradeReason(reason);
    setUpgradeOpen(true);
  }

  // Check auth
  const checkAuth = useCallback(() => {
    emailAuthApi
      .session()
      .then((u) => setUser(u))
      .catch(() => setUser(null))
      .finally(() => setAuthChecked(true));
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    function onFocus() {
      if (!user) checkAuth();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [user, checkAuth]);

  useEffect(() => {
    function onAuthChanged() {
      checkAuth();
    }
    window.addEventListener("ensoul:auth-changed", onAuthChanged);
    return () => window.removeEventListener("ensoul:auth-changed", onAuthChanged);
  }, [checkAuth]);

  // Block the browser's default drag-and-drop behaviour at the page level so a
  // mis-aimed file drop on the vibe-write page never causes the browser to
  // navigate away by opening / downloading the file. The Smart Import
  // dropzone re-enables drop only inside its own container.
  useEffect(() => {
    function block(e: DragEvent) {
      e.preventDefault();
    }
    window.addEventListener("dragover", block);
    window.addEventListener("drop", block);
    return () => {
      window.removeEventListener("dragover", block);
      window.removeEventListener("drop", block);
    };
  }, []);

  // Load workspaces
  useEffect(() => {
    if (!user) return;
    const reqId = ++workspacesReqSeq.current;
    workspaceApi.list().then((res) => {
      if (reqId !== workspacesReqSeq.current) return;
      setWorkspaces(res.workspaces || []);
      if (res.workspaces?.length > 0 && !activeWsId) {
        setActiveWsId(res.workspaces[0].id);
      }
    }).catch(() => {});
  }, [user]);

  // Load chats when workspace changes
  useEffect(() => {
    if (!activeWsId) { setChats([]); setActiveChatId(null); return; }
    const reqId = ++chatsReqSeq.current;
    workspaceApi.listChats(activeWsId).then((res) => {
      if (reqId !== chatsReqSeq.current) return;
      setChats(res.chats || []);
    }).catch(() => {});
  }, [activeWsId]);

  // Load memories when workspace changes
  useEffect(() => {
    if (!activeWsId) { setMemories([]); return; }
    const reqId = ++memoriesReqSeq.current;
    workspaceApi.listMemories(activeWsId).then((res) => {
      if (reqId !== memoriesReqSeq.current) return;
      setMemories(res.memories || []);
    }).catch(() => {});
  }, [activeWsId]);

  // Load messages
  useEffect(() => {
    if (!activeChatId) {
      setMessages([]);
      setSoulEnhancedByMsg({});
      setMemoryCatsByMsg({});
      return;
    }
    // If this chat was just created via `handleSend`, the optimistic
    // user/assistant placeholders are already in `messages`. Skip the
    // remote fetch once to avoid the duplicate-bubble / lost-stream bug.
    if (skipNextMessagesFetch.current.has(activeChatId)) {
      skipNextMessagesFetch.current.delete(activeChatId);
      return;
    }
    const reqId = ++messagesReqSeq.current;
    workspaceApi.getMessages(activeChatId, { limit: 50 }).then((res) => {
      if (reqId !== messagesReqSeq.current) return;
      const loadedMessages = res.messages || [];
      setMessages(loadedMessages);
      setHasMoreMessages(res.has_more ?? false);
      const soulMap: Record<string, string[]> = {};
      const catMap: Record<string, string[]> = {};
      for (const msg of loadedMessages) {
        if (msg.role === "assistant") {
          if (msg.soul_handles?.length) soulMap[msg.id] = msg.soul_handles;
          if (msg.memory_cats?.length) catMap[msg.id] = msg.memory_cats;
        }
      }
      setSoulEnhancedByMsg(soulMap);
      setMemoryCatsByMsg(catMap);
    }).catch(() => {});
  }, [activeChatId]);

  // Scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // Ensure workspace exists
  const ensureWorkspace = useCallback(async () => {
    if (workspaces.length > 0) return workspaces[0].id;
    try {
      const ws = await workspaceApi.create("My Workspace");
      setWorkspaces([ws]);
      setActiveWsId(ws.id);
      return ws.id;
    } catch { return null; }
  }, [workspaces]);

  // Load older messages
  async function handleLoadMore() {
    if (!activeChatId || loadingMore || !hasMoreMessages || messages.length === 0) return;
    setLoadingMore(true);
    const oldest = messages[0].created_at;
    try {
      const res = await workspaceApi.getMessages(activeChatId, { limit: 50, before: oldest });
      const older = res.messages || [];
      if (older.length === 0) { setHasMoreMessages(false); return; }
      const container = messagesScrollRef.current;
      const prevScrollHeight = container?.scrollHeight ?? 0;
      setMessages((prev) => [...older, ...prev]);
      setHasMoreMessages(res.has_more ?? false);
      const soulMap: Record<string, string[]> = {};
      const catMap: Record<string, string[]> = {};
      for (const msg of older) {
        if (msg.role === "assistant") {
          if (msg.soul_handles?.length) soulMap[msg.id] = msg.soul_handles;
          if (msg.memory_cats?.length) catMap[msg.id] = msg.memory_cats;
        }
      }
      setSoulEnhancedByMsg((prev) => ({ ...soulMap, ...prev }));
      setMemoryCatsByMsg((prev) => ({ ...catMap, ...prev }));
      requestAnimationFrame(() => {
        if (container) container.scrollTop = container.scrollHeight - prevScrollHeight;
      });
    } catch {} finally {
      setLoadingMore(false);
    }
  }

  // Send message
  async function handleSend(prefill?: string) {
    const text = prefill || input.trim();
    if (!text || sending) return;
    setSending(true);
    setSendingElapsed(0);
    sendingTimerRef.current = setInterval(() => setSendingElapsed((s) => s + 1), 1000);
    try {
      let wsId = activeWsId;
      if (!wsId) { wsId = await ensureWorkspace(); if (!wsId) return; }

      let chatId = activeChatId;
      if (!chatId) {
        const chat = await workspaceApi.createChat(wsId);
        setChats((prev) => [chat, ...prev]);
        chatId = chat.id;
        // Mark this chat so the messages useEffect doesn't refetch
        // and overwrite the optimistic placeholders we're about to push.
        skipNextMessagesFetch.current.add(chatId);
        setActiveChatId(chatId);
      }

      if (!prefill) setInput("");
      setCurrentView("chat");

      // ---- Parse [Tweet]...[/Tweet] markers into structured attached_tweet ----
      let cleanText = text;
      let attachedTweet: { url?: string; author_handle?: string; text?: string } | undefined;
      const tweetMatch = text.match(/\[Tweet\]([\s\S]*?)\[\/Tweet\]/i);
      if (tweetMatch) {
        const block = tweetMatch[1].trim();
        // Optional URL line + body
        const urlMatch = block.match(/(https?:\/\/(?:twitter\.com|x\.com)\/([A-Za-z0-9_]{1,15})\/status\/\d+)/);
        attachedTweet = {
          url: urlMatch?.[1],
          author_handle: urlMatch?.[2],
          text: block.replace(/^https?:\/\/\S+\s*/, "").trim() || block,
        };
        cleanText = text.replace(tweetMatch[0], "").trim();
        if (!cleanText) cleanText = "Help me reply";
      }

      // ---- Auto-detect bare tweet URL (no [Tweet] markers needed) ----
      // The model can't follow links, so a pasted URL alone makes it answer
      // "I can't access external links". Detect it client-side, fetch the
      // tweet body via the backend, and attach it transparently.
      if (!attachedTweet) {
        const bareUrlMatch = cleanText.match(
          /(https?:\/\/(?:twitter\.com|x\.com)\/([A-Za-z0-9_]{1,15})\/status\/\d+)/
        );
        if (bareUrlMatch) {
          const url = bareUrlMatch[1];
          const handle = bareUrlMatch[2];
          try {
            const fetched = await workspaceApi.fetchTweet(url);
            attachedTweet = {
              url: fetched.url || url,
              author_handle: fetched.author_handle || handle,
              text: fetched.text || "",
            };
            // Strip the URL from the visible text; keep the user's intent words
            cleanText = cleanText.replace(url, "").replace(/\s+/g, " ").trim();
            if (!cleanText) cleanText = "Help me reply";
          } catch (err) {
            console.warn("[vibe-write] failed to fetch tweet, falling back", err);
            // Still attach minimal info so backend goes into reply mode
            attachedTweet = { url, author_handle: handle, text: "" };
            cleanText = cleanText.replace(url, "").replace(/\s+/g, " ").trim();
            if (!cleanText) cleanText = "Help me reply";
          }
        }
      }

      const tempUserMsg: VibeChatMsg = {
        id: `temp-${Date.now()}`, chat_id: chatId, role: "user",
        content: text, credits_cost: 0, created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, tempUserMsg]);

      // Streaming placeholder for assistant
      const tempAsstId = `temp-asst-${Date.now()}`;
      const tempAsstMsg: VibeChatMsg = {
        id: tempAsstId, chat_id: chatId, role: "assistant",
        content: "", credits_cost: 0, created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, tempAsstMsg]);

      let metaSoulHandles: string[] = [];
      let metaMemoryCats: string[] = [];
      let metaMethodologySlugs: string[] = [];
      let metaOutputLangs: string[] = [];
      let metaScenario = "";
      const collectedVariants: VariantItem[] = [];

      await workspaceApi.sendMessageStream(
        chatId,
        attachedTweet
          ? { content: cleanText, attached_tweet: attachedTweet, variant_count: user?.is_pro ? 3 : 1 }
          : cleanText,
        {
        onMeta: (m) => {
          metaScenario = m.scenario || "";
        },
        onContext: (ctx) => {
          metaSoulHandles = ctx.soul_handles || [];
          metaMemoryCats = ctx.used_memory_cats || [];
          metaMethodologySlugs = ctx.methodology_slugs || [];
          metaOutputLangs = (ctx.output_langs || []).filter((l) => !!l);
        },
        onSoulLock: (info) => {
          // Backend only emits this when a Free user attached a tweet
          // whose author owns a Soul. Guard against stale/spurious events.
          if (!attachedTweet) {
            console.warn("[vibe-write] received soul_lock without attached tweet, ignoring", info);
            return;
          }
          showUpgrade("feature");
          console.info("[vibe-write] soul lock for @" + info.handle);
        },
        onVariant: (v) => {
          collectedVariants.push(v);
          // Stash structured variants under the temp message id; rendering
          // logic checks variantsByMsg first and renders cards instead of markdown.
          setVariantsByMsg((prev) => ({
            ...prev,
            [tempAsstId]: [...collectedVariants].sort((a, b) => a.idx - b.idx),
          }));
        },
        onMemorySuggest: (m) => {
          setMemories((prev) => {
            // Append if not already present
            if (prev.some((x) => x.id === m.id)) return prev;
            return [...prev, m as VibeMemory];
          });
        },
        onChunk: (token) => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAsstId ? { ...msg, content: msg.content + token } : msg
            )
          );
        },
        onDone: (info) => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAsstId
                ? {
                    ...msg,
                    id: info.assistant_message_id,
                    credits_cost: info.credits_used,
                    content: info.cleaned_content ?? msg.content,
                    scenario: metaScenario || msg.scenario,
                  }
                : msg.id === tempUserMsg.id
                ? { ...msg, id: msg.id.replace("temp-", "saved-") }
                : msg
            )
          );
          if (info.pending_memories && info.pending_memories.length > 0) {
            setMemories((prev) => [...prev, ...info.pending_memories!]);
          }
          if (metaSoulHandles.length > 0) {
            setSoulEnhancedByMsg((prev) => ({
              ...prev,
              [info.assistant_message_id]: metaSoulHandles,
            }));
          }
          if (metaMemoryCats.length > 0) {
            setMemoryCatsByMsg((prev) => ({
              ...prev,
              [info.assistant_message_id]: metaMemoryCats,
            }));
          }
          if (metaMethodologySlugs.length > 0) {
            setMethodologySlugsByMsg((prev) => ({
              ...prev,
              [info.assistant_message_id]: metaMethodologySlugs,
            }));
          }
          if (metaOutputLangs.length > 0) {
            setOutputLangsByMsg((prev) => ({
              ...prev,
              [info.assistant_message_id]: metaOutputLangs,
            }));
          }
          // Migrate variants under temp id → real assistant id
          if (collectedVariants.length > 0) {
            setVariantsByMsg((prev) => {
              const next = { ...prev };
              delete next[tempAsstId];
              next[info.assistant_message_id] = [...collectedVariants].sort((a, b) => a.idx - b.idx);
              return next;
            });
          }
          setUser((prev) =>
            prev ? { ...prev, credits: prev.credits - info.credits_used } : prev
          );
        },
      });

      setChats((prev) =>
        prev.map((c) =>
          c.id === chatId
            ? { ...c, title: text.slice(0, 50) + (text.length > 50 ? "..." : ""), updated_at: new Date().toISOString() }
            : c
        )
      );
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to send";
      const code = err instanceof ApiError ? err.code : undefined;
      if (code === "INSUFFICIENT_CREDITS" || msg.includes("credits") || msg.includes("insufficient")) {
        showUpgrade("credits");
      } else if (code === "WORKSPACE_LIMIT" || msg.includes("workspace limit")) {
        showUpgrade("workspace");
      } else {
        setMessages((prev) => [...prev, {
          id: `error-${Date.now()}`, chat_id: activeChatId || "", role: "assistant",
          content: `⚠️ ${msg}`, credits_cost: 0, created_at: new Date().toISOString(),
        }]);
      }
    } finally {
      if (sendingTimerRef.current) { clearInterval(sendingTimerRef.current); sendingTimerRef.current = null; }
      setSendingElapsed(0);
      setSending(false);
      inputRef.current?.focus();
    }
  }

  function handleNewChat() {
    setActiveChatId(null);
    setMessages([]);
    setInput("");
    setCurrentView("chat");
    inputRef.current?.focus();
  }

  async function handleDeleteChat(chatId: string) {
    try {
      await workspaceApi.deleteChat(chatId);
      setChats((prev) => prev.filter((c) => c.id !== chatId));
      if (activeChatId === chatId) { setActiveChatId(null); setMessages([]); }
    } catch {}
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleSend(); }
  }

  // Quick actions
  function handleQuickAction(action: string) {
    handleNewChat();
    setTimeout(() => {
      setInput(action);
      inputRef.current?.focus();
    }, 50);
  }

  // Memory suggestion handling
  const [dismissedSuggestions, setDismissedSuggestions] = useState<Set<string>>(new Set());

  async function handleSaveMemory(msgId: string, suggestion: MemorySuggestion) {
    if (!activeWsId) return;
    try {
      const mem = await workspaceApi.createMemory(activeWsId, suggestion.category, suggestion.content);
      setMemories((prev) => [...prev, mem]);
      setDismissedSuggestions((prev) => new Set(prev).add(msgId));
    } catch {
      // ignore
    }
  }

  function handleSkipMemory(msgId: string) {
    setDismissedSuggestions((prev) => new Set(prev).add(msgId));
  }

  // Login modal (for unauthenticated CTA)
  const [addingMemory, setAddingMemory] = useState<{ cat: string; content: string } | null>(null);
  const [loginOpen, setLoginOpen] = useState(false);

  // Smart Import (paste any text → AI auto-categorises into the 5 memory categories)
  const [importOpen, setImportOpen] = useState(false);
  const [importText, setImportText] = useState("");
  const [importMode, setImportMode] = useState<"review" | "auto-accept">("review");
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [importDragOver, setImportDragOver] = useState(false);

  // Memory-page header: rename workspace (inline edit) + refresh self-portrait
  // + import from arbitrary Twitter handle.
  const [renamingWs, setRenamingWs] = useState(false);
  const [renameValue, setRenameValue] = useState("");
  const [renameSaving, setRenameSaving] = useState(false);
  const [refreshOpen, setRefreshOpen] = useState(false);
  const [refreshAutoAccept, setRefreshAutoAccept] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<string | null>(null);
  const [twImportOpen, setTwImportOpen] = useState(false);
  const [twImportHandle, setTwImportHandle] = useState("");
  const [twImportAutoAccept, setTwImportAutoAccept] = useState(false);
  const [twImporting, setTwImporting] = useState(false);
  const [twImportError, setTwImportError] = useState<string | null>(null);
  const importFileInputRef = useRef<HTMLInputElement>(null);

  // Read a .md / .txt / .markdown file into the import textarea.
  // Multiple files are concatenated with a separator.
  async function loadImportFiles(files: FileList | File[]) {
    const accepted: File[] = [];
    const list = Array.from(files);
    for (const f of list) {
      const lower = f.name.toLowerCase();
      const isText =
        f.type.startsWith("text/") ||
        lower.endsWith(".md") ||
        lower.endsWith(".markdown") ||
        lower.endsWith(".txt");
      if (!isText) continue;
      // Hard cap per file at the same 20k limit; oversized files trigger a friendly error.
      if (f.size > 20000 * 4) {
        setImportError(t("smartImportTooLong"));
        return;
      }
      accepted.push(f);
    }
    if (accepted.length === 0) {
      setImportError(t("smartImportFileType"));
      return;
    }
    try {
      const parts = await Promise.all(
        accepted.map(async (f) => {
          const text = await f.text();
          return accepted.length > 1 ? `# ${f.name}\n\n${text}` : text;
        })
      );
      const merged = parts.join("\n\n---\n\n");
      setImportText((prev) => {
        const combined = prev.trim() ? prev.trim() + "\n\n" + merged : merged;
        return combined.slice(0, 20000);
      });
      setImportError(null);
    } catch {
      setImportError(t("smartImportFailed"));
    }
  }

  async function handleSmartImport() {
    if (!activeWsId) return;
    const text = importText.trim();
    if (!text) return;
    setImporting(true);
    setImportError(null);
    try {
      const res = await workspaceApi.importMemories(activeWsId, text, importMode);
      // Merge new memories into local state.
      setMemories((prev) => [...prev, ...res.suggestions]);
      setImportOpen(false);
      setImportText("");
    } catch (err: unknown) {
      const code = err instanceof ApiError ? err.code : undefined;
      if (code === "TEXT_TOO_LONG") setImportError(t("smartImportTooLong"));
      else if (code === "RATE_LIMITED") setImportError(t("smartImportRateLimit"));
      else if (code === "WORKSPACE_MEMORY_FULL") setImportError(t("smartImportFull"));
      else if (code === "INVALID_CONTENT") setImportError(t("smartImportFileType"));
      else setImportError(t("smartImportFailed"));
    } finally {
      setImporting(false);
    }
  }

  // ---- Memory page: rename workspace (inline edit) ----
  function startRenameWs() {
    if (!activeWs) return;
    setRenameValue(activeWs.name || "");
    setRenamingWs(true);
  }
  async function commitRenameWs() {
    if (!activeWsId) { setRenamingWs(false); return; }
    const next = renameValue.trim();
    if (!next || next === activeWs?.name) { setRenamingWs(false); return; }
    if (next.length > 100) return;
    setRenameSaving(true);
    try {
      const updated = await workspaceApi.update(activeWsId, { name: next });
      setWorkspaces((prev) => prev.map((w) => (w.id === activeWsId ? { ...w, ...updated } : w)));
      setRenamingWs(false);
    } catch {
      // Silently revert; UX is non-blocking.
      setRenamingWs(false);
    } finally {
      setRenameSaving(false);
    }
  }

  // ---- Memory page: refresh self-portrait (re-distill workspace's own handle) ----
  function mapTwitterErrorCode(code?: string): string {
    if (code === "INSUFFICIENT_CREDITS") return t("creditsInsufficient");
    if (code === "RATE_LIMITED") return t("importTwitterRateLimited");
    if (code === "TWITTER_FETCH_FAILED" || code === "PROFILE_FETCH_FAILED") return t("importTwitterFetchFailed");
    if (code === "INVALID_HANDLE") return t("importTwitterInvalidHandle");
    if (code === "WORKSPACE_MEMORY_FULL") return t("smartImportFull");
    return t("importTwitterFailed");
  }
  async function handleRefreshSelfPortrait() {
    if (!activeWsId || !activeWs?.twitter_handle) return;
    setRefreshing(true);
    setRefreshError(null);
    try {
      const res = await workspaceApi.setup(activeWsId, activeWs.twitter_handle, refreshAutoAccept);
      if (res.pending_memories?.length) {
        setMemories((prev) => [...prev, ...res.pending_memories]);
      }
      setRefreshOpen(false);
    } catch (err: unknown) {
      const code = err instanceof ApiError ? err.code : undefined;
      setRefreshError(mapTwitterErrorCode(code));
    } finally {
      setRefreshing(false);
    }
  }

  // Extract a Twitter/X handle from raw user input. Accepts:
  //   "nina_rong", "@nina_rong",
  //   "x.com/nina_rong", "twitter.com/nina_rong",
  //   "https://x.com/nina_rong", "https://x.com/nina_rong/status/123",
  //   "https://x.com/nina_rong?lang=en"
  // Returns "" when nothing usable is found.
  function extractTwitterHandle(raw: string): string {
    const s = raw.trim();
    if (!s) return "";
    // URL form
    const urlMatch = s.match(/(?:https?:\/\/)?(?:www\.)?(?:twitter\.com|x\.com)\/(?:#!\/)?@?([A-Za-z0-9_]{1,15})\b/i);
    if (urlMatch) return urlMatch[1];
    // Bare @handle / handle
    const bare = s.replace(/^@/, "").split(/[/?#\s]/)[0];
    if (/^[A-Za-z0-9_]{1,15}$/.test(bare)) return bare;
    return "";
  }

  // ---- Memory page: import arbitrary Twitter handle ----
  async function handleTwitterImport() {
    if (!activeWsId) return;
    const handle = extractTwitterHandle(twImportHandle);
    if (!handle) {
      setTwImportError(t("importTwitterInvalidHandle"));
      return;
    }
    setTwImporting(true);
    setTwImportError(null);
    try {
      const res = await workspaceApi.importTwitter(activeWsId, handle, twImportAutoAccept);
      if (res.pending_memories?.length) {
        setMemories((prev) => [...prev, ...res.pending_memories]);
      }
      setTwImportOpen(false);
      setTwImportHandle("");
    } catch (err: unknown) {
      const code = err instanceof ApiError ? err.code : undefined;
      setTwImportError(mapTwitterErrorCode(code));
    } finally {
      setTwImporting(false);
    }
  }

  if (!authChecked) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  // --- Unauthenticated: show demo preview ---
  if (!user) {
    return (
      <div className="flex h-full">
        {/* Demo sidebar */}
        <aside className="flex w-[280px] shrink-0 flex-col border-r border-[#1e1e2e] bg-[#0d0d14]">
          {/* Demo workspace header */}
          <div className="border-b border-[#1e1e2e] p-3">
            <div className="flex items-center gap-2.5 rounded-lg bg-[#1e1e2e]/50 px-3 py-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-sm text-[#8b5cf6]">✦</div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-[#e2e8f0]">Vibe Write</div>
                <div className="text-xs text-[#64748b]">Demo</div>
              </div>
            </div>
          </div>
          <div className="p-3">
            <div className="flex w-full items-center justify-center gap-2 rounded-lg border border-[#1e1e2e] px-3 py-2 text-sm text-[#94a3b8]">
              <span>+</span>
              <span>{t("newChat")}</span>
            </div>
          </div>
          {/* Demo quick actions */}
          <div className="flex gap-2 px-3 pb-2">
            <span className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8]">🐦 {t("quickTweet")}</span>
            <span className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8]">💬 {t("quickReply")}</span>
          </div>
          <div className="flex-1 px-2 space-y-0.5">
            {[
              { id: "d1", title: t("demoChat1"), badge: "create" },
              { id: "d2", title: t("demoChat2"), badge: "create" },
              { id: "d3", title: t("demoChat3"), badge: "deep" },
            ].map((chat, i) => (
              <div
                key={chat.id}
                className={`rounded-lg px-3 py-2 text-sm ${
                  i === 0 ? "bg-[#1e1e2e] text-[#e2e8f0]" : "text-[#94a3b8]"
                }`}
              >
                <span className="truncate">{chat.title}</span>
              </div>
            ))}
          </div>
          <div className="border-t border-[#1e1e2e] p-2">
            <div className="flex">
              <div className="flex-1 rounded-lg py-2 text-center text-xs text-[#64748b]">🧠 {t("memory")}</div>
              <div className="flex-1 rounded-lg py-2 text-center text-xs text-[#64748b]">⚙️ {t("settings")}</div>
            </div>
          </div>
        </aside>

        {/* Demo conversation */}
        <div className="flex flex-1 flex-col">
          <div className="flex-1 overflow-y-auto">
            <div className="mx-auto max-w-3xl px-4 py-6 space-y-6">
              {([
                { role: "user" as const, content: t("demoUserMsg1") },
                { role: "assistant" as const, content: t("demoAssistantMsg1") },
                { role: "user" as const, content: t("demoUserMsg2") },
                { role: "assistant" as const, content: t("demoAssistantMsg2") },
              ]).map((msg, i) => (
                <div key={i} className={`flex gap-3 ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                  {msg.role === "assistant" && (
                    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-xs text-[#8b5cf6]">✦</div>
                  )}
                  <div className={`max-w-[80%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
                    msg.role === "user" ? "bg-[#8b5cf6] text-white" : "bg-[#1e1e2e] text-[#e2e8f0]"
                  }`}>
                    {msg.role === "assistant" ? (
                      <div className="chat-markdown"><ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown></div>
                    ) : (
                      <p className="whitespace-pre-wrap">{msg.content}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* CTA bar */}
          <div className="border-t border-[#1e1e2e] bg-[#0a0a0f] p-4">
            <div className="mx-auto max-w-3xl">
              <button
                onClick={() => setLoginOpen(true)}
                className="flex w-full items-center justify-center gap-3 rounded-xl border border-[#8b5cf6]/30 bg-[#14141f] px-4 py-3.5 transition-all hover:border-[#8b5cf6] hover:bg-[#1e1e2e]"
              >
                <span className="text-sm text-[#94a3b8]">{t("demoCtaHint")}</span>
                <span className="rounded-lg bg-[#8b5cf6] px-4 py-1.5 text-sm font-semibold text-white shadow-md shadow-[#8b5cf6]/20">
                  {t("demoStart")}
                </span>
              </button>
              <p className="mt-2 text-center text-xs text-[#64748b]">{t("demoFree")}</p>
            </div>
          </div>
        </div>

        {/* Demo right panel */}
        <aside className="hidden w-[300px] shrink-0 flex-col border-l border-[#1e1e2e] bg-[#0d0d14] lg:flex">
          <div className="flex border-b border-[#1e1e2e]">
            <div className="flex-1 border-b-2 border-[#8b5cf6] py-2.5 text-center text-xs font-medium text-[#e2e8f0]">{t("memory")}</div>
            <div className="flex-1 py-2.5 text-center text-xs text-[#64748b]">{t("context")}</div>
          </div>
          <div className="flex-1 overflow-y-auto p-3 space-y-4">
            <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
              <div className="mb-1 text-xs font-medium text-[#8b5cf6]">👤 {t("catProfile")}</div>
              <div className="text-xs text-[#94a3b8]">{t("demoMemoryProfile")}</div>
            </div>
            <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
              <div className="mb-1 text-xs font-medium text-[#ef4444]">📏 {t("catRules")}</div>
              <div className="text-xs text-[#94a3b8]">{t("demoMemoryRules")}</div>
            </div>
          </div>
        </aside>

        <LoginModal
          open={loginOpen}
          onClose={() => setLoginOpen(false)}
          onLogin={(u) => { setUser(u); setLoginOpen(false); window.dispatchEvent(new Event("ensoul:auth-changed")); }}
        />
      </div>
    );
  }

  // Helper: group memories by category (accepted + legacy without status)
  const memoriesByCategory = MEMORY_CATEGORIES.map((cat) => ({
    ...cat,
    label: t(`cat${cat.key.charAt(0).toUpperCase()}${cat.key.slice(1)}` as Parameters<typeof t>[0]),
    items: memories.filter(
      (m) => m.category === cat.key && (m.status === "accepted" || !m.status)
    ),
  }));

  // AI-suggested memories awaiting user review
  const pendingMemories = memories.filter((m) => m.status === "pending");

  async function reviewPending(memId: string, action: "accept" | "reject") {
    try {
      const updated = await workspaceApi.reviewMemory(memId, action);
      setMemories((prev) => prev.map((m) => (m.id === memId ? updated : m)));
    } catch {
      // ignore
    }
  }

  async function handleFeedback(msgId: string, value: -1 | 0 | 1) {
    // Optimistic update
    setMessages((prev) =>
      prev.map((m) => (m.id === msgId ? { ...m, feedback: value } : m))
    );
    try {
      await workspaceApi.feedbackMessage(msgId, value);
    } catch {
      // Revert on failure
      setMessages((prev) =>
        prev.map((m) => (m.id === msgId ? { ...m, feedback: 0 } : m))
      );
    }
  }

  // Copy a string to clipboard, flashing a "copied" indicator under `key`.
  async function copyToClipboard(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedKey(key);
      setTimeout(() => setCopiedKey((cur) => (cur === key ? null : cur)), 1500);
    } catch {
      // Fallback: best-effort no-op (clipboard blocked in some browsers/contexts)
    }
  }

  function openOnTwitter(text: string) {
    const url = `https://twitter.com/intent/tweet?text=${encodeURIComponent(text)}`;
    window.open(url, "_blank", "noopener,noreferrer");
  }

  function refineVariant(text: string) {
    setInput((prev) => {
      const prefix = prev.trim() ? prev.trim() + "\n\n" : "";
      return prefix + t("refinePrefix") + "\n" + text;
    });
    inputRef.current?.focus();
  }

  // Save an assistant message into the Archive memory category.
  async function handleSaveToArchive(msg: VibeChatMsg, contentOverride?: string) {
    if (!activeWsId) return;
    if (archivedMsgs.has(msg.id)) return;
    const content = (contentOverride ?? msg.content).trim();
    if (!content) return;
    try {
      const mem = await workspaceApi.createMemory(activeWsId, "archive", content);
      setMemories((prev) => [...prev, mem]);
      setArchivedMsgs((prev) => new Set(prev).add(msg.id));
    } catch {
      // Network / server error — silent.
    }
  }

  async function handleSetupCreate() {
    if (!setupName.trim()) return;
    setSetupCreating(true);
    try {
      const handle = setupHandle.trim().replace(/^@/, "");
      const ws = await workspaceApi.create(setupName.trim(), handle || undefined);
      setWorkspaces([ws]);
      setActiveWsId(ws.id);

      // If a Twitter handle was provided, kick off the seed-memory distillation.
      // Best-effort: failures here do NOT break workspace creation.
      if (handle) {
        try {
          const seed = await workspaceApi.setup(ws.id, handle, false);
          if (seed.pending_memories?.length) {
            setMemories((prev) => [...prev, ...seed.pending_memories]);
          }
        } catch (seedErr) {
          // Surface as a non-blocking warning in the console; user can re-run later.
          console.warn("[vibe-write] setup distillation failed:", seedErr);
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      const code = err instanceof ApiError ? err.code : undefined;
      if (code === "WORKSPACE_LIMIT" || msg.includes("workspace limit")) showUpgrade("workspace");
    } finally {
      setSetupCreating(false);
    }
  }

  if (workspaces.length === 0) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-8">
          <div className="mb-6 flex items-center justify-center gap-2">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-lg text-[#8b5cf6]">✦</div>
            <span className="text-lg font-bold text-[#e2e8f0]">Vibe Write</span>
          </div>

          <h2 className="mb-1 text-center text-base font-semibold text-[#e2e8f0]">{t("setupTitle")}</h2>
          <p className="mb-6 text-center text-xs text-[#64748b]">{t("setupDesc")}</p>

          <div className="space-y-4">
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">{t("setupStep1")}</label>
              <input
                value={setupHandle}
                onChange={(e) => setSetupHandle(e.target.value)}
                placeholder="@YourTwitterHandle"
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] placeholder-[#64748b] outline-none focus:border-[#8b5cf6]"
              />
              <p className="mt-1 text-[10px] text-[#64748b]">{t("setupHandleHint")}</p>
            </div>

            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">{t("setupStep2")}</label>
              <input
                value={setupName}
                onChange={(e) => setSetupName(e.target.value)}
                placeholder={t("setupNamePlaceholder")}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] placeholder-[#64748b] outline-none focus:border-[#8b5cf6]"
                onKeyDown={(e) => { if (e.key === "Enter") handleSetupCreate(); }}
              />
            </div>

            <button
              onClick={handleSetupCreate}
              disabled={!setupName.trim() || setupCreating}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-[#8b5cf6] px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-40"
            >
              {setupCreating ? (
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
              ) : (
                <>✦ {t("setupCreate")}</>
              )}
            </button>
          </div>
        </div>
      </div>
    );
  }

  // ---------- AUTHENTICATED: 3-panel layout ----------
  return (
    <div className="flex h-full">
      {/* ===== Left Sidebar ===== */}
      {sidebarOpen && (
        <aside className="flex w-[280px] shrink-0 flex-col border-r border-[#1e1e2e] bg-[#0d0d14]">
          {/* Workspace Switcher
              Click row     → toggle Memory view of the active workspace
              Click chevron → open switch-workspace dropdown */}
          <div className="border-b border-[#1e1e2e] p-3">
            <div
              className={`relative flex items-center gap-2.5 rounded-lg px-3 py-2 transition-colors ${
                currentView === "memory" ? "bg-[#8b5cf6]/15 ring-1 ring-[#8b5cf6]/40" : "bg-[#1e1e2e]/50 hover:bg-[#1e1e2e]"
              }`}
            >
              <button
                type="button"
                onClick={() => setCurrentView(currentView === "memory" ? "chat" : "memory")}
                className="flex flex-1 items-center gap-2.5 text-left min-w-0"
                title={currentView === "memory" ? t("backToChat") : t("openMemory")}
              >
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-sm font-bold text-[#8b5cf6]">
                  {currentView === "memory" ? "🧠" : (activeWs?.name?.charAt(0)?.toUpperCase() || "✦")}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="truncate text-sm font-medium text-[#e2e8f0]">{activeWs?.name || t("welcomeTitle")}</div>
                  <div className="truncate text-xs text-[#64748b]">
                    {currentView === "memory"
                      ? t("memory")
                      : (activeWs?.twitter_handle ? `@${activeWs.twitter_handle}` : t("clickForMemory"))}
                  </div>
                </div>
              </button>
              <button
                type="button"
                onClick={() => setWsDropdownOpen(!wsDropdownOpen)}
                className="rounded-md px-1.5 py-1 text-xs text-[#64748b] hover:bg-[#1e1e2e] hover:text-[#e2e8f0]"
                title={t("switchWorkspace")}
              >
                {wsDropdownOpen ? "▴" : "▾"}
              </button>
            </div>

            {/* Workspace dropdown */}
            {wsDropdownOpen && workspaces.length > 1 && (
              <div className="mt-1 rounded-lg border border-[#1e1e2e] bg-[#14141f] py-1 shadow-xl">
                {workspaces.map((ws) => (
                  <button
                    key={ws.id}
                    onClick={() => { setActiveWsId(ws.id); setWsDropdownOpen(false); setActiveChatId(null); setMessages([]); setCurrentView("chat"); }}
                    className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-[#1e1e2e] ${
                      ws.id === activeWsId ? "text-[#8b5cf6]" : "text-[#94a3b8]"
                    }`}
                  >
                    <div className="flex h-6 w-6 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-xs font-bold text-[#8b5cf6]">
                      {ws.name?.charAt(0)?.toUpperCase() || "W"}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="truncate">{ws.name}</div>
                      {ws.twitter_handle && <div className="truncate text-xs text-[#64748b]">@{ws.twitter_handle}</div>}
                    </div>
                    {ws.id === activeWsId && <span className="text-[#8b5cf6]">✓</span>}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* New Chat + Quick Actions */}
          <div className="p-3 space-y-2">
            <button
              onClick={handleNewChat}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-[#1e1e2e] px-3 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
            >
              <span>+</span>
              <span>{t("newChat")}</span>
            </button>
            <div className="flex flex-wrap gap-1.5">
              <button
                onClick={() => handleQuickAction(t("quickTweetPrompt"))}
                className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8] transition-colors hover:bg-[#8b5cf6]/20 hover:text-[#e2e8f0]"
              >
                🐦 {t("quickTweet")}
              </button>
              <button
                onClick={() => handleQuickAction(t("quickReplyPrompt"))}
                className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8] transition-colors hover:bg-[#8b5cf6]/20 hover:text-[#e2e8f0]"
              >
                💬 {t("quickReply")}
              </button>
              <button
                onClick={() => handleQuickAction(t("quickArticlePrompt"))}
                className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8] transition-colors hover:bg-[#8b5cf6]/20 hover:text-[#e2e8f0]"
              >
                📝 {t("quickArticle")}
              </button>
              <button
                onClick={() => handleQuickAction(t("quickGrowthPrompt"))}
                className="rounded-md bg-[#1e1e2e] px-2.5 py-1 text-xs text-[#94a3b8] transition-colors hover:bg-[#8b5cf6]/20 hover:text-[#e2e8f0]"
              >
                📈 {t("quickGrowth")}
              </button>
            </div>
          </div>

          {/* Chat List */}
          <div className="flex-1 overflow-y-auto px-2">
            {chats.length === 0 ? (
              <p className="px-3 py-4 text-center text-xs text-[#64748b]">{t("noChats")}</p>
            ) : (
              <div className="space-y-0.5">
                {chats.map((chat) => (
                  <div
                    key={chat.id}
                    className={`group flex items-center rounded-lg px-3 py-2 text-sm cursor-pointer transition-colors ${
                      activeChatId === chat.id ? "bg-[#1e1e2e] text-[#e2e8f0]" : "text-[#94a3b8] hover:bg-[#1e1e2e]/50"
                    }`}
                    onClick={() => { setActiveChatId(chat.id); setCurrentView("chat"); }}
                  >
                    <span className="flex-1 truncate">{chat.title || t("newChat")}</span>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDeleteChat(chat.id); }}
                      className="ml-2 shrink-0 text-[#64748b] opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </aside>
      )}

      {/* ===== Main Area ===== */}
      <div className="flex flex-1 flex-col relative min-w-0">
        {/* Sidebar toggle */}
        <button
          onClick={() => setSidebarOpen(!sidebarOpen)}
          className="absolute left-0 top-2 z-10 rounded-r-md border border-l-0 border-[#1e1e2e] bg-[#14141f] px-1.5 py-2 text-[#64748b] hover:text-[#e2e8f0]"
        >
          {sidebarOpen ? "◂" : "▸"}
        </button>

        {/* Right panel toggle */}
        <button
          onClick={() => setRightPanelOpen(!rightPanelOpen)}
          className="absolute right-0 top-2 z-10 rounded-l-md border border-r-0 border-[#1e1e2e] bg-[#14141f] px-1.5 py-2 text-[#64748b] hover:text-[#e2e8f0]"
        >
          {rightPanelOpen ? "▸" : "◂"}
        </button>

        {/* ---- Memory Manager View ---- */}
        {currentView === "memory" ? (
          <div className="flex-1 overflow-y-auto p-6">
            <div className="mx-auto max-w-5xl">
              <div className="mb-6 flex items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <h2 className="text-lg font-bold text-[#e2e8f0]">🧠 {t("memoryManager")}</h2>
                  {renamingWs ? (
                    <div className="mt-1 flex items-center gap-2">
                      <input
                        autoFocus
                        value={renameValue}
                        onChange={(e) => setRenameValue(e.target.value.slice(0, 100))}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") void commitRenameWs();
                          else if (e.key === "Escape") setRenamingWs(false);
                        }}
                        onBlur={() => { if (!renameSaving) void commitRenameWs(); }}
                        disabled={renameSaving}
                        maxLength={100}
                        className="w-full max-w-xs rounded-md border border-[#8b5cf6]/40 bg-[#0a0a0f] px-2 py-1 text-xs text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                      />
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={startRenameWs}
                      title={t("memoryRenameTitle")}
                      className="mt-0.5 flex items-center gap-1.5 text-xs text-[#64748b] transition-colors hover:text-[#a78bfa]"
                    >
                      <span className="truncate">{activeWs?.name}</span>
                      <span className="opacity-60">✏️</span>
                    </button>
                  )}
                </div>
                <div className="flex shrink-0 flex-wrap items-center gap-2">
                  {activeWs?.twitter_handle && (
                    <button
                      onClick={() => { setRefreshError(null); setRefreshAutoAccept(false); setRefreshOpen(true); }}
                      className="flex items-center gap-1.5 rounded-lg border border-sky-500/40 bg-sky-500/10 px-3 py-1.5 text-xs font-medium text-sky-300 transition-colors hover:bg-sky-500/20"
                    >
                      <span>🔄</span>
                      <span>{t("refreshSelfPortraitBtn")}</span>
                    </button>
                  )}
                  <button
                    onClick={() => { setTwImportError(null); setTwImportHandle(""); setTwImportAutoAccept(false); setTwImportOpen(true); }}
                    className="flex items-center gap-1.5 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-300 transition-colors hover:bg-emerald-500/20"
                  >
                    <span>🐦</span>
                    <span>{t("importTwitterBtn")}</span>
                  </button>
                  <button
                    onClick={() => { setImportError(null); setImportOpen(true); }}
                    className="flex items-center gap-1.5 rounded-lg border border-[#8b5cf6]/40 bg-[#8b5cf6]/10 px-3 py-1.5 text-xs font-medium text-[#a78bfa] transition-colors hover:bg-[#8b5cf6]/20"
                  >
                    <span>📥</span>
                    <span>{t("smartImport")}</span>
                  </button>
                </div>
              </div>

              {pendingMemories.length > 0 && (
                <div className="mb-6 rounded-xl border border-amber-500/30 bg-amber-500/5 p-4">
                  <div className="mb-3 flex items-center gap-2 text-sm font-medium text-amber-300">
                    ✨ AI suggested {pendingMemories.length} memor{pendingMemories.length === 1 ? "y" : "ies"} for review
                  </div>
                  <div className="space-y-2">
                    {pendingMemories.map((m) => {
                      const handleMatch = m.reason?.match(/@([A-Za-z0-9_]{1,30})/);
                      const sourceHandle = handleMatch?.[1] ?? null;
                      return (
                      <div key={m.id} className="flex items-start gap-2 rounded-lg border border-amber-500/20 bg-[#0d0d14] p-3">
                        <div className="flex-1">
                          <div className="mb-1 flex flex-wrap items-center gap-2 text-[10px] uppercase text-[#64748b]">
                            <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-400">{m.category}</span>
                            {sourceHandle && (
                              <span className="rounded bg-sky-500/10 px-1.5 py-0.5 text-sky-300">🐦 @{sourceHandle}</span>
                            )}
                            {m.reason && <span className="italic normal-case">— {m.reason}</span>}
                          </div>
                          <div className="text-sm text-[#e2e8f0]">{m.content}</div>
                        </div>
                        <div className="flex shrink-0 gap-1">
                          <button
                            onClick={() => reviewPending(m.id, "accept")}
                            className="rounded bg-emerald-600 px-2 py-1 text-xs font-medium text-white hover:bg-emerald-500"
                          >
                            Accept
                          </button>
                          <button
                            onClick={() => reviewPending(m.id, "reject")}
                            className="rounded border border-[#2a2a3a] px-2 py-1 text-xs text-[#94a3b8] hover:bg-[#1e1e2e]"
                          >
                            Reject
                          </button>
                        </div>
                      </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* 5-column grid */}
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
                {memoriesByCategory.map((cat) => (
                  <div key={cat.key} className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14]">
                    <div className="flex items-center justify-between border-b border-[#1e1e2e] px-3 py-2.5">
                      <div className="flex items-center gap-1.5 text-sm font-medium" style={{ color: cat.color }}>
                        <span>{cat.icon}</span>
                        <span>{cat.label}</span>
                        <span className="ml-1 rounded-full bg-[#1e1e2e] px-1.5 py-0.5 text-[10px] text-[#64748b]">{cat.items.length}</span>
                      </div>
                      <button
                        onClick={() => setAddingMemory({ cat: cat.key, content: "" })}
                        title={t("add")}
                        className="text-xs text-[#64748b] transition-colors hover:text-[#e2e8f0]"
                      >
                        ＋
                      </button>
                    </div>
                    {addingMemory?.cat === cat.key && (
                      <div className="border-b border-[#1e1e2e] p-2">
                        <textarea
                          autoFocus
                          value={addingMemory.content}
                          onChange={(e) => setAddingMemory({ cat: cat.key, content: e.target.value })}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" && !e.shiftKey) {
                              e.preventDefault();
                              const content = addingMemory.content.trim();
                              if (content && activeWsId) {
                                workspaceApi.createMemory(activeWsId, cat.key, content)
                                  .then((mem) => {
                                    setMemories((prev) => [...prev, mem]);
                                  })
                                  .catch(() => { /* silent */ });
                              }
                              setAddingMemory(null);
                            } else if (e.key === "Escape") {
                              setAddingMemory(null);
                            }
                          }}
                          placeholder={t("addMemoryPrompt")}
                          rows={2}
                          className="w-full resize-none rounded-md bg-[#1e1e2e] px-2 py-1.5 text-xs text-[#e2e8f0] placeholder-[#64748b] outline-none focus:ring-1 focus:ring-[#8b5cf6]/50"
                        />
                        <div className="mt-1 flex justify-end gap-1">
                          <button onClick={() => setAddingMemory(null)} className="px-2 py-0.5 text-[10px] text-[#64748b] hover:text-[#e2e8f0]">{t("cancel")}</button>
                          <button
                            onClick={() => {
                              const content = addingMemory.content.trim();
                              if (content && activeWsId) {
                                workspaceApi.createMemory(activeWsId, cat.key, content)
                                  .then((mem) => {
                                    setMemories((prev) => [...prev, mem]);
                                  })
                                  .catch(() => { /* silent */ });
                              }
                              setAddingMemory(null);
                            }}
                            className="rounded-md bg-[#8b5cf6]/20 px-2 py-0.5 text-[10px] text-[#8b5cf6] hover:bg-[#8b5cf6]/30"
                          >{t("save")}</button>
                        </div>
                      </div>
                    )}
                    <div className="max-h-80 overflow-y-auto p-2 space-y-2">
                      {cat.items.length === 0 ? (
                        <p className="px-2 py-3 text-center text-[10px] text-[#64748b]">{t("noMemories")}</p>
                      ) : (
                        cat.items.map((mem) => (
                          <div key={mem.id} className="group rounded-lg bg-[#1e1e2e]/50 p-2.5">
                            <p className="text-xs leading-relaxed text-[#94a3b8]">{mem.content.slice(0, 120)}{mem.content.length > 120 ? "…" : ""}</p>
                            <div className="mt-1.5 flex items-center justify-between">
                              <span className="text-[10px] text-[#64748b]">{mem.source}</span>
                              <button
                                onClick={() => {
                                  workspaceApi.deleteMemory(mem.id).then(() => {
                                    setMemories((prev) => prev.filter((m) => m.id !== mem.id));
                                  }).catch(() => {});
                                }}
                                className="text-[10px] text-[#64748b] opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100"
                              >
                                {t("delete")}
                              </button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <>
            {/* ---- Chat Header ---- */}
            {activeChatId && (
              <div className="flex items-center justify-between border-b border-[#1e1e2e] px-12 py-2.5">
                <div className="flex items-center gap-2 min-w-0">
                  <h3 className="truncate text-sm font-medium text-[#e2e8f0]">
                    {chats.find((c) => c.id === activeChatId)?.title || t("newChat")}
                  </h3>
                </div>
                <div className="flex items-center gap-1">
                  <button className="rounded p-1.5 text-[#64748b] transition-colors hover:bg-[#1e1e2e] hover:text-[#e2e8f0]" title={t("saveToMemory")}>
                    🧠
                  </button>
                </div>
              </div>
            )}

            {/* ---- Messages ---- */}
            <div ref={messagesScrollRef} className="flex-1 overflow-y-auto">
              {messages.length === 0 ? (
                <div className="flex h-full flex-col items-center justify-center px-4">
                  <div className="mb-4 text-5xl">✍️</div>
                  <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">{t("welcomeTitle")}</h2>
                  <p className="max-w-md text-center text-sm text-[#94a3b8]">{t("welcomeDesc")}</p>
                  {/* Quick action hint in empty state */}
                  <div className="mt-6 flex flex-wrap justify-center gap-2">
                    {[
                      { label: `🐦 ${t("quickTweet")}`, prompt: t("quickTweetPrompt") },
                      { label: `💬 ${t("quickReply")}`, prompt: t("quickReplyPrompt") },
                      { label: `📝 ${t("quickArticle")}`, prompt: t("quickArticlePrompt") },
                      { label: `📈 ${t("quickGrowth")}`, prompt: t("quickGrowthPrompt") },
                    ].map((qa) => (
                      <button
                        key={qa.label}
                        onClick={() => handleQuickAction(qa.prompt)}
                        className="rounded-lg border border-[#1e1e2e] px-3 py-2 text-xs text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                      >
                        {qa.label}
                      </button>
                    ))}
                  </div>
                </div>
              ) : (
                <div className="mx-auto max-w-3xl px-4 py-6 space-y-6">
                  {hasMoreMessages && (
                    <div className="flex justify-center pb-2">
                      <button
                        onClick={handleLoadMore}
                        disabled={loadingMore}
                        className="rounded-full border border-[#1e1e2e] px-4 py-1.5 text-xs text-[#64748b] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0] disabled:opacity-40"
                      >
                        {loadingMore ? (
                          <span className="inline-block h-3 w-3 animate-spin rounded-full border border-[#8b5cf6] border-t-transparent" />
                        ) : t("loadOlder")}
                      </button>
                    </div>
                  )}
                  {messages.map((msg) => {
                    const parsed = msg.role === "assistant" ? parseMemorySuggestion(msg.content) : null;
                    const displayContent = parsed?.body || msg.content;
                    const suggestion = parsed?.suggestion;
                    const showSuggestion = suggestion && !dismissedSuggestions.has(msg.id);
                    return (
                    <div key={msg.id} className="space-y-2">
                      <div className={`flex gap-3 ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                        {msg.role === "assistant" && (
                          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-xs text-[#8b5cf6]">✦</div>
                        )}
                      <div className={`max-w-[80%] ${msg.role === "assistant" ? "" : ""}`}>
                        {/* Context chips for AI messages */}
                        {msg.role === "assistant" && !msg.id.startsWith("error-") && (Boolean(memoryCatsByMsg[msg.id]?.length) || Boolean(soulEnhancedByMsg[msg.id])) && (
                          <div className="mb-1.5 flex flex-wrap gap-1">
                            {soulEnhancedByMsg[msg.id] && (
                              <span className="inline-flex items-center gap-0.5 rounded-full bg-[#06b6d4]/15 px-2 py-0.5 text-[10px] text-[#06b6d4]">
                                👁 {t("soulEnhanced", { handles: soulEnhancedByMsg[msg.id].map((h) => `@${h}`).join(", ") })}
                              </span>
                            )}
                            {MEMORY_CATEGORIES.filter((cat) => memoryCatsByMsg[msg.id]?.includes(cat.key)).map((cat) => (
                              <span
                                key={cat.key}
                                className="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[10px]"
                                style={{ backgroundColor: `${cat.color}15`, color: cat.color }}
                              >
                                {cat.icon} {t(`cat${cat.key.charAt(0).toUpperCase()}${cat.key.slice(1)}` as Parameters<typeof t>[0])}
                              </span>
                            ))}
                          </div>
                        )}
                        <div className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ${
                          msg.role === "user" ? "bg-[#8b5cf6] text-white" : "bg-[#1e1e2e] text-[#e2e8f0]"
                        }`}>
                          {msg.role === "assistant" ? (
                            (() => {
                              const variants = variantsByMsg[msg.id];
                              if (variants && variants.length > 0) {
                                return (
                                  <div className="space-y-3">
                                    {variants.map((v, i) => {
                                      const variantKey = `${msg.id}:${v.idx}`;
                                      return (
                                        <div
                                          key={variantKey}
                                          className={`rounded-xl border p-3 ${
                                            v.recommended
                                              ? "border-[#8b5cf6]/50 bg-[#8b5cf6]/5"
                                              : "border-[#2a2a3a] bg-[#0d0d14]"
                                          }`}
                                        >
                                          <div className="mb-1.5 flex items-center gap-2 text-[11px]">
                                            <span className={`font-semibold ${v.recommended ? "text-[#a78bfa]" : "text-[#94a3b8]"}`}>
                                              {v.recommended ? "✦ " : ""}{t("variantLabel", { letter: String.fromCharCode(65 + i) })}
                                            </span>
                                            {v.lang && (
                                              <span className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-[9px] uppercase tracking-wider text-[#64748b]">
                                                {v.lang}
                                              </span>
                                            )}
                                          </div>
                                          <div className="chat-markdown">
                                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{v.content}</ReactMarkdown>
                                          </div>
                                          {v.reason && (
                                            <p className="mt-1.5 border-t border-[#2a2a3a] pt-1.5 text-[10px] italic text-[#64748b]">
                                              → {v.reason}
                                            </p>
                                          )}
                                          {!msg.id.startsWith("temp-") && (
                                            <div className="mt-2 flex flex-wrap gap-1.5">
                                              <button
                                                onClick={() => copyToClipboard(v.content, variantKey)}
                                                className="rounded-md bg-[#1e1e2e] px-2 py-1 text-[10px] text-[#94a3b8] transition-colors hover:bg-[#2a2a3a] hover:text-[#e2e8f0]"
                                                title={t("copy")}
                                              >
                                                {copiedKey === variantKey ? `✓ ${t("copied")}` : `📋 ${t("copy")}`}
                                              </button>
                                              <button
                                                onClick={() => openOnTwitter(v.content)}
                                                className="rounded-md bg-[#1e1e2e] px-2 py-1 text-[10px] text-[#94a3b8] transition-colors hover:bg-[#2a2a3a] hover:text-[#1d9bf0]"
                                                title={t("openTwitter")}
                                              >
                                                🐦 {t("openTwitter")}
                                              </button>
                                              <button
                                                onClick={() => refineVariant(v.content)}
                                                className="rounded-md bg-[#1e1e2e] px-2 py-1 text-[10px] text-[#94a3b8] transition-colors hover:bg-[#2a2a3a] hover:text-[#a78bfa]"
                                                title={t("refine")}
                                              >
                                                ✨ {t("refine")}
                                              </button>
                                              <button
                                                onClick={() => handleSaveToArchive(msg, v.content)}
                                                disabled={archivedMsgs.has(msg.id)}
                                                className="rounded-md bg-[#1e1e2e] px-2 py-1 text-[10px] text-[#94a3b8] transition-colors hover:bg-[#2a2a3a] hover:text-[#f59e0b] disabled:opacity-50"
                                                title={t("saveToArchive")}
                                              >
                                                {archivedMsgs.has(msg.id) ? `✓ ${t("archived")}` : `📂 ${t("saveToArchive")}`}
                                              </button>
                                            </div>
                                          )}
                                        </div>
                                      );
                                    })}
                                  </div>
                                );
                              }
                              return (
                                <div className="chat-markdown"><ReactMarkdown remarkPlugins={[remarkGfm]}>{displayContent}</ReactMarkdown></div>
                              );
                            })()
                          ) : (
                            <p className="whitespace-pre-wrap">{msg.content}</p>
                          )}
                          {msg.role === "assistant" && !msg.id.startsWith("temp-") && !msg.id.startsWith("error-") && (
                            <div className="mt-2 flex items-center gap-1 border-t border-[#2a2a3a] pt-2 text-[#64748b]">
                              <button
                                onClick={() => handleFeedback(msg.id, msg.feedback === 1 ? 0 : 1)}
                                className={`rounded px-1.5 py-0.5 text-xs transition-colors ${
                                  msg.feedback === 1 ? "bg-emerald-500/20 text-emerald-400" : "hover:bg-[#2a2a3a]"
                                }`}
                                title="Helpful"
                              >
                                👍
                              </button>
                              <button
                                onClick={() => handleFeedback(msg.id, msg.feedback === -1 ? 0 : -1)}
                                className={`rounded px-1.5 py-0.5 text-xs transition-colors ${
                                  msg.feedback === -1 ? "bg-red-500/20 text-red-400" : "hover:bg-[#2a2a3a]"
                                }`}
                                title="Not helpful"
                              >
                                👎
                              </button>
                              {!variantsByMsg[msg.id] && (
                                <>
                                  <button
                                    onClick={() => copyToClipboard(displayContent, `msg:${msg.id}`)}
                                    className="rounded px-1.5 py-0.5 text-xs transition-colors hover:bg-[#2a2a3a]"
                                    title={t("copy")}
                                  >
                                    {copiedKey === `msg:${msg.id}` ? "✓" : "📋"}
                                  </button>
                                  <button
                                    onClick={() => handleSaveToArchive(msg)}
                                    disabled={archivedMsgs.has(msg.id)}
                                    className={`rounded px-1.5 py-0.5 text-xs transition-colors disabled:opacity-50 ${
                                      archivedMsgs.has(msg.id) ? "text-[#f59e0b]" : "hover:bg-[#2a2a3a]"
                                    }`}
                                    title={t("saveToArchive")}
                                  >
                                    {archivedMsgs.has(msg.id) ? "✓ 📂" : "📂"}
                                  </button>
                                </>
                              )}
                              {msg.scenario && msg.scenario !== "general" && (
                                <span className="ml-auto text-[10px] uppercase text-[#475569]">
                                  {msg.scenario}
                                </span>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                      </div>
                      {/* Memory Suggestion Card */}
                      {showSuggestion && (
                        <div className="ml-10 max-w-[70%] rounded-xl border border-[#f59e0b]/30 bg-[#f59e0b]/5 p-3">
                          <div className="mb-1.5 text-xs font-medium text-[#f59e0b]">🧠 {t("memorySuggest")}</div>
                          <p className="text-xs text-[#e2e8f0]">{suggestion.content}</p>
                          {suggestion.reason && <p className="mt-1 text-[10px] text-[#94a3b8]">→ {suggestion.reason}</p>}
                          <div className="mt-2 flex gap-2">
                            <button
                              onClick={() => handleSaveMemory(msg.id, suggestion)}
                              className="rounded-md bg-[#f59e0b]/20 px-3 py-1 text-xs font-medium text-[#f59e0b] transition-colors hover:bg-[#f59e0b]/30"
                            >
                              {t("saveTo")} {MEMORY_CATEGORIES.find(c => c.key === suggestion.category)?.icon || "📝"} {suggestion.category}
                            </button>
                            <button
                              onClick={() => handleSkipMemory(msg.id)}
                              className="rounded-md px-3 py-1 text-xs text-[#64748b] transition-colors hover:text-[#94a3b8]"
                            >
                              {t("skip")}
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  );
                  })}
                  {/* AI 思考中 placeholder */}
                  {sending && (
                    <div className="flex gap-3">
                      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-xs text-[#8b5cf6]">✦</div>
                      <div className="rounded-2xl bg-[#1e1e2e] px-4 py-3 text-sm text-[#64748b]">
                        <span className="inline-flex items-center gap-1">
                          <span className="inline-block h-1.5 w-1.5 animate-bounce rounded-full bg-[#64748b]" style={{ animationDelay: "0ms" }} />
                          <span className="inline-block h-1.5 w-1.5 animate-bounce rounded-full bg-[#64748b]" style={{ animationDelay: "150ms" }} />
                          <span className="inline-block h-1.5 w-1.5 animate-bounce rounded-full bg-[#64748b]" style={{ animationDelay: "300ms" }} />
                          {sendingElapsed >= 3 && (
                            <span className="ml-2 text-[10px] text-[#475569]">{sendingElapsed}s</span>
                          )}
                        </span>
                      </div>
                    </div>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* ---- Input Area ---- */}
            <div className="border-t border-[#1e1e2e] bg-[#0a0a0f] p-4">
              <div className="mx-auto max-w-3xl">
                {sending && sendingElapsed >= 5 && (
                  <p className="mb-2 text-center text-[10px] text-[#475569]">
                    {t("slowRequestHint", { seconds: sendingElapsed })}
                  </p>
                )}
                <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] focus-within:border-[#8b5cf6]">
                  <div className="flex items-end gap-3 p-3">
                    <textarea
                      ref={inputRef}
                      value={input}
                      onChange={(e) => setInput(e.target.value)}
                      onKeyDown={handleKeyDown}
                      placeholder={t("inputPlaceholder")}
                      rows={1}
                      className="flex-1 resize-none bg-transparent text-sm text-[#e2e8f0] placeholder-[#64748b] outline-none"
                      style={{ maxHeight: "120px" }}
                      disabled={sending}
                    />
                    <button
                      onClick={() => handleSend()}
                      disabled={!input.trim() || sending}
                      className="shrink-0 rounded-lg bg-[#8b5cf6] p-2 text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-30"
                    >
                      {sending ? (
                        <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                      ) : (
                        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M12 19V5m0 0l-7 7m7-7l7 7" />
                        </svg>
                      )}
                    </button>
                  </div>
                  {/* Input tools bar */}
                  <div className="flex items-center justify-between border-t border-[#1e1e2e]/50 px-3 py-1.5">
                    <div className="flex gap-1">
                      <button
                        onClick={() => {
                          setInput((prev) => {
                            const prefix = prev ? prev + "\n\n" : "";
                            return prefix + `[Tweet]\n\n[/Tweet]\n\n${t("pasteTweetSuffix")}`;
                          });
                          inputRef.current?.focus();
                          // 把光标定位到 [Tweet] 的内容区
                          requestAnimationFrame(() => {
                            const ta = inputRef.current;
                            if (ta) {
                              const pos = ta.value.indexOf("[Tweet]\n") + "[Tweet]\n".length;
                              ta.setSelectionRange(pos, pos);
                            }
                          });
                        }}
                        className="rounded px-2 py-1 text-[10px] text-[#64748b] transition-colors hover:bg-[#1e1e2e] hover:text-[#94a3b8]"
                      >
                        📎 {t("toolPaste")}
                      </button>
                      <button
                        onClick={() => {
                          setInput((prev) => prev + (prev ? "\n\n" : "") + t("translatePromptPrefix"));
                          inputRef.current?.focus();
                        }}
                        className="rounded px-2 py-1 text-[10px] text-[#64748b] transition-colors hover:bg-[#1e1e2e] hover:text-[#94a3b8]"
                      >
                        🔄 {t("toolTranslate")}
                      </button>
                      <button
                        onClick={() => {
                          const cats = memories.map((m) => `[${m.category}] ${m.content.slice(0, 60)}`).slice(0, 10);
                          if (cats.length === 0) {
                            setInput((prev) => prev + (prev ? "\n" : "") + t("noMemoriesToCite"));
                          } else {
                            setInput((prev) => prev + (prev ? "\n\n" : "") + t("citeMemoryPrefix") + "\n" + cats.join("\n"));
                          }
                          inputRef.current?.focus();
                        }}
                        className="rounded px-2 py-1 text-[10px] text-[#64748b] transition-colors hover:bg-[#1e1e2e] hover:text-[#94a3b8]"
                      >
                        🧠 {t("toolCiteMemory")}
                      </button>
                    </div>
                    <div className="text-[10px] text-[#64748b]">{t("inputHint")}</div>
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* ===== Right Panel ===== */}
      {rightPanelOpen && (
        <aside className="hidden w-[300px] shrink-0 flex-col border-l border-[#1e1e2e] bg-[#0d0d14] lg:flex">
          {/* Tabs */}
          <div className="flex border-b border-[#1e1e2e]">
            <button
              onClick={() => setRightTab("memory")}
              className={`flex-1 py-2.5 text-center text-xs font-medium transition-colors ${
                rightTab === "memory" ? "border-b-2 border-[#8b5cf6] text-[#e2e8f0]" : "text-[#64748b] hover:text-[#94a3b8]"
              }`}
            >
              {t("memory")}
            </button>
            <button
              onClick={() => setRightTab("context")}
              className={`flex-1 py-2.5 text-center text-xs font-medium transition-colors ${
                rightTab === "context" ? "border-b-2 border-[#8b5cf6] text-[#e2e8f0]" : "text-[#64748b] hover:text-[#94a3b8]"
              }`}
            >
              {t("context")}
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-3 space-y-4">
            {rightTab === "memory" ? (
              <>
                {/* Profile Summary */}
                {activeWs && (
                  <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                    <div className="mb-2 flex items-center gap-2">
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-sm font-bold text-[#8b5cf6]">
                        {activeWs.name?.charAt(0)?.toUpperCase() || "W"}
                      </div>
                      <div>
                        <div className="text-sm font-medium text-[#e2e8f0]">{activeWs.name}</div>
                        {activeWs.twitter_handle && (
                          <div className="text-xs text-[#64748b]">@{activeWs.twitter_handle}</div>
                        )}
                      </div>
                    </div>
                    {/* Profile memory snippet */}
                    {memoriesByCategory[0].items.length > 0 && (
                      <p className="text-xs leading-relaxed text-[#94a3b8]">
                        {memoriesByCategory[0].items[0].content.slice(0, 150)}{memoriesByCategory[0].items[0].content.length > 150 ? "…" : ""}
                      </p>
                    )}
                  </div>
                )}

                {/* Memory Categories */}
                {memoriesByCategory.slice(1).map((cat) => (
                  <div key={cat.key}>
                    <div className="mb-1.5 flex items-center justify-between">
                      <div className="flex items-center gap-1 text-xs font-medium" style={{ color: cat.color }}>
                        <span>{cat.icon}</span>
                        <span>{cat.label}</span>
                        <span className="ml-1 text-[#64748b]">{cat.items.length}</span>
                      </div>
                      <button
                        onClick={() => setAddingMemory({ cat: cat.key, content: "" })}
                        className="text-[10px] text-[#64748b] transition-colors hover:text-[#e2e8f0]"
                      >
                        + {t("add")}
                      </button>
                    </div>
                    {addingMemory?.cat === cat.key && (
                      <div className="mb-1.5">
                        <textarea
                          autoFocus
                          value={addingMemory.content}
                          onChange={(e) => setAddingMemory({ cat: cat.key, content: e.target.value })}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" && !e.shiftKey) {
                              e.preventDefault();
                              const content = addingMemory.content.trim();
                              if (content && activeWsId) {
                                workspaceApi.createMemory(activeWsId, cat.key, content)
                                  .then((mem) => {
                                    setMemories((prev) => [...prev, mem]);
                                  })
                                  .catch(() => { /* silent */ });
                              }
                              setAddingMemory(null);
                            } else if (e.key === "Escape") {
                              setAddingMemory(null);
                            }
                          }}
                          placeholder={t("addMemoryPrompt")}
                          rows={2}
                          className="w-full resize-none rounded-md bg-[#1e1e2e] px-2 py-1.5 text-[11px] text-[#e2e8f0] placeholder-[#64748b] outline-none focus:ring-1 focus:ring-[#8b5cf6]/50"
                        />
                        <div className="mt-1 flex justify-end gap-1">
                          <button onClick={() => setAddingMemory(null)} className="px-2 py-0.5 text-[10px] text-[#64748b] hover:text-[#e2e8f0]">{t("cancel")}</button>
                          <button
                            onClick={() => {
                              const content = addingMemory.content.trim();
                              if (content && activeWsId) {
                                workspaceApi.createMemory(activeWsId, cat.key, content)
                                  .then((mem) => {
                                    setMemories((prev) => [...prev, mem]);
                                  })
                                  .catch(() => { /* silent */ });
                              }
                              setAddingMemory(null);
                            }}
                            className="rounded-md bg-[#8b5cf6]/20 px-2 py-0.5 text-[10px] text-[#8b5cf6] hover:bg-[#8b5cf6]/30"
                          >{t("save")}</button>
                        </div>
                      </div>
                    )}
                    {cat.items.length > 0 ? (
                      <div className="space-y-1.5">
                        {cat.items.slice(0, 3).map((mem) => (
                          <div key={mem.id} className="rounded-lg bg-[#1e1e2e]/50 p-2.5">
                            <p className="text-[11px] leading-relaxed text-[#94a3b8]">
                              {mem.content.slice(0, 100)}{mem.content.length > 100 ? "…" : ""}
                            </p>
                          </div>
                        ))}
                        {cat.items.length > 3 && (
                          <button
                            onClick={() => setCurrentView("memory")}
                            className="w-full text-center text-[10px] text-[#64748b] hover:text-[#8b5cf6]"
                          >
                            +{cat.items.length - 3} {t("more")}
                          </button>
                        )}
                      </div>
                    ) : (
                      <p className="text-[10px] text-[#64748b]">{t("noMemories")}</p>
                    )}
                  </div>
                ))}
              </>
            ) : (
              /* Context tab - 展示本次对话实际用到的上下文汇总 */
              (() => {
                const usedSouls = Array.from(new Set(Object.values(soulEnhancedByMsg).flat()));
                const usedCatKeys = Array.from(new Set(Object.values(memoryCatsByMsg).flat()));
                const usedCats = MEMORY_CATEGORIES.filter((c) => usedCatKeys.includes(c.key));
                const usedMethodologies = Array.from(new Set(Object.values(methodologySlugsByMsg).flat()));
                const usedLangs = Array.from(new Set(Object.values(outputLangsByMsg).flat()));
                const totalCredits = messages.reduce((sum, m) => sum + (m.credits_cost ?? 0), 0);
                const hasContext =
                  usedSouls.length > 0 ||
                  usedCats.length > 0 ||
                  usedMethodologies.length > 0 ||
                  usedLangs.length > 0;
                return (
                  <div className="space-y-4">
                    {/* Stats row */}
                    <div className="grid grid-cols-2 gap-2">
                      <div className="rounded-lg bg-[#1e1e2e]/50 p-2.5 text-center">
                        <div className="text-lg font-bold text-[#e2e8f0]">{messages.length}</div>
                        <div className="mt-0.5 text-[10px] text-[#64748b]">{t("ctxMessages")}</div>
                      </div>
                      <div className="rounded-lg bg-[#1e1e2e]/50 p-2.5 text-center">
                        <div className="text-lg font-bold text-[#e2e8f0]">{totalCredits}</div>
                        <div className="mt-0.5 text-[10px] text-[#64748b]">{t("ctxCredits")}</div>
                      </div>
                    </div>

                    {!hasContext && messages.length === 0 ? (
                      <div className="flex flex-col items-center justify-center py-8 text-center">
                        <div className="mb-2 text-2xl">📋</div>
                        <p className="text-xs font-medium text-[#64748b]">{t("ctxNoContext")}</p>
                        <p className="mt-1 text-[10px] text-[#475569]">{t("ctxStart")}</p>
                      </div>
                    ) : (
                      <>
                        {/* Soul context */}
                        {usedSouls.length > 0 && (
                          <div>
                            <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#64748b]">{t("ctxSoulUsed")}</div>
                            <div className="flex flex-wrap gap-1.5">
                              {usedSouls.map((h) => (
                                <span key={h} className="inline-flex items-center gap-1 rounded-full bg-[#06b6d4]/15 px-2.5 py-1 text-xs text-[#06b6d4]">
                                  👁 @{h}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Memory categories */}
                        {usedCats.length > 0 && (
                          <div>
                            <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#64748b]">{t("ctxMemUsed")}</div>
                            <div className="flex flex-wrap gap-1.5">
                              {usedCats.map((cat) => (
                                <span
                                  key={cat.key}
                                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs"
                                  style={{ backgroundColor: `${cat.color}15`, color: cat.color }}
                                >
                                  {cat.icon} {t(`cat${cat.key.charAt(0).toUpperCase()}${cat.key.slice(1)}` as Parameters<typeof t>[0])}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Methodology slugs */}
                        {usedMethodologies.length > 0 && (
                          <div>
                            <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#64748b]">{t("ctxMethodology")}</div>
                            <div className="flex flex-wrap gap-1.5">
                              {usedMethodologies.map((slug) => (
                                <span
                                  key={slug}
                                  className="inline-flex items-center gap-1 rounded-full bg-[#a78bfa]/15 px-2.5 py-1 text-[11px] text-[#a78bfa]"
                                  title={slug}
                                >
                                  📚 {slug}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Output languages */}
                        {usedLangs.length > 0 && (
                          <div>
                            <div className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-[#64748b]">{t("ctxOutputLangs")}</div>
                            <div className="flex flex-wrap gap-1.5">
                              {usedLangs.map((lang) => (
                                <span
                                  key={lang}
                                  className="inline-flex items-center gap-1 rounded-full bg-[#10b981]/15 px-2.5 py-1 text-[11px] uppercase text-[#10b981]"
                                >
                                  🌐 {lang}
                                </span>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Empty context but messages exist */}
                        {!hasContext && messages.length > 0 && (
                          <p className="text-center text-[11px] text-[#475569]">{t("ctxNoContext")}</p>
                        )}
                      </>
                    )}
                  </div>
                );
              })()
            )}
          </div>
        </aside>
      )}

      {/* Upgrade modal */}
      <UpgradeModal open={upgradeOpen} onClose={() => setUpgradeOpen(false)} reason={upgradeReason} />

      {/* Smart Import dialog */}
      {importOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => { if (!importing) setImportOpen(false); }}
        >
          <div
            className="mx-4 w-full max-w-2xl rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-4 flex items-start justify-between gap-3">
              <div>
                <h3 className="text-lg font-bold text-[#e2e8f0]">📥 {t("smartImportTitle")}</h3>
                <p className="mt-1 text-xs text-[#94a3b8]">{t("smartImportDesc")}</p>
              </div>
              <button
                onClick={() => { if (!importing) setImportOpen(false); }}
                className="text-[#64748b] hover:text-[#e2e8f0]"
                aria-label="Close"
              >
                ×
              </button>
            </div>

            <div
              className={`relative rounded-lg ring-1 transition-colors ${
                importDragOver
                  ? "ring-[#8b5cf6]/70 ring-2"
                  : "ring-[#1e1e2e] focus-within:ring-[#8b5cf6]/50"
              }`}
              onDragOver={(e) => {
                e.preventDefault();
                e.stopPropagation();
                if (!importDragOver) setImportDragOver(true);
              }}
              onDragLeave={(e) => {
                e.preventDefault();
                e.stopPropagation();
                setImportDragOver(false);
              }}
              onDrop={(e) => {
                e.preventDefault();
                e.stopPropagation();
                setImportDragOver(false);
                if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                  void loadImportFiles(e.dataTransfer.files);
                }
              }}
            >
              <textarea
                autoFocus
                value={importText}
                onChange={(e) => setImportText(e.target.value)}
                placeholder={t("smartImportPlaceholder")}
                rows={10}
                maxLength={20000}
                className="w-full resize-none rounded-lg bg-[#0d0d14] px-3 py-2.5 text-sm text-[#e2e8f0] placeholder-[#64748b] outline-none"
              />
              {importDragOver && (
                <div className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-lg bg-[#8b5cf6]/10 text-sm font-medium text-[#a78bfa]">
                  📂 {t("smartImportDropHere")}
                </div>
              )}
            </div>
            <input
              ref={importFileInputRef}
              type="file"
              accept=".md,.markdown,.txt,text/plain,text/markdown"
              multiple
              hidden
              onChange={(e) => {
                if (e.target.files && e.target.files.length > 0) {
                  void loadImportFiles(e.target.files);
                }
                // Reset so the same file can be selected again
                e.target.value = "";
              }}
            />
            <div className="mt-1 flex items-center justify-between text-[10px]">
              <button
                type="button"
                onClick={() => importFileInputRef.current?.click()}
                className="flex items-center gap-1 text-[#64748b] transition-colors hover:text-[#a78bfa]"
              >
                <span>📎</span>
                <span>{t("smartImportUpload")}</span>
              </button>
              <span className={importText.length > 18000 ? "text-amber-400" : "text-[#64748b]"}>
                {importText.length} / 20000
              </span>
            </div>

            <div className="mt-3 flex flex-wrap items-center gap-3 text-xs text-[#94a3b8]">
              <label className="flex cursor-pointer items-center gap-1.5">
                <input
                  type="radio"
                  name="import-mode"
                  checked={importMode === "review"}
                  onChange={() => setImportMode("review")}
                  className="accent-[#8b5cf6]"
                />
                <span>{t("smartImportReview")}</span>
              </label>
              <label className="flex cursor-pointer items-center gap-1.5">
                <input
                  type="radio"
                  name="import-mode"
                  checked={importMode === "auto-accept"}
                  onChange={() => setImportMode("auto-accept")}
                  className="accent-[#8b5cf6]"
                />
                <span>{t("smartImportAuto")}</span>
              </label>
            </div>

            {importError && (
              <div className="mt-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-300">
                {importError}
              </div>
            )}

            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={() => setImportOpen(false)}
                disabled={importing}
                className="rounded-lg border border-[#2a2a3a] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] disabled:opacity-50"
              >
                {t("cancel")}
              </button>
              <button
                onClick={handleSmartImport}
                disabled={importing || importText.trim().length === 0}
                className="flex items-center gap-1.5 rounded-lg bg-[#8b5cf6] px-4 py-1.5 text-xs font-medium text-white hover:bg-[#7c3aed] disabled:cursor-not-allowed disabled:opacity-50"
              >
                {importing && (
                  <span className="h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />
                )}
                <span>{importing ? t("smartImportImporting") : t("smartImportStart")}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Refresh self-portrait dialog */}
      {refreshOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => { if (!refreshing) setRefreshOpen(false); }}
        >
          <div
            className="mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-3 flex items-start justify-between gap-3">
              <h3 className="text-lg font-bold text-[#e2e8f0]">🔄 {t("refreshSelfPortraitTitle")}</h3>
              <button
                onClick={() => { if (!refreshing) setRefreshOpen(false); }}
                className="text-[#64748b] hover:text-[#e2e8f0]"
                aria-label="Close"
              >×</button>
            </div>
            <p className="mb-3 text-xs text-[#94a3b8]">
              {t("refreshSelfPortraitDesc", { handle: activeWs?.twitter_handle ?? "" })}
            </p>
            <p className="mb-4 text-[11px] text-amber-300">
              {t("importTwitterCostHint")}
            </p>
            {user?.is_pro && (
              <label className="mb-4 flex cursor-pointer items-center gap-2 text-xs text-[#94a3b8]">
                <input
                  type="checkbox"
                  checked={refreshAutoAccept}
                  onChange={(e) => setRefreshAutoAccept(e.target.checked)}
                  className="accent-[#8b5cf6]"
                />
                <span>{t("importTwitterAutoAcceptLabel")}</span>
              </label>
            )}
            {refreshError && (
              <div className="mb-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-300">
                {refreshError}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setRefreshOpen(false)}
                disabled={refreshing}
                className="rounded-lg border border-[#2a2a3a] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] disabled:opacity-50"
              >{t("cancel")}</button>
              <button
                onClick={handleRefreshSelfPortrait}
                disabled={refreshing}
                className="flex items-center gap-1.5 rounded-lg bg-sky-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-sky-500 disabled:opacity-50"
              >
                {refreshing && <span className="h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />}
                <span>{refreshing ? t("smartImportImporting") : t("refreshSelfPortraitBtn")}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Import-from-Twitter dialog */}
      {twImportOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => { if (!twImporting) setTwImportOpen(false); }}
        >
          <div
            className="mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-3 flex items-start justify-between gap-3">
              <h3 className="text-lg font-bold text-[#e2e8f0]">🐦 {t("importTwitterTitle")}</h3>
              <button
                onClick={() => { if (!twImporting) setTwImportOpen(false); }}
                className="text-[#64748b] hover:text-[#e2e8f0]"
                aria-label="Close"
              >×</button>
            </div>
            <p className="mb-3 text-xs text-[#94a3b8]">{t("importTwitterDesc")}</p>
            <input
              autoFocus
              value={twImportHandle}
              onChange={(e) => setTwImportHandle(e.target.value)}
              placeholder={t("importTwitterPlaceholder")}
              maxLength={300}
              onKeyDown={(e) => { if (e.key === "Enter" && !twImporting) void handleTwitterImport(); }}
              className="mb-2 w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] placeholder-[#64748b] outline-none focus:border-emerald-500/60"
            />
            {(() => {
              const parsed = extractTwitterHandle(twImportHandle);
              if (!twImportHandle.trim()) return null;
              return parsed ? (
                <p className="mb-3 text-[11px] text-emerald-400">→ @{parsed}</p>
              ) : (
                <p className="mb-3 text-[11px] text-red-400">{t("importTwitterInvalidHandle")}</p>
              );
            })()}
            <p className="mb-4 text-[11px] text-amber-300">{t("importTwitterCostHint")}</p>
            {user?.is_pro && (
              <label className="mb-4 flex cursor-pointer items-center gap-2 text-xs text-[#94a3b8]">
                <input
                  type="checkbox"
                  checked={twImportAutoAccept}
                  onChange={(e) => setTwImportAutoAccept(e.target.checked)}
                  className="accent-[#8b5cf6]"
                />
                <span>{t("importTwitterAutoAcceptLabel")}</span>
              </label>
            )}
            {twImportError && (
              <div className="mb-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-300">
                {twImportError}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setTwImportOpen(false)}
                disabled={twImporting}
                className="rounded-lg border border-[#2a2a3a] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e] disabled:opacity-50"
              >{t("cancel")}</button>
              <button
                onClick={handleTwitterImport}
                disabled={twImporting || twImportHandle.trim().length === 0}
                className="flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {twImporting && <span className="h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />}
                <span>{twImporting ? t("smartImportImporting") : t("importTwitterBtn")}</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}


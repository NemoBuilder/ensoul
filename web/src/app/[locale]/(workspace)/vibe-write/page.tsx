"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import {
  workspaceApi,
  emailAuthApi,
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
        setActiveChatId(chatId);
      }

      if (!prefill) setInput("");
      setCurrentView("chat");

      const tempUserMsg: VibeChatMsg = {
        id: `temp-${Date.now()}`, chat_id: chatId, role: "user",
        content: text, credits_cost: 0, created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, tempUserMsg]);

      const result = await workspaceApi.sendMessage(chatId, text);
      setMessages((prev) => [
        ...prev.filter((m) => m.id !== tempUserMsg.id),
        result.user_message, result.assistant_message,
      ]);

      if (result.soul_enhanced && result.soul_handles && result.soul_handles.length > 0) {
        setSoulEnhancedByMsg((prev) => ({
          ...prev,
          [result.assistant_message.id]: result.soul_handles || [],
        }));
      }
      if (result.memory_cats && result.memory_cats.length > 0) {
        setMemoryCatsByMsg((prev) => ({
          ...prev,
          [result.assistant_message.id]: result.memory_cats || [],
        }));
      }

      setUser((prev) => {
        if (!prev) return prev;
        return { ...prev, credits: prev.credits - result.credits_used };
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
      if (msg.includes("credits") || msg.includes("insufficient")) {
        showUpgrade("credits");
      } else if (msg.includes("workspace limit")) {
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
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      if (msg.includes("Pro") || msg.includes("category")) {
        showUpgrade("memory");
      }
    }
  }

  function handleSkipMemory(msgId: string) {
    setDismissedSuggestions((prev) => new Set(prev).add(msgId));
  }

  // Login modal (for unauthenticated CTA)
  const [addingMemory, setAddingMemory] = useState<{ cat: string; content: string } | null>(null);
  const [loginOpen, setLoginOpen] = useState(false);

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

  // Helper: group memories by category
  const memoriesByCategory = MEMORY_CATEGORIES.map((cat) => ({
    ...cat,
    label: t(`cat${cat.key.charAt(0).toUpperCase()}${cat.key.slice(1)}` as Parameters<typeof t>[0]),
    items: memories.filter((m) => m.category === cat.key),
  }));

  async function handleSetupCreate() {
    if (!setupName.trim()) return;
    setSetupCreating(true);
    try {
      const handle = setupHandle.trim().replace(/^@/, "");
      const ws = await workspaceApi.create(setupName.trim(), handle || undefined);
      setWorkspaces([ws]);
      setActiveWsId(ws.id);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "";
      if (msg.includes("workspace limit")) showUpgrade("workspace");
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
          {/* Workspace Switcher */}
          <div className="border-b border-[#1e1e2e] p-3">
            <div
              className="relative flex cursor-pointer items-center gap-2.5 rounded-lg bg-[#1e1e2e]/50 px-3 py-2 transition-colors hover:bg-[#1e1e2e]"
              onClick={() => setWsDropdownOpen(!wsDropdownOpen)}
            >
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-sm font-bold text-[#8b5cf6]">
                {activeWs?.name?.charAt(0)?.toUpperCase() || "✦"}
              </div>
              <div className="flex-1 min-w-0">
                <div className="truncate text-sm font-medium text-[#e2e8f0]">{activeWs?.name || t("welcomeTitle")}</div>
                {activeWs?.twitter_handle && (
                  <div className="truncate text-xs text-[#64748b]">@{activeWs.twitter_handle}</div>
                )}
              </div>
              <span className="text-xs text-[#64748b]">{wsDropdownOpen ? "▴" : "▾"}</span>
            </div>

            {/* Workspace dropdown */}
            {wsDropdownOpen && workspaces.length > 1 && (
              <div className="mt-1 rounded-lg border border-[#1e1e2e] bg-[#14141f] py-1 shadow-xl">
                {workspaces.map((ws) => (
                  <button
                    key={ws.id}
                    onClick={() => { setActiveWsId(ws.id); setWsDropdownOpen(false); setActiveChatId(null); setMessages([]); }}
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

          {/* Sidebar Footer: Memory / Settings */}
          <div className="border-t border-[#1e1e2e] p-2">
            <div className="flex">
              <button
                onClick={() => setCurrentView(currentView === "memory" ? "chat" : "memory")}
                className={`flex-1 rounded-lg py-2 text-center text-xs transition-colors ${
                  currentView === "memory" ? "bg-[#1e1e2e] text-[#e2e8f0]" : "text-[#64748b] hover:text-[#94a3b8]"
                }`}
              >
                🧠 {t("memory")}
              </button>
              <button className="flex-1 rounded-lg py-2 text-center text-xs text-[#64748b] hover:text-[#94a3b8]">
                ⚙️ {t("settings")}
              </button>
            </div>
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
              <div className="mb-6 flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-[#e2e8f0]">🧠 {t("memoryManager")}</h2>
                  <p className="mt-0.5 text-xs text-[#64748b]">{activeWs?.name}</p>
                </div>
              </div>

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
                        onClick={() => {
                          const isPro = user?.is_pro;
                          if (!isPro && !["profile", "rules"].includes(cat.key)) {
                            showUpgrade("memory");
                            return;
                          }
                          setAddingMemory({ cat: cat.key, content: "" });
                        }}
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
                                  .then((mem) => setMemories((prev) => [...prev, mem]))
                                  .catch((err: unknown) => {
                                    const msg = err instanceof Error ? err.message : "";
                                    if (msg.includes("Pro") || msg.includes("category")) showUpgrade("memory");
                                  });
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
                                  .then((mem) => setMemories((prev) => [...prev, mem]))
                                  .catch((err: unknown) => {
                                    const msg = err instanceof Error ? err.message : "";
                                    if (msg.includes("Pro") || msg.includes("category")) showUpgrade("memory");
                                  });
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
                            <div className="chat-markdown"><ReactMarkdown remarkPlugins={[remarkGfm]}>{displayContent}</ReactMarkdown></div>
                          ) : (
                            <p className="whitespace-pre-wrap">{msg.content}</p>
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
                        onClick={() => {
                          if (!user?.is_pro && !["profile", "rules"].includes(cat.key)) {
                            showUpgrade("memory");
                            return;
                          }
                          setAddingMemory({ cat: cat.key, content: "" });
                        }}
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
                                  .then((mem) => setMemories((prev) => [...prev, mem]))
                                  .catch((err: unknown) => {
                                    const msg = err instanceof Error ? err.message : "";
                                    if (msg.includes("Pro") || msg.includes("category")) showUpgrade("memory");
                                  });
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
                                  .then((mem) => setMemories((prev) => [...prev, mem]))
                                  .catch((err: unknown) => {
                                    const msg = err instanceof Error ? err.message : "";
                                    if (msg.includes("Pro") || msg.includes("category")) showUpgrade("memory");
                                  });
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
                const totalCredits = messages.reduce((sum, m) => sum + (m.credits_cost ?? 0), 0);
                const hasContext = usedSouls.length > 0 || usedCats.length > 0;
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
    </div>
  );
}


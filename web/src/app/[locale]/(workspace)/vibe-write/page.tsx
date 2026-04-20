"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useTranslations } from "next-intl";
import {
  workspaceApi,
  emailAuthApi,
  type Workspace,
  type VibeChatItem,
  type VibeChatMsg,
  type EmailSessionInfo,
} from "@/lib/api";
import UpgradeModal from "@/components/UpgradeModal";
import LoginModal from "@/components/LoginModal";

export default function VibeWritePage() {
  const t = useTranslations("VibeWrite2");

  // Auth
  const [user, setUser] = useState<EmailSessionInfo | null>(null);
  const [authChecked, setAuthChecked] = useState(false);

  // Workspace
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [activeWsId, setActiveWsId] = useState<string | null>(null);

  // Chats
  const [chats, setChats] = useState<VibeChatItem[]>([]);
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [messages, setMessages] = useState<VibeChatMsg[]>([]);

  // UI state
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeReason, setUpgradeReason] = useState<"credits" | "workspace" | "memory" | "feature">("feature");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

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

  // Re-check auth on window focus (catches login from layout header)
  useEffect(() => {
    function onFocus() {
      if (!user) checkAuth();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [user, checkAuth]);

  // Listen for auth changes from layout (login/logout via header)
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
    workspaceApi.list().then((res) => {
      setWorkspaces(res.workspaces || []);
      if (res.workspaces?.length > 0 && !activeWsId) {
        setActiveWsId(res.workspaces[0].id);
      }
    }).catch(() => {});
  }, [user]);

  // Load chats
  useEffect(() => {
    if (!activeWsId) { setChats([]); setActiveChatId(null); return; }
    workspaceApi.listChats(activeWsId).then((res) => {
      setChats(res.chats || []);
    }).catch(() => {});
  }, [activeWsId]);

  // Load messages
  useEffect(() => {
    if (!activeChatId) { setMessages([]); return; }
    workspaceApi.getMessages(activeChatId).then((res) => {
      setMessages(res.messages || []);
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

  // Send message
  async function handleSend() {
    if (!input.trim() || sending) return;
    setSending(true);
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

      const content = input.trim();
      setInput("");

      const tempUserMsg: VibeChatMsg = {
        id: `temp-${Date.now()}`, chat_id: chatId, role: "user",
        content, credits_cost: 0, created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, tempUserMsg]);

      const result = await workspaceApi.sendMessage(chatId, content);
      setMessages((prev) => [
        ...prev.filter((m) => m.id !== tempUserMsg.id),
        result.user_message, result.assistant_message,
      ]);

      if (user) setUser({ ...user, credits: user.credits - result.credits_used });

      setChats((prev) =>
        prev.map((c) =>
          c.id === chatId
            ? { ...c, title: content.slice(0, 50) + (content.length > 50 ? "..." : ""), updated_at: new Date().toISOString() }
            : c
        )
      );
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to send";
      // Detect feature gate errors
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
      setSending(false);
      inputRef.current?.focus();
    }
  }

  function handleNewChat() {
    setActiveChatId(null);
    setMessages([]);
    setInput("");
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

  // Login modal (for unauthenticated CTA)
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
        <aside className="flex w-64 shrink-0 flex-col border-r border-[#1e1e2e] bg-[#0d0d14]">
          <div className="p-3">
            <div className="flex w-full items-center justify-center gap-2 rounded-lg border border-[#1e1e2e] px-3 py-2.5 text-sm text-[#94a3b8]">
              <span>+</span>
              <span>{t("newChat")}</span>
            </div>
          </div>
          <div className="flex-1 px-2 space-y-0.5">
            {[
              { id: "d1", title: t("demoChat1") },
              { id: "d2", title: t("demoChat2") },
              { id: "d3", title: t("demoChat3") },
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
          <div className="border-t border-[#1e1e2e] p-3">
            <div className="flex items-center justify-between text-xs text-[#64748b]">
              <span>{t("credits")}</span>
              <span className="font-mono text-[#94a3b8]">50</span>
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
                <div key={i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div className={`max-w-[85%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
                    msg.role === "user" ? "bg-[#8b5cf6] text-white" : "bg-[#1e1e2e] text-[#e2e8f0]"
                  }`}>
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* CTA bar — replaces input area */}
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

        <LoginModal
          open={loginOpen}
          onClose={() => setLoginOpen(false)}
          onLogin={(u) => { setUser(u); setLoginOpen(false); window.dispatchEvent(new Event("ensoul:auth-changed")); }}
        />
      </div>
    );
  }

  return (
    <div className="flex h-full">
      {/* Sidebar */}
      {sidebarOpen && (
        <aside className="flex w-64 shrink-0 flex-col border-r border-[#1e1e2e] bg-[#0d0d14]">
          <div className="p-3">
            <button
              onClick={handleNewChat}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-[#1e1e2e] px-3 py-2.5 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
            >
              <span>+</span>
              <span>{t("newChat")}</span>
            </button>
          </div>
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
                    onClick={() => setActiveChatId(chat.id)}
                  >
                    <span className="flex-1 truncate">{chat.title || "New Chat"}</span>
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
          {user && (
            <div className="border-t border-[#1e1e2e] p-3">
              <div className="flex items-center justify-between text-xs text-[#64748b]">
                <span>{t("credits")}</span>
                <span className="font-mono text-[#94a3b8]">{user.credits}</span>
              </div>
            </div>
          )}
        </aside>
      )}

      {/* Main area */}
      <div className="flex flex-1 flex-col relative">
        <button
          onClick={() => setSidebarOpen(!sidebarOpen)}
          className="absolute left-0 top-2 z-10 rounded-r-md border border-l-0 border-[#1e1e2e] bg-[#14141f] px-1.5 py-2 text-[#64748b] hover:text-[#e2e8f0]"
        >
          {sidebarOpen ? "◂" : "▸"}
        </button>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto">
          {messages.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center px-4">
              <div className="mb-4 text-5xl">✍️</div>
              <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">{t("welcomeTitle")}</h2>
              <p className="max-w-md text-center text-sm text-[#94a3b8]">{t("welcomeDesc")}</p>
            </div>
          ) : (
            <div className="mx-auto max-w-3xl px-4 py-6 space-y-6">
              {messages.map((msg) => (
                <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div className={`max-w-[85%] rounded-2xl px-4 py-3 text-sm leading-relaxed ${
                    msg.role === "user" ? "bg-[#8b5cf6] text-white" : "bg-[#1e1e2e] text-[#e2e8f0]"
                  }`}>
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                  </div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* Input */}
        <div className="border-t border-[#1e1e2e] bg-[#0a0a0f] p-4">
          <div className="mx-auto max-w-3xl">
            <div className="flex items-end gap-3 rounded-xl border border-[#1e1e2e] bg-[#14141f] p-3 focus-within:border-[#8b5cf6]">
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
                onClick={handleSend}
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
            <p className="mt-2 text-center text-xs text-[#64748b]">{t("inputHint")}</p>
          </div>
        </div>
      </div>

      {/* Upgrade modal */}
      <UpgradeModal open={upgradeOpen} onClose={() => setUpgradeOpen(false)} reason={upgradeReason} />
    </div>
  );
}


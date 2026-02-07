"use client";

import { useState, useRef, useEffect, use } from "react";
import Link from "next/link";
import { chatApi, shellApi, Shell } from "@/lib/api";
import { stageConfig, Stage } from "@/lib/utils";

interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

export default function ChatPage({
  params,
}: {
  params: Promise<{ handle: string }>;
}) {
  const { handle } = use(params);
  const [shell, setShell] = useState<Shell | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  // Load shell info
  useEffect(() => {
    shellApi.get(handle).then(setShell).catch(() => {});
  }, [handle]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  // Send message with SSE streaming
  async function sendMessage() {
    const text = input.trim();
    if (!text || streaming) return;

    setInput("");
    setError("");
    setStreaming(true);

    // Add user message
    const userMsg: ChatMessage = { role: "user", content: text };
    setMessages((prev) => [...prev, userMsg]);

    // Add empty assistant message to stream into
    setMessages((prev) => [...prev, { role: "assistant", content: "" }]);

    try {
      const res = await chatApi.sendMessage(handle, text);

      if (!res.ok) {
        const errData = await res.json().catch(() => ({ error: "Chat failed" }));
        throw new Error(errData.error || `HTTP ${res.status}`);
      }

      const reader = res.body?.getReader();
      const decoder = new TextDecoder();

      if (!reader) throw new Error("No response stream");

      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const data = line.slice(6);
            if (data === "[DONE]") continue;
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

  const stage = shell ? stageConfig[shell.stage as Stage] || stageConfig.embryo : stageConfig.embryo;

  return (
    <div className="flex h-screen flex-col pt-16">
      {/* Chat header */}
      <div className="border-b border-[#1e1e2e] bg-[#14141f] px-4 py-3">
        <div className="mx-auto flex max-w-3xl items-center justify-between">
          <div className="flex items-center gap-3">
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
          <div className="flex items-center gap-2">
            <div className={`h-2 w-2 rounded-full ${shell?.stage === "embryo" ? "bg-gray-500" : "bg-green-500"}`} />
            <span className="text-xs text-[#94a3b8]">
              {shell?.stage === "embryo" ? "Embryo" : "Online"}
            </span>
          </div>
        </div>
      </div>

      {/* Messages area */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-y-auto px-4 py-6"
      >
        <div className="mx-auto max-w-3xl space-y-4">
          {messages.length === 0 && (
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
            </div>
          )}

          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[80%] rounded-lg px-4 py-3 text-sm leading-relaxed ${
                  msg.role === "user"
                    ? "bg-[#8b5cf6] text-white"
                    : "border border-[#1e1e2e] bg-[#14141f] text-[#e2e8f0]"
                }`}
              >
                {msg.content || (
                  <span className="inline-flex gap-1">
                    <span className="animate-pulse">●</span>
                    <span className="animate-pulse" style={{ animationDelay: "0.2s" }}>●</span>
                    <span className="animate-pulse" style={{ animationDelay: "0.4s" }}>●</span>
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="border-t border-red-500/30 bg-red-500/5 px-4 py-2 text-center text-xs text-red-400">
          {error}
        </div>
      )}

      {/* Input area */}
      <div className="border-t border-[#1e1e2e] bg-[#14141f] px-4 py-4">
        <div className="mx-auto flex max-w-3xl gap-3">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && sendMessage()}
            placeholder={`Message @${handle}...`}
            className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none focus:border-[#8b5cf6]"
            disabled={streaming}
          />
          <button
            onClick={sendMessage}
            disabled={streaming || !input.trim()}
            className="rounded-lg bg-[#8b5cf6] px-5 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  );
}

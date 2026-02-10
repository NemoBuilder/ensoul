"use client";

import { useState, useEffect, use, useRef } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { shareApi, ChatShareData, ChatShareMessage, statsApi, GlobalStats } from "@/lib/api";
import { stageConfig, Stage } from "@/lib/utils";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export default function SharePage({
  params,
}: {
  params: Promise<{ code: string }>;
}) {
  const { code } = use(params);
  const t = useTranslations("Share");
  const [share, setShare] = useState<ChatShareData | null>(null);
  const [messages, setMessages] = useState<ChatShareMessage[]>([]);
  const [stats, setStats] = useState<GlobalStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [imgErr, setImgErr] = useState(false);
  const cardRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    async function load() {
      try {
        const [data, globalStats] = await Promise.all([
          shareApi.get(code),
          statsApi.global().catch(() => null),
        ]);
        setShare(data);
        setStats(globalStats);
        // Parse messages JSON string
        try {
          const parsed = JSON.parse(data.messages);
          setMessages(Array.isArray(parsed) ? parsed : []);
        } catch {
          setMessages([]);
        }
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : "Share not found");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [code]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center pt-16">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
      </div>
    );
  }

  if (error || !share) {
    return (
      <div className="mx-auto max-w-2xl px-4 pt-24 pb-16 text-center">
        <div className="mb-4 text-5xl">🔗</div>
        <h2 className="mb-2 text-2xl font-bold text-[#e2e8f0]">{t("notFound")}</h2>
        <p className="mb-6 text-[#94a3b8]">{t("notFoundDesc")}</p>
        <Link href="/explore" className="text-[#8b5cf6] hover:underline">
          {t("exploreSouls")}
        </Link>
      </div>
    );
  }

  const stage = stageConfig[(share.stage as Stage)] || stageConfig.embryo;

  return (
    <div className="mx-auto max-w-2xl px-4 pt-20 pb-16">
      {/* ═══ Share Card ═══ */}
      <div
        ref={cardRef}
        className="overflow-hidden rounded-2xl border border-[#1e1e2e] bg-[#14141f]"
      >
        {/* Header — Soul identity */}
        <div className="flex items-center gap-3 border-b border-[#1e1e2e] px-5 py-4">
          <div className="relative h-12 w-12 overflow-hidden rounded-full border-2 border-[#1e1e2e] bg-[#0a0a0f]">
            {share.avatar_url && !imgErr ? (
              <img
                src={share.avatar_url}
                alt={share.handle}
                className="h-full w-full object-cover"
                onError={() => setImgErr(true)}
              />
            ) : (
              <div className="flex h-full w-full items-center justify-center text-lg text-[#8b5cf6]">
                🧠
              </div>
            )}
          </div>
          <div className="flex-1">
            <h2 className="font-semibold text-[#e2e8f0]">
              @{share.handle}
            </h2>
            <span className={`text-xs ${stage.textClass}`}>
              {stage.label} · DNA v{share.dna_version}
            </span>
          </div>
          <div className="text-right">
            <div className="text-xs text-[#94a3b8]">{t("sharedConversation")}</div>
          </div>
        </div>

        {/* Messages */}
        <div className="space-y-4 px-5 py-5">
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              {msg.role === "user" ? (
                <div className="max-w-[80%] rounded-2xl rounded-br-sm bg-[#8b5cf6] px-4 py-3 text-sm leading-relaxed text-white whitespace-pre-wrap">
                  {msg.content}
                </div>
              ) : (
                <div className="max-w-[85%] rounded-2xl rounded-bl-sm border border-[#1e1e2e] bg-[#0a0a0f] px-5 py-4 text-sm text-[#e2e8f0]">
                  <div className="chat-markdown">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {msg.content}
                    </ReactMarkdown>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Watermark */}
        <div className="border-t border-[#1e1e2e] px-5 py-3 text-center">
          <span className="text-xs text-[#94a3b8]/60">
            ensoul.ac/s/{share.code}
          </span>
        </div>
      </div>

      {/* ═══ CTA Section ═══ */}
      <div className="mt-6 space-y-3">
        {/* Primary CTA */}
        <Link
          href={`/soul/${share.handle}/chat`}
          className="flex w-full items-center justify-center gap-2 rounded-xl bg-[#8b5cf6] px-6 py-3.5 font-semibold text-white transition-colors hover:bg-[#a78bfa]"
        >
          {t("continueChat", { handle: share.handle })}
        </Link>

        {/* Secondary CTAs */}
        <div className="flex gap-3">
          <Link
            href={`/soul/${share.handle}`}
            className="flex flex-1 items-center justify-center gap-2 rounded-xl border border-[#1e1e2e] bg-[#14141f] px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]/50"
          >
            <span>🔍</span> {t("exploreSoul")}
          </Link>
          <Link
            href="/explore"
            className="flex flex-1 items-center justify-center gap-2 rounded-xl border border-[#1e1e2e] bg-[#14141f] px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]/50"
          >
            <span>🌐</span> {t("exploreAll")}
          </Link>
        </div>
      </div>

      {/* ═══ How It Works — Education Section ═══ */}
      <div className="mt-10">
        <h3 className="mb-4 text-center text-sm font-medium text-[#94a3b8]">
          {t("howItWorks")}
        </h3>
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
            <div className="mb-2 text-2xl">🐚</div>
            <p className="text-xs font-medium text-[#e2e8f0]">{t("step1Title")}</p>
            <p className="mt-1 text-xs text-[#94a3b8]">{t("step1Desc")}</p>
          </div>
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
            <div className="mb-2 text-2xl">🦞</div>
            <p className="text-xs font-medium text-[#e2e8f0]">{t("step2Title")}</p>
            <p className="mt-1 text-xs text-[#94a3b8]">{t("step2Desc")}</p>
          </div>
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-4 text-center">
            <div className="mb-2 text-2xl">💬</div>
            <p className="text-xs font-medium text-[#e2e8f0]">{t("step3Title")}</p>
            <p className="mt-1 text-xs text-[#94a3b8]">{t("step3Desc")}</p>
          </div>
        </div>
      </div>

      {/* ═══ Stats Bar ═══ */}
      {stats && (
        <div className="mt-6 flex items-center justify-center gap-6 rounded-xl border border-[#1e1e2e] bg-[#14141f] px-6 py-3">
          <div className="text-center">
            <div className="text-sm font-bold text-[#e2e8f0]">{stats.souls}</div>
            <div className="text-xs text-[#94a3b8]">{t("statSouls")}</div>
          </div>
          <div className="h-4 w-px bg-[#1e1e2e]" />
          <div className="text-center">
            <div className="text-sm font-bold text-[#e2e8f0]">{stats.fragments}</div>
            <div className="text-xs text-[#94a3b8]">{t("statFragments")}</div>
          </div>
          <div className="h-4 w-px bg-[#1e1e2e]" />
          <div className="text-center">
            <div className="text-sm font-bold text-[#e2e8f0]">{stats.claws}</div>
            <div className="text-xs text-[#94a3b8]">{t("statClaws")}</div>
          </div>
          <div className="h-4 w-px bg-[#1e1e2e]" />
          <div className="text-center">
            <div className="text-sm font-bold text-[#e2e8f0]">{stats.chats}</div>
            <div className="text-xs text-[#94a3b8]">{t("statChats")}</div>
          </div>
        </div>
      )}

      {/* ═══ Footer tagline ═══ */}
      <div className="mt-6 text-center">
        <p className="text-xs text-[#94a3b8]/60">
          {t("tagline")}
        </p>
      </div>
    </div>
  );
}

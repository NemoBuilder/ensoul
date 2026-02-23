"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import {
  sniperApi,
  type SubscriptionStatus,
  type SniperKOL,
  type SniperReply,
  type UserPersona,
} from "@/lib/api";

export default function SniperDashboardPage() {
  const t = useTranslations("Sniper");
  const { isConnected } = useAccount();
  const [sub, setSub] = useState<SubscriptionStatus | null>(null);
  const [kols, setKols] = useState<SniperKOL[]>([]);
  const [replies, setReplies] = useState<SniperReply[]>([]);
  const [persona, setPersona] = useState<UserPersona | null>(null);
  const [loading, setLoading] = useState(true);
  const [newKOL, setNewKOL] = useState("");
  const [addingKOL, setAddingKOL] = useState(false);

  // Persona form state
  const [pBio, setPBio] = useState("");
  const [pStyle, setPStyle] = useState("");
  const [pMaterials, setPMaterials] = useState("");
  const [pLanguage, setPLanguage] = useState("en");
  const [savingPersona, setSavingPersona] = useState(false);
  const [personaMsg, setPersonaMsg] = useState("");

  // Reply generation
  const [replyHandle, setReplyHandle] = useState("");
  const [replyTweetId, setReplyTweetId] = useState("");
  const [replyTweetText, setReplyTweetText] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generatedReply, setGeneratedReply] = useState<SniperReply | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [error, setError] = useState("");

  const fetchData = useCallback(async () => {
    try {
      const [subData, kolData, replyData, personaData] = await Promise.all([
        sniperApi.getSubscription(),
        sniperApi.listKOLs(),
        sniperApi.getReplies(),
        sniperApi.getPersona(),
      ]);
      setSub(subData);
      setKols(kolData.kols || []);
      setReplies(replyData.replies || []);
      if (personaData.configured && personaData.persona) {
        setPersona(personaData.persona);
        setPBio(personaData.persona.bio);
        setPStyle(personaData.persona.style);
        setPMaterials(personaData.persona.materials);
        setPLanguage(personaData.persona.language);
      }
    } catch {
      // Not subscribed or error
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isConnected) fetchData();
    else setLoading(false);
  }, [isConnected, fetchData]);

  async function handleAddKOL() {
    if (!newKOL.trim()) return;
    setAddingKOL(true);
    try {
      await sniperApi.addKOL(newKOL.trim().replace(/^@/, ""));
      setNewKOL("");
      const data = await sniperApi.listKOLs();
      setKols(data.kols || []);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add KOL");
    } finally {
      setAddingKOL(false);
    }
  }

  async function handleRemoveKOL(id: string) {
    try {
      await sniperApi.removeKOL(id);
      setKols((prev) => prev.filter((k) => k.id !== id));
    } catch {}
  }

  async function handleSavePersona() {
    setSavingPersona(true);
    setPersonaMsg("");
    try {
      const p = await sniperApi.setPersona(pBio, pStyle, pMaterials, pLanguage);
      setPersona(p);
      setPersonaMsg(t("personaSaved"));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save persona");
    } finally {
      setSavingPersona(false);
    }
  }

  async function handleGenerate() {
    if (!replyHandle.trim() || !replyTweetId.trim() || !replyTweetText.trim()) return;
    setGenerating(true);
    setGeneratedReply(null);
    setError("");
    try {
      const data = await sniperApi.generateReply(
        replyHandle.trim().replace(/^@/, ""),
        replyTweetId.trim(),
        replyTweetText.trim(),
      );
      setGeneratedReply(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Generation failed");
    } finally {
      setGenerating(false);
    }
  }

  function copyText(text: string, id: string) {
    navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  }

  if (!isConnected) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16 text-center">
        <p className="mb-4 text-[#94a3b8]">{t("connectToSubscribe")}</p>
        <ConnectButton />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        </div>
      </div>
    );
  }

  if (!sub?.active) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-24 pb-16 text-center">
        <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("dashboard")}</h1>
        <p className="mb-6 text-[#94a3b8]">{t("notSubscribed")}</p>
        <Link href="/sniper" className="text-[#8b5cf6] hover:underline">{t("subscribeCTA")}</Link>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("dashboard")}</h1>

      {error && (
        <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Subscription Status */}
      <div className="mb-6 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
        <div className="flex flex-wrap items-center gap-4">
          <div>
            <span className="text-xs text-[#94a3b8]">{t("currentPlan")}</span>
            <p className="font-bold text-[#8b5cf6]">{sub.tier}</p>
          </div>
          <div>
            <span className="text-xs text-[#94a3b8]">{t("expires")}</span>
            <p className="text-sm text-[#e2e8f0]">{sub.expires_at ? new Date(sub.expires_at).toLocaleDateString() : "-"}</p>
          </div>
          <div>
            <span className="text-xs text-[#94a3b8]">{t("usage")}</span>
            <p className="text-sm text-[#e2e8f0]">{sub.daily_replies}/{sub.daily_limit}</p>
          </div>
          <div>
            <span className="text-xs text-[#94a3b8]">KOLs</span>
            <p className="text-sm text-[#e2e8f0]">{sub.kol_count}/{sub.kol_limit}</p>
          </div>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* KOL Tracking */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
          <h2 className="mb-3 text-sm font-semibold text-[#e2e8f0]">{t("trackedKOLs")}</h2>
          <div className="mb-3 flex gap-2">
            <input
              type="text"
              value={newKOL}
              onChange={(e) => setNewKOL(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleAddKOL()}
              placeholder={t("addKOLPlaceholder")}
              className="flex-1 rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none"
            />
            <button
              onClick={handleAddKOL}
              disabled={addingKOL}
              className="rounded-md bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {t("addKOL")}
            </button>
          </div>
          {kols.length === 0 ? (
            <p className="py-4 text-center text-sm text-[#94a3b8]">{t("noKOLs")}</p>
          ) : (
            <div className="space-y-2">
              {kols.map((k) => (
                <div key={k.id} className="flex items-center justify-between rounded-md bg-[#0a0a0f] px-3 py-2">
                  <span className="text-sm text-[#e2e8f0]">@{k.handle}</span>
                  <button
                    onClick={() => handleRemoveKOL(k.id)}
                    className="text-xs text-red-400 hover:text-red-300"
                  >
                    {t("removeKOL")}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Persona Settings */}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
          <h2 className="mb-3 text-sm font-semibold text-[#e2e8f0]">{t("persona")}</h2>
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">{t("personaBio")}</label>
              <textarea
                value={pBio}
                onChange={(e) => setPBio(e.target.value)}
                rows={2}
                className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">{t("personaStyle")}</label>
              <input
                type="text"
                value={pStyle}
                onChange={(e) => setPStyle(e.target.value)}
                className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#94a3b8]">{t("personaLanguage")}</label>
              <select
                value={pLanguage}
                onChange={(e) => setPLanguage(e.target.value)}
                className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              >
                <option value="en">English</option>
                <option value="zh">中文</option>
                <option value="ja">日本語</option>
                <option value="ko">한국어</option>
              </select>
            </div>
            <button
              onClick={handleSavePersona}
              disabled={savingPersona}
              className="w-full rounded-md bg-[#8b5cf6] py-2 text-sm font-medium text-white hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {savingPersona ? t("saving") : t("savePersona")}
            </button>
            {personaMsg && <p className="text-center text-xs text-green-400">{personaMsg}</p>}
          </div>
        </div>
      </div>

      {/* Reply Generation */}
      <div className="mt-6 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
        <h2 className="mb-3 text-sm font-semibold text-[#e2e8f0]">{t("generateReply")}</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <input
            type="text"
            value={replyHandle}
            onChange={(e) => setReplyHandle(e.target.value)}
            placeholder="KOL handle"
            className="rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
          />
          <input
            type="text"
            value={replyTweetId}
            onChange={(e) => setReplyTweetId(e.target.value)}
            placeholder={t("tweetId")}
            className="rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
          />
          <button
            onClick={handleGenerate}
            disabled={generating}
            className="rounded-md bg-[#8b5cf6] py-2 text-sm font-medium text-white hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {generating ? t("generating") : t("generate")}
          </button>
        </div>
        <textarea
          value={replyTweetText}
          onChange={(e) => setReplyTweetText(e.target.value)}
          placeholder={t("tweetText")}
          rows={2}
          className="mt-3 w-full rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
        />

        {generatedReply && generatedReply.replies && (
          <div className="mt-4 space-y-3">
            {generatedReply.replies.map((r, i) => (
              <div key={i} className="rounded-md bg-[#0a0a0f] p-3">
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-xs font-medium text-[#8b5cf6]">{r.style}</span>
                  <button
                    onClick={() => copyText(r.content, `${i}`)}
                    className="text-xs text-[#94a3b8] hover:text-[#e2e8f0]"
                  >
                    {copied === `${i}` ? t("copied") : t("copy")}
                  </button>
                </div>
                <p className="text-sm text-[#e2e8f0]">{r.content}</p>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Recent Replies */}
      <div className="mt-6 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4">
        <h2 className="mb-3 text-sm font-semibold text-[#e2e8f0]">{t("recentReplies")}</h2>
        {replies.length === 0 ? (
          <p className="py-4 text-center text-sm text-[#94a3b8]">{t("noReplies")}</p>
        ) : (
          <div className="space-y-3">
            {replies.slice(0, 10).map((r) => (
              <div key={r.id} className="rounded-md bg-[#0a0a0f] p-3">
                <p className="mb-1 text-xs text-[#94a3b8]">
                  Tweet: {r.tweet_text?.slice(0, 80)}...
                </p>
                {r.replies?.map((v, i) => (
                  <div key={i} className="mt-2 border-l-2 border-[#8b5cf6]/30 pl-3">
                    <span className="text-xs text-[#8b5cf6]">{v.style}</span>
                    <p className="text-sm text-[#e2e8f0]">{v.content}</p>
                  </div>
                ))}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

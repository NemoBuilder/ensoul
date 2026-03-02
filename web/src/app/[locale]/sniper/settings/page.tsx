"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { Link } from "@/i18n/navigation";
import {
  sniperApi,
  type SniperTag,
  type SubscriptionStatus,
} from "@/lib/api";
import TagCloudFilter from "@/components/sniper/TagCloudFilter";

export default function SniperSettingsPage() {
  const t = useTranslations("Sniper");
  const { isConnected } = useAccount();

  // Tags
  const [tags, setTags] = useState<SniperTag[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);

  // Muted
  const [mutedHandles, setMutedHandles] = useState<string[]>([]);

  // Persona
  const [pBio, setPBio] = useState("");
  const [pStyle, setPStyle] = useState("");
  const [pMaterials, setPMaterials] = useState("");
  const [pLanguage, setPLanguage] = useState("en");
  const [savingPersona, setSavingPersona] = useState(false);
  const [personaMsg, setPersonaMsg] = useState("");

  // Subscription
  const [subscription, setSubscription] = useState<SubscriptionStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    try {
      const [tagData, userTags, muted, sub, personaData] = await Promise.allSettled([
        sniperApi.getTags(),
        sniperApi.getUserTags(),
        sniperApi.getMuted(),
        sniperApi.getSubscription(),
        sniperApi.getPersona(),
      ]);

      if (tagData.status === "fulfilled") {
        setTags(tagData.value.tags || []);
        // Use defaults if no user tags
        if (userTags.status === "fulfilled" && userTags.value.tag_ids?.length > 0) {
          setSelectedTagIds(userTags.value.tag_ids);
        } else {
          setSelectedTagIds(tagData.value.defaults || []);
        }
      }

      if (muted.status === "fulfilled") {
        setMutedHandles(muted.value.handles || []);
      }

      if (sub.status === "fulfilled") {
        setSubscription(sub.value);
      }

      if (personaData.status === "fulfilled" && personaData.value.configured && personaData.value.persona) {
        const p = personaData.value.persona;
        setPBio(p.bio);
        setPStyle(p.style);
        setPMaterials(p.materials);
        setPLanguage(p.language);
      }
    } catch {
      // Error loading
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (isConnected) fetchData();
    else setLoading(false);
  }, [isConnected, fetchData]);

  async function handleToggleTag(tagId: string) {
    const next = selectedTagIds.includes(tagId)
      ? selectedTagIds.filter((id) => id !== tagId)
      : [...selectedTagIds, tagId];
    setSelectedTagIds(next);
    try {
      await sniperApi.updateUserTags(next);
    } catch {}
  }

  async function handleUnmute(handle: string) {
    setMutedHandles((prev) => prev.filter((h) => h !== handle));
    try {
      await sniperApi.unmuteAccount(handle);
    } catch {
      // Revert
      setMutedHandles((prev) => [...prev, handle]);
    }
  }

  async function handleSavePersona() {
    setSavingPersona(true);
    setPersonaMsg("");
    try {
      await sniperApi.setPersona(pBio, pStyle, pMaterials, pLanguage);
      setPersonaMsg(t("personaSaved"));
    } catch (err: unknown) {
      setPersonaMsg(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSavingPersona(false);
    }
  }

  if (!isConnected) {
    return (
      <div className="mx-auto max-w-2xl px-4 pt-24 pb-16 text-center">
        <h1 className="text-2xl font-bold text-[#e2e8f0]">{t("settingsTitle")}</h1>
        <p className="mt-4 text-[#94a3b8]">{t("connectToSubscribe")}</p>
        <div className="mt-6 flex justify-center">
          <ConnectButton />
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-2xl px-4 pt-24 pb-16 text-center">
        <div className="h-8 w-8 mx-auto animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
        <p className="mt-3 text-sm text-[#64748b]">{t("loading")}</p>
      </div>
    );
  }

  const isPro = subscription?.active && subscription?.tier === "pro";

  return (
    <div className="mx-auto max-w-2xl px-4 pt-20 pb-16">
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[#e2e8f0]">{t("settingsTitle")}</h1>
        </div>
        <Link
          href="/sniper"
          className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs font-medium text-[#94a3b8]
            hover:border-[#334155] hover:text-[#e2e8f0] transition-colors"
        >
          ← {t("title")}
        </Link>
      </div>

      {/* Tag preferences */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-[#e2e8f0] mb-2">{t("tagPreferences")}</h2>
        <p className="text-sm text-[#64748b] mb-4">{t("tagPreferencesDesc")}</p>
        <TagCloudFilter
          tags={tags}
          selectedTagIds={selectedTagIds}
          onToggleTag={handleToggleTag}
        />
      </section>

      {/* Muted accounts */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-[#e2e8f0] mb-2">{t("mutedAccounts")}</h2>
        <p className="text-sm text-[#64748b] mb-4">{t("mutedAccountsDesc")}</p>
        {mutedHandles.length === 0 ? (
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6 text-center text-sm text-[#64748b]">
            {t("noMutedAccounts")}
          </div>
        ) : (
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] divide-y divide-[#1e1e2e]">
            {mutedHandles.map((handle) => (
              <div key={handle} className="flex items-center justify-between px-4 py-3">
                <span className="text-sm text-[#e2e8f0]">@{handle}</span>
                <button
                  onClick={() => handleUnmute(handle)}
                  className="rounded px-3 py-1 text-xs font-medium text-[#f87171] hover:bg-[#1e1e2e] transition-colors"
                >
                  {t("unmute")}
                </button>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Persona settings (Pro only) */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-[#e2e8f0] mb-2">{t("personaSettings")}</h2>
        <p className="text-sm text-[#64748b] mb-4">{t("personaSettingsDesc")}</p>

        {!isPro ? (
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6 text-center">
            <p className="text-sm text-[#64748b] mb-3">{t("upgradeToSnipe")}</p>
            <button
              onClick={() => {}}
              className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white hover:bg-[#a78bfa] transition-colors"
            >
              {t("upgradePro")} — {t("proPrice")}{t("perMonth")}
            </button>
          </div>
        ) : (
          <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6 space-y-4">
            <div>
              <label className="block text-sm font-medium text-[#94a3b8] mb-1">
                📝 {t("personaBio")}
              </label>
              <textarea
                value={pBio}
                onChange={(e) => setPBio(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-3 text-sm text-[#e2e8f0]
                  placeholder-[#64748b] focus:border-[#8b5cf6] focus:outline-none resize-none"
                rows={3}
                placeholder="Crypto researcher & BNB Chain builder..."
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-[#94a3b8] mb-1">
                🎨 {t("personaStyle")}
              </label>
              <textarea
                value={pStyle}
                onChange={(e) => setPStyle(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-3 text-sm text-[#e2e8f0]
                  placeholder-[#64748b] focus:border-[#8b5cf6] focus:outline-none resize-none"
                rows={2}
                placeholder="Technical but approachable. Occasionally humorous..."
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-[#94a3b8] mb-1">
                📚 {t("personaMaterials")}
              </label>
              <textarea
                value={pMaterials}
                onChange={(e) => setPMaterials(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-3 text-sm text-[#e2e8f0]
                  placeholder-[#64748b] focus:border-[#8b5cf6] focus:outline-none resize-none"
                rows={2}
                placeholder="My blog posts, research threads..."
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-[#94a3b8] mb-1">
                🌐 {t("personaLanguage")}
              </label>
              <div className="flex flex-wrap gap-3">
                {[
                  { value: "en", label: "English" },
                  { value: "zh", label: "中文" },
                  { value: "ja", label: "日本語" },
                  { value: "auto", label: t("personaLanguageAuto") },
                ].map((opt) => (
                  <label key={opt.value} className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="radio"
                      name="persona-lang"
                      value={opt.value}
                      checked={pLanguage === opt.value}
                      onChange={(e) => setPLanguage(e.target.value)}
                      className="accent-[#8b5cf6]"
                    />
                    <span className="text-sm text-[#e2e8f0]">{opt.label}</span>
                  </label>
                ))}
              </div>
            </div>

            {personaMsg && (
              <p className="text-sm text-[#8b5cf6]">{personaMsg}</p>
            )}

            <button
              onClick={handleSavePersona}
              disabled={savingPersona}
              className="rounded-lg bg-[#8b5cf6] px-6 py-2 text-sm font-semibold text-white
                hover:bg-[#a78bfa] disabled:opacity-50 transition-colors"
            >
              {savingPersona ? t("saving") : t("savePersona")}
            </button>
          </div>
        )}
      </section>

      {/* Subscription info */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold text-[#e2e8f0] mb-2">{t("currentPlan")}</h2>
        <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-6">
          {isPro ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <span className="rounded-full bg-[#8b5cf6]/10 px-3 py-1 text-xs font-medium text-[#c4b5fd]">
                  {t("proPlan")}
                </span>
              </div>
              {subscription?.expires_at && (
                <p className="text-sm text-[#64748b]">
                  {t("expires")}: {new Date(subscription.expires_at).toLocaleDateString()}
                </p>
              )}
              <p className="text-sm text-[#64748b]">
                {t("todayUsage", {
                  used: subscription?.daily_snipes ?? 0,
                  limit: subscription?.daily_limit ?? 50,
                })}
              </p>
            </div>
          ) : (
            <div className="text-center">
              <p className="text-sm text-[#64748b] mb-3">{t("notSubscribed")}</p>
              <button className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white hover:bg-[#a78bfa] transition-colors">
                {t("subscribeCTA")}
              </button>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { kolClaimApi } from "@/lib/api";

export default function ClaimPage() {
  const t = useTranslations("Claim");
  const { isConnected } = useAccount();

  const [handle, setHandle] = useState("");
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [verifyCode, setVerifyCode] = useState("");
  const [tweetId, setTweetId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  async function handleInitiate() {
    if (!handle.trim()) return;
    setError("");
    setLoading(true);
    try {
      const data = await kolClaimApi.initiate(handle.trim().replace(/^@/, ""));
      setVerifyCode(data.verify_code);
      setStep(2);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to initiate claim");
    } finally {
      setLoading(false);
    }
  }

  async function handleVerify() {
    if (!tweetId.trim()) return;
    setError("");
    setLoading(true);
    try {
      await kolClaimApi.verify(handle.trim().replace(/^@/, ""), tweetId.trim());
      setSuccess(true);
      setStep(3);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setLoading(false);
    }
  }

  if (!isConnected) {
    return (
      <div className="mx-auto max-w-xl px-4 pt-24 pb-16 text-center">
        <h1 className="mb-4 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
        <p className="mb-6 text-[#94a3b8]">{t("connectPrompt")}</p>
        <ConnectButton />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {error && (
        <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">{error}</div>
      )}

      {success ? (
        <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-8 text-center">
          <p className="text-lg font-bold text-green-400">🎉 {t("success")}</p>
        </div>
      ) : (
        <div className="space-y-6">
          {/* Step indicators */}
          <div className="flex items-center gap-2">
            {[1, 2, 3].map((s) => (
              <div key={s} className="flex items-center gap-2">
                <div className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold ${
                  step >= s ? "bg-[#8b5cf6] text-white" : "bg-[#1e1e2e] text-[#94a3b8]"
                }`}>
                  {s}
                </div>
                {s < 3 && <div className={`h-0.5 w-8 ${step > s ? "bg-[#8b5cf6]" : "bg-[#1e1e2e]"}`} />}
              </div>
            ))}
          </div>

          {/* Step 1: Enter handle */}
          {step === 1 && (
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
              <h2 className="mb-4 text-sm font-semibold text-[#e2e8f0]">{t("step1")}</h2>
              <label className="mb-2 block text-xs text-[#94a3b8]">{t("handleLabel")}</label>
              <div className="flex gap-3">
                <div className="flex flex-1 items-center rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3">
                  <span className="text-[#94a3b8]">@</span>
                  <input
                    type="text"
                    value={handle}
                    onChange={(e) => setHandle(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleInitiate()}
                    placeholder={t("handlePlaceholder")}
                    className="w-full bg-transparent px-2 py-3 text-[#e2e8f0] outline-none"
                  />
                </div>
                <button
                  onClick={handleInitiate}
                  disabled={loading || !handle.trim()}
                  className="rounded-md bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white hover:bg-[#a78bfa] disabled:opacity-50"
                >
                  {loading ? t("initiating") : t("initiate")}
                </button>
              </div>
            </div>
          )}

          {/* Step 2: Post tweet with code */}
          {step === 2 && (
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
              <h2 className="mb-4 text-sm font-semibold text-[#e2e8f0]">{t("step2")}</h2>
              <p className="mb-3 text-sm text-[#94a3b8]">
                {t("instruction", { handle: handle.replace(/^@/, "") })}
              </p>
              <div className="mb-6 rounded-md bg-[#0a0a0f] p-4 text-center">
                <code className="font-mono text-lg font-bold text-[#8b5cf6]">{verifyCode}</code>
              </div>
              <label className="mb-2 block text-xs text-[#94a3b8]">{t("tweetIdLabel")}</label>
              <div className="flex gap-3">
                <input
                  type="text"
                  value={tweetId}
                  onChange={(e) => setTweetId(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleVerify()}
                  placeholder={t("tweetIdPlaceholder")}
                  className="flex-1 rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-3 text-[#e2e8f0] outline-none"
                />
                <button
                  onClick={handleVerify}
                  disabled={loading || !tweetId.trim()}
                  className="rounded-md bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white hover:bg-[#a78bfa] disabled:opacity-50"
                >
                  {loading ? t("verifying") : t("verify")}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

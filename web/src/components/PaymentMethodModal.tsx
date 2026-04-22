"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import { billingApi } from "@/lib/api";

interface PaymentMethodModalProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Two-button payment method selector. Card path → LemonSqueezy checkout
 * (no brand mention shown to user). Crypto path → /pay/crypto flow.
 */
export default function PaymentMethodModal({ open, onClose }: PaymentMethodModalProps) {
  const t = useTranslations("PaymentMethod");
  const tPay = useTranslations("CryptoPay");
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const MONTH_OPTIONS = [1, 3, 6, 12] as const;
  const [months, setMonths] = useState<number>(1);

  if (!open) return null;

  async function handleCard() {
    setLoading(true);
    setError("");
    try {
      const res = await billingApi.checkout();
      window.location.href = res.url;
    } catch {
      setError(t("cardError"));
      setLoading(false);
    }
  }

  function handleCrypto() {
    onClose();
    router.push(`/pay/crypto?months=${months}`);
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-5 text-center">
          <h2 className="text-lg font-bold text-[#e2e8f0]">{t("title")}</h2>
          <p className="mt-1 text-sm text-[#94a3b8]">{t("subtitle")}</p>
        </div>

        {/* Month selector — applies to crypto path. Card path is monthly subscription. */}
        <div className="mb-5">
          <div className="mb-2 flex items-center justify-between text-xs">
            <span className="text-[#94a3b8]">{tPay("selectMonths")}</span>
            <span className="text-[#64748b]">{tPay("monthsHint")}</span>
          </div>
          <div className="grid grid-cols-4 gap-2">
            {MONTH_OPTIONS.map((m) => {
              const active = months === m;
              return (
                <button
                  key={m}
                  onClick={() => setMonths(m)}
                  className={`rounded-lg border p-2 text-center transition-colors ${
                    active
                      ? "border-[#8b5cf6] bg-[#8b5cf6]/10"
                      : "border-[#1e1e2e] hover:border-[#475569]"
                  }`}
                >
                  <div className="text-sm font-semibold text-[#e2e8f0]">{m}</div>
                  <div className="text-[10px] text-[#94a3b8]">{tPay("monthsUnit")}</div>
                </button>
              );
            })}
          </div>
        </div>

        <div className="space-y-3">
          {(() => {
            const cardEnabled = process.env.NEXT_PUBLIC_CARD_PAYMENT_ENABLED === "true";
            return (
              <button
                onClick={cardEnabled ? handleCard : undefined}
                disabled={loading || !cardEnabled}
                title={cardEnabled ? undefined : t("cardComingSoon")}
                className={`group flex w-full items-center gap-4 rounded-xl border p-4 text-left transition-colors ${
                  cardEnabled
                    ? "border-[#1e1e2e] bg-[#0a0a0f] hover:border-[#8b5cf6] disabled:opacity-50"
                    : "cursor-not-allowed border-[#1e1e2e] bg-[#0a0a0f]/60 opacity-60"
                }`}
              >
                <span className="text-2xl">💳</span>
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <div className="text-sm font-semibold text-[#e2e8f0]">{t("cardTitle")}</div>
                    {!cardEnabled && (
                      <span className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-[10px] font-semibold text-[#94a3b8]">
                        {t("cardComingSoon")}
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-[#94a3b8]">{t("cardDesc")}</div>
                </div>
                <span className="text-[#94a3b8] group-hover:text-[#a78bfa]">→</span>
              </button>
            );
          })()}

          <button
            onClick={handleCrypto}
            disabled={loading}
            className="group flex w-full items-center gap-4 rounded-xl border border-[#1e1e2e] bg-[#0a0a0f] p-4 text-left transition-colors hover:border-[#8b5cf6] disabled:opacity-50"
          >
            <span className="text-2xl">🪙</span>
            <div className="flex-1">
              <div className="text-sm font-semibold text-[#e2e8f0]">{t("cryptoTitle")}</div>
              <div className="text-xs text-[#94a3b8]">{t("cryptoDesc")}</div>
            </div>
            <span className="text-[#94a3b8] group-hover:text-[#a78bfa]">→</span>
          </button>
        </div>

        {error && (
          <p className="mt-3 text-center text-sm text-red-400">{error}</p>
        )}

        <button
          onClick={onClose}
          className="mt-4 w-full rounded-lg border border-[#1e1e2e] px-4 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
        >
          {t("cancel")}
        </button>
      </div>
    </div>
  );
}

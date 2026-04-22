"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { PRICING } from "@/lib/pricing";
import PaymentMethodModal from "./PaymentMethodModal";

interface UpgradeModalProps {
  open: boolean;
  onClose: () => void;
  reason?: "credits" | "workspace" | "memory" | "feature";
}

export default function UpgradeModal({ open, onClose, reason = "feature" }: UpgradeModalProps) {
  const t = useTranslations("Upgrade");
  const [methodOpen, setMethodOpen] = useState(false);

  if (!open) return null;

  function handleUpgrade() {
    setMethodOpen(true);
  }

  const reasonText: Record<string, string> = {
    credits: t("reasonCredits"),
    workspace: t("reasonWorkspace"),
    // "memory" reason is no longer triggered (V3: memory categories are free for all)
    // but kept in the union for backward-compat; fall through to the generic copy.
    memory: t("reasonFeature"),
    feature: t("reasonFeature"),
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div
        className="mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="mb-4 text-center">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-[#8b5cf6]/20">
            <span className="text-2xl">⚡</span>
          </div>
          <h2 className="text-lg font-bold text-[#e2e8f0]">{t("title")}</h2>
          <p className="mt-1 text-sm text-[#94a3b8]">{reasonText[reason]}</p>
        </div>

        {/* Features */}
        <div className="mb-6 space-y-3 rounded-xl bg-[#0a0a0f] p-4">
          {["credits5000", "workspaces10", "allMemory", "soulEnhanced", "variants3", "prioritySupport"].map((key) => (
            <div key={key} className="flex items-center gap-2 text-sm text-[#e2e8f0]">
              <span className="text-[#8b5cf6]">✓</span>
              <span>{t(key)}</span>
            </div>
          ))}
        </div>

        {/* Price */}
        <div className="mb-2 text-center">
          <span className="text-3xl font-bold text-[#e2e8f0]">${PRICING.pro.priceUSD}</span>
          <span className="text-sm text-[#64748b]"> / {t("month")}</span>
        </div>

        {/* Link to full pricing page */}
        <div className="mb-4 text-center">
          <Link
            href="/pricing"
            onClick={onClose}
            className="text-xs text-[#94a3b8] underline-offset-2 hover:text-[#a78bfa] hover:underline"
          >
            {t("viewFullComparison")}
          </Link>
        </div>

        {/* Actions */}
        <div className="flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 rounded-lg border border-[#1e1e2e] px-4 py-2.5 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
          >
            {t("maybeLater")}
          </button>
          <button
            onClick={handleUpgrade}
            className="flex-1 rounded-lg bg-[#8b5cf6] px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
          >
            {t("upgradePro")}
          </button>
        </div>
      </div>
      <PaymentMethodModal open={methodOpen} onClose={() => setMethodOpen(false)} />
    </div>
  );
}

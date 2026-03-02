"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAccount, useWriteContract, usePublicClient } from "wagmi";
import { parseAbi, parseUnits } from "viem";
import { sniperApi } from "@/lib/api";

const USDT_ADDRESS = (process.env.NEXT_PUBLIC_USDT_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const TREASURY_ADDRESS = (process.env.NEXT_PUBLIC_TREASURY_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const ERC20_ABI = parseAbi(["function transfer(address to, uint256 amount) returns (bool)"]);

interface SubscribeModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export default function SubscribeModal({ open, onClose, onSuccess }: SubscribeModalProps) {
  const t = useTranslations("Sniper");
  const { address } = useAccount();
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const [subscribing, setSubscribing] = useState(false);
  const [error, setError] = useState("");

  if (!open) return null;

  async function handleSubscribe() {
    if (!address || !publicClient) return;
    setError("");
    setSubscribing(true);

    try {
      const txHash = await writeContractAsync({
        address: USDT_ADDRESS,
        abi: ERC20_ABI,
        functionName: "transfer",
        args: [TREASURY_ADDRESS, parseUnits("99", 18)],
      });

      await publicClient.waitForTransactionReceipt({ hash: txHash });
      await sniperApi.subscribe("pro", txHash, "USDT", 99);
      onSuccess();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setSubscribing(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 shadow-2xl">
        {/* Close */}
        <button
          onClick={onClose}
          className="absolute right-4 top-4 text-[#64748b] hover:text-[#94a3b8]"
        >
          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {/* Content */}
        <div className="text-center">
          <div className="text-4xl mb-3">🎯</div>
          <h3 className="text-xl font-bold text-[#e2e8f0]">{t("upgradePro")}</h3>
          <p className="mt-2 text-sm text-[#94a3b8]">{t("upgradeToSnipe")}</p>
        </div>

        {/* Pro features */}
        <div className="mt-6 rounded-xl border border-[#8b5cf6]/30 bg-[#8b5cf6]/5 p-4">
          <div className="flex items-baseline justify-between">
            <span className="text-lg font-bold text-[#e2e8f0]">{t("pro")}</span>
            <div className="flex items-baseline gap-1">
              <span className="font-mono text-3xl font-bold text-[#8b5cf6]">{t("proPrice")}</span>
              <span className="text-sm text-[#64748b]">{t("perMonth")}</span>
            </div>
          </div>
          <ul className="mt-4 space-y-2 text-sm text-[#94a3b8]">
            <li className="flex items-center gap-2">
              <span className="text-[#8b5cf6]">✓</span> 50 snipes/day
            </li>
            <li className="flex items-center gap-2">
              <span className="text-[#8b5cf6]">✓</span> Claude Sonnet AI model
            </li>
            <li className="flex items-center gap-2">
              <span className="text-[#8b5cf6]">✓</span> Soul 🧬 persona boost
            </li>
            <li className="flex items-center gap-2">
              <span className="text-[#8b5cf6]">✓</span> Custom Persona config
            </li>
            <li className="flex items-center gap-2">
              <span className="text-[#8b5cf6]">✓</span> SSE real-time push
            </li>
          </ul>
        </div>

        {error && (
          <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">
            {error}
          </div>
        )}

        {/* Actions */}
        <div className="mt-6 space-y-3">
          <button
            onClick={handleSubscribe}
            disabled={subscribing}
            className="w-full rounded-xl bg-[#8b5cf6] py-3 text-sm font-semibold text-white
              transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {subscribing ? t("subscribing") : `${t("upgradePro")} — ${t("proPrice")}${t("perMonth")}`}
          </button>
          <button
            onClick={onClose}
            className="w-full rounded-xl border border-[#1e1e2e] py-3 text-sm font-medium text-[#94a3b8]
              hover:border-[#334155] hover:text-[#e2e8f0] transition-colors"
          >
            {t("continueBrowsing")}
          </button>
        </div>

        <p className="mt-4 text-center text-xs text-[#64748b]">{t("paymentNote")}</p>
      </div>
    </div>
  );
}

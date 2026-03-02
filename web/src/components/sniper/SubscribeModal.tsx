"use client";

import { useState, useEffect, useCallback } from "react";
import { useTranslations } from "next-intl";
import { useAccount, useWriteContract, usePublicClient, useSendTransaction } from "wagmi";
import { parseAbi, parseUnits, parseEther } from "viem";
import { sniperApi, type SubscribePrice } from "@/lib/api";

const USDT_ADDRESS = (process.env.NEXT_PUBLIC_USDT_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const ERC20_ABI = parseAbi(["function transfer(address to, uint256 amount) returns (bool)"]);

type PaymentMethod = "USDT" | "BNB";

interface SubscribeModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export default function SubscribeModal({ open, onClose, onSuccess }: SubscribeModalProps) {
  const t = useTranslations("Sniper");
  const { address } = useAccount();
  const { writeContractAsync } = useWriteContract();
  const { sendTransactionAsync } = useSendTransaction();
  const publicClient = usePublicClient();
  const [subscribing, setSubscribing] = useState(false);
  const [error, setError] = useState("");
  const [method, setMethod] = useState<PaymentMethod>("USDT");
  const [priceData, setPriceData] = useState<SubscribePrice | null>(null);
  const [loadingPrice, setLoadingPrice] = useState(false);

  // Fetch price when modal opens
  const fetchPrice = useCallback(async () => {
    setLoadingPrice(true);
    try {
      const data = await sniperApi.getSubscribePrice("pro");
      setPriceData(data);
    } catch {
      // fallback
      setPriceData(null);
    } finally {
      setLoadingPrice(false);
    }
  }, []);

  useEffect(() => {
    if (open) {
      fetchPrice();
      // Refresh price every 60s while modal is open
      const interval = setInterval(fetchPrice, 60_000);
      return () => clearInterval(interval);
    }
  }, [open, fetchPrice]);

  if (!open) return null;

  const priceUSDT = priceData?.price_usdt ?? 99;
  const priceBNB = priceData?.price_bnb ? parseFloat(priceData.price_bnb) : 0;
  const bnbPrice = priceData?.bnb_price ?? 0;
  const treasuryAddr = (priceData?.treasury || process.env.NEXT_PUBLIC_TREASURY_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;

  async function handleSubscribe() {
    if (!address || !publicClient) return;
    setError("");
    setSubscribing(true);

    try {
      let txHash: `0x${string}`;

      if (method === "USDT") {
        // ERC-20 USDT transfer to treasury
        txHash = await writeContractAsync({
          address: USDT_ADDRESS,
          abi: ERC20_ABI,
          functionName: "transfer",
          args: [treasuryAddr, parseUnits(String(priceUSDT), 18)],
        });
      } else {
        // Native BNB transfer to treasury
        if (priceBNB <= 0) {
          setError("BNB price unavailable, please use USDT");
          setSubscribing(false);
          return;
        }
        // Use 6 decimal places for BNB amount
        const bnbStr = priceBNB.toFixed(6);
        txHash = await sendTransactionAsync({
          to: treasuryAddr,
          value: parseEther(bnbStr),
        });
      }

      await publicClient.waitForTransactionReceipt({ hash: txHash });
      await sniperApi.subscribe(
        "pro",
        txHash,
        method,
        method === "USDT" ? priceUSDT : priceBNB,
      );
      onSuccess();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Payment failed");
    } finally {
      setSubscribing(false);
    }
  }

  const displayPrice = method === "USDT"
    ? `${priceUSDT} USDT`
    : priceBNB > 0 ? `${priceBNB.toFixed(4)} BNB` : "—";

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

        {/* Payment method toggle */}
        <div className="mt-5">
          <p className="mb-2 text-xs font-medium text-[#94a3b8] uppercase tracking-wider">
            {t("paymentMethod")}
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setMethod("USDT")}
              className={`flex-1 rounded-lg border py-2.5 text-sm font-semibold transition-colors ${
                method === "USDT"
                  ? "border-[#8b5cf6] bg-[#8b5cf6]/10 text-[#8b5cf6]"
                  : "border-[#1e1e2e] text-[#64748b] hover:border-[#334155]"
              }`}
            >
              💵 USDT
            </button>
            <button
              onClick={() => setMethod("BNB")}
              disabled={priceBNB <= 0 && !loadingPrice}
              className={`flex-1 rounded-lg border py-2.5 text-sm font-semibold transition-colors
                disabled:opacity-40 disabled:cursor-not-allowed ${
                method === "BNB"
                  ? "border-[#f0b90b] bg-[#f0b90b]/10 text-[#f0b90b]"
                  : "border-[#1e1e2e] text-[#64748b] hover:border-[#334155]"
              }`}
            >
              ⟠ BNB
            </button>
          </div>

          {/* Price display */}
          <div className="mt-3 rounded-lg bg-[#0a0a14] px-4 py-3 text-center">
            {loadingPrice ? (
              <span className="text-sm text-[#64748b] animate-pulse">{t("loading")}</span>
            ) : (
              <>
                <span className="font-mono text-lg font-bold text-[#e2e8f0]">{displayPrice}</span>
                {method === "BNB" && bnbPrice > 0 && (
                  <p className="mt-1 text-xs text-[#64748b]">
                    1 BNB ≈ ${bnbPrice.toFixed(2)} · {t("bnbPriceNote")}
                  </p>
                )}
              </>
            )}
          </div>
        </div>

        {error && (
          <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/5 p-3 text-sm text-red-400">
            {error}
          </div>
        )}

        {/* Actions */}
        <div className="mt-5 space-y-3">
          <button
            onClick={handleSubscribe}
            disabled={subscribing || loadingPrice}
            className="w-full rounded-xl bg-[#8b5cf6] py-3 text-sm font-semibold text-white
              transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {subscribing
              ? t("subscribing")
              : `${t("payNow")} ${displayPrice}`}
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

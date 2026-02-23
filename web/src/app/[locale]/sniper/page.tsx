"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount, useWriteContract, usePublicClient } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { parseAbi, parseUnits } from "viem";
import { sniperApi } from "@/lib/api";

const USDT_ADDRESS = (process.env.NEXT_PUBLIC_USDT_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const TREASURY_ADDRESS = (process.env.NEXT_PUBLIC_TREASURY_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const ERC20_ABI = parseAbi(["function transfer(address to, uint256 amount) returns (bool)"]);

const TIERS = [
  { key: "starter", price: 9.9, kols: 3, replies: 10, model: "GPT-4o-mini" },
  { key: "pro", price: 29.9, kols: 10, replies: 50, model: "GPT-4o" },
  { key: "elite", price: 99.9, kols: 30, replies: 200, model: "Claude Opus" },
];

export default function SniperPage() {
  const t = useTranslations("Sniper");
  const { address, isConnected } = useAccount();
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const [subscribing, setSubscribing] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);

  async function handleSubscribe(tier: typeof TIERS[0]) {
    if (!address || !publicClient) return;
    setError("");
    setSuccess(false);
    setSubscribing(tier.key);

    try {
      const txHash = await writeContractAsync({
        address: USDT_ADDRESS,
        abi: ERC20_ABI,
        functionName: "transfer",
        args: [TREASURY_ADDRESS, parseUnits(tier.price.toString(), 18)],
      });

      await publicClient.waitForTransactionReceipt({ hash: txHash });
      await sniperApi.subscribe(tier.key, txHash, "USDT", tier.price);
      setSuccess(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Subscription failed");
    } finally {
      setSubscribing(null);
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {!isConnected ? (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
          <p className="mb-4 text-[#94a3b8]">{t("connectToSubscribe")}</p>
          <ConnectButton />
        </div>
      ) : (
        <>
          {success && (
            <div className="mb-6 rounded-lg border border-green-500/30 bg-green-500/5 p-4 text-center text-sm text-green-400">
              Subscription activated! <Link href="/sniper/dashboard" className="underline">Go to Dashboard →</Link>
            </div>
          )}

          {error && (
            <div className="mb-6 rounded-lg border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-400">
              {error}
            </div>
          )}

          <div className="grid gap-6 md:grid-cols-3">
            {TIERS.map((tier) => (
              <div
                key={tier.key}
                className={`rounded-lg border p-6 ${
                  tier.key === "pro"
                    ? "border-[#8b5cf6]/50 bg-[#8b5cf6]/5"
                    : "border-[#1e1e2e] bg-[#14141f]"
                }`}
              >
                <h3 className="text-lg font-bold text-[#e2e8f0]">
                  {t(tier.key as "starter" | "pro" | "elite")}
                </h3>
                <div className="mt-2 flex items-baseline gap-1">
                  <span className="font-mono text-3xl font-bold text-[#8b5cf6]">
                    ${tier.price}
                  </span>
                  <span className="text-sm text-[#94a3b8]">{t("perMonth")}</span>
                </div>
                <ul className="mt-4 space-y-2 text-sm text-[#94a3b8]">
                  <li>· {tier.kols} {t("kols")}</li>
                  <li>· {tier.replies} {t("repliesPerDay")}</li>
                  <li>· {t("model")}: {tier.model}</li>
                </ul>
                <button
                  onClick={() => handleSubscribe(tier)}
                  disabled={subscribing !== null}
                  className="mt-6 w-full rounded-lg bg-[#8b5cf6] py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
                >
                  {subscribing === tier.key ? t("subscribing") : t("subscribe")}
                </button>
              </div>
            ))}
          </div>

          <p className="mt-6 text-center text-xs text-[#94a3b8]">{t("paymentNote")}</p>

          <div className="mt-8 text-center">
            <Link
              href="/sniper/dashboard"
              className="text-sm text-[#8b5cf6] hover:underline"
            >
              {t("dashboard")} →
            </Link>
          </div>
        </>
      )}
    </div>
  );
}

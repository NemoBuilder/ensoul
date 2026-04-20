"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { shellApi, type Shell } from "@/lib/api";
import SoulCard from "@/components/SoulCard";

export default function MySoulsPage() {
  const t = useTranslations("MySouls");
  const { address, isConnected } = useAccount();
  const [mySouls, setMySouls] = useState<Shell[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const fetchMySouls = useCallback(async () => {
    if (!address) return;
    setLoading(true);
    setError("");

    try {
      const result = await shellApi.byOwner(address);
      setMySouls(result.shells || []);
    } catch (err) {
      console.error("Failed to fetch my souls:", err);
      setError(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [address, t]);

  useEffect(() => {
    if (isConnected && address) {
      fetchMySouls();
    } else {
      setMySouls([]);
    }
  }, [isConnected, address, fetchMySouls]);

  return (
    <div className="mx-auto max-w-7xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {!isConnected && (
        <div className="flex flex-col items-center gap-4 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <span className="text-4xl">👻</span>
          <p className="text-[#94a3b8]">{t("connectPrompt")}</p>
          <ConnectButton />
        </div>
      )}

      {isConnected && loading && (
        <div className="flex flex-col items-center gap-3 py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
          <p className="text-sm text-[#94a3b8]">{t("checking")}</p>
        </div>
      )}

      {isConnected && !loading && error && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-6 text-center">
          <p className="text-red-400">{error}</p>
          <button
            onClick={fetchMySouls}
            className="mt-3 rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white hover:bg-[#a78bfa]"
          >
            {t("retry")}
          </button>
        </div>
      )}

      {isConnected && !loading && !error && mySouls.length === 0 && (
        <div className="flex flex-col items-center gap-4 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <span className="text-4xl">🌱</span>
          <p className="text-[#94a3b8]">{t("empty")}</p>
          <Link
            href="/mint"
            className="rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
          >
            {t("mintFirst")}
          </Link>
        </div>
      )}

      {isConnected && !loading && !error && mySouls.length > 0 && (
        <>
          <p className="mb-4 text-sm text-[#94a3b8]">
            {t("ownedBy", { count: mySouls.length })}{" "}
            <span className="font-mono text-[#8b5cf6]">
              {address?.slice(0, 6)}...{address?.slice(-4)}
            </span>
          </p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {mySouls.map((soul) => (
              <SoulCard key={soul.id} soul={soul} />
            ))}
          </div>
        </>
      )}
    </div>
  );
}

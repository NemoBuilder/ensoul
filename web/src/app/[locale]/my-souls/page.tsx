"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { useAccount, usePublicClient } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { parseAbi } from "viem";
import { shellApi, type Shell } from "@/lib/api";
import SoulCard from "@/components/SoulCard";

const IDENTITY_REGISTRY = "0x8004A169FB4a3325136EB29fA0ceB6D2e539a432" as `0x${string}`;

const ERC721_ABI = parseAbi([
  "function ownerOf(uint256 tokenId) view returns (address)",
]);

export default function MySoulsPage() {
  const t = useTranslations("MySouls");
  const { address, isConnected } = useAccount();
  const publicClient = usePublicClient();
  const [mySouls, setMySouls] = useState<Shell[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const fetchMySouls = useCallback(async () => {
    if (!address || !publicClient) return;
    setLoading(true);
    setError("");

    try {
      const result = await shellApi.list({ limit: 500 });
      const mintedShells = (result.shells || []).filter(
        (s) => s.agent_id != null && s.agent_id > 0
      );

      if (mintedShells.length === 0) {
        setMySouls([]);
        setLoading(false);
        return;
      }

      const ownerResults = await publicClient.multicall({
        contracts: mintedShells.map((s) => ({
          address: IDENTITY_REGISTRY,
          abi: ERC721_ABI,
          functionName: "ownerOf",
          args: [BigInt(s.agent_id!)],
        })),
        allowFailure: true,
      });

      const owned: Shell[] = [];
      for (let i = 0; i < mintedShells.length; i++) {
        const res = ownerResults[i];
        if (
          res.status === "success" &&
          (res.result as string).toLowerCase() === address.toLowerCase()
        ) {
          owned.push(mintedShells[i]);
        }
      }

      setMySouls(owned);
    } catch (err) {
      console.error("Failed to fetch my souls:", err);
      setError(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [address, publicClient, t]);

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

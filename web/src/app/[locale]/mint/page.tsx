"use client";

import { useState } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import { shellApi, mintV2Api, SeedPreview, MintPriceInfo, MintPermitResponse } from "@/lib/api";
import { dimensionLabels } from "@/lib/utils";
import RadarChart from "@/components/RadarChart";
import {
  useAccount,
  useChainId,
  useSwitchChain,
  useSignMessage,
  useWriteContract,
  usePublicClient,
} from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { bsc } from "wagmi/chains";
import { parseAbi, decodeEventLog } from "viem";

const IDENTITY_REGISTRY_ABI = parseAbi([
  "event Registered(uint256 indexed agentId, string agentURI, address indexed owner)",
]);

const ENSOUL_MINTER_V2_ADDRESS = (process.env.NEXT_PUBLIC_MINTER_V2_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const ENSOUL_MINTER_V2_ABI = parseAbi([
  "function mint(string agentURI, bytes32 handleHash, uint256 price, uint256 deadline, uint256 nonce, bytes signature) payable returns (uint256 agentId)",
  "event Minted(address indexed user, uint256 indexed agentId, bytes32 indexed handleHash, uint256 fee)",
]);

export default function MintPage() {
  const t = useTranslations("Mint");
  const router = useRouter();
  const { address, isConnected } = useAccount();
  const chainId = useChainId();
  const { switchChain } = useSwitchChain();
  const { signMessageAsync } = useSignMessage();
  const { writeContractAsync } = useWriteContract();
  const publicClient = usePublicClient();
  const isCorrectChain = chainId === bsc.id;

  const [handle, setHandle] = useState("");
  const [preview, setPreview] = useState<SeedPreview | null>(null);
  const [priceInfo, setPriceInfo] = useState<MintPriceInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [minting, setMinting] = useState(false);
  const [mintStep, setMintStep] = useState("");
  const [error, setError] = useState("");
  const [imgErr, setImgErr] = useState(false);

  const formatBNB = (price: number) => {
    const s = price < 0.001 ? price.toFixed(6) : price.toFixed(4);
    return s.replace(/\.?0+$/, "");
  };

  async function handlePreview() {
    if (!handle.trim()) return;
    setError("");
    setPreview(null);
    setPriceInfo(null);
    setImgErr(false);
    const cleanHandle = handle.trim().replace(/^@/, "");
    if (!/^[a-zA-Z0-9_]{1,15}$/.test(cleanHandle)) {
      setError("Invalid handle: only letters, numbers, and underscores are allowed (max 15 characters)");
      return;
    }

    if (address) {
      try {
        const quota = await shellApi.mintQuota(address);
        if (!quota.can_mint) {
          setError(t("mintLimitReached", { minted: quota.minted, limit: quota.limit }));
          return;
        }
      } catch {}
    }

    setLoading(true);
    try {
      const [data, price] = await Promise.all([
        shellApi.preview(cleanHandle),
        mintV2Api.getPrice(cleanHandle),
      ]);
      if (price.already_minted) {
        setError(t("handleAlreadyMinted", { handle: cleanHandle }));
        setLoading(false);
        return;
      }
      setPreview(data);
      setPriceInfo(price);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Preview failed");
    } finally {
      setLoading(false);
    }
  }

  function buildAgentURI(p: SeedPreview): string {
    const regFile = {
      type: "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
      name: `@${p.handle} · Ensoul`,
      description: p.seed_summary,
      image: p.avatar_url,
      services: [
        { name: "web", endpoint: `https://ensoul.ac/soul/${p.handle}` },
        { name: "chat", endpoint: `https://ensoul.ac/soul/${p.handle}/chat` },
      ],
      active: true,
      ensoul: { handle: p.handle, stage: "embryo", dnaVersion: 1 },
    };
    const json = JSON.stringify(regFile);
    const base64 = btoa(unescape(encodeURIComponent(json)));
    return `data:application/json;base64,${base64}`;
  }

  async function handleMint() {
    if (!preview || !address || !priceInfo) return;
    setError("");
    setMinting(true);
    let signature = "";
    let dbCreated = false;
    let chainSuccess = false;

    try {
      // Step 1: Sign wallet message
      setMintStep(t("stepSign"));
      const message = `ensoul:mint:${preview.handle}`;
      signature = await signMessageAsync({ message });

      // Step 2: Create DB record
      setMintStep(t("stepBackend"));
      await shellApi.mint(preview.handle, address, signature, preview);
      dbCreated = true;

      // Step 3: Get permit from backend
      setMintStep(t("stepPermit"));
      const permitData: MintPermitResponse = await mintV2Api.getPermit(preview.handle, address, signature);

      // Step 4: Send on-chain tx with permit
      setMintStep(t("stepChain"));
      const agentURI = buildAgentURI(preview);

      const txHash = await writeContractAsync({
        address: ENSOUL_MINTER_V2_ADDRESS,
        abi: ENSOUL_MINTER_V2_ABI,
        functionName: "mint",
        args: [
          agentURI,
          (permitData.permit.handle_hash.startsWith("0x") ? permitData.permit.handle_hash : `0x${permitData.permit.handle_hash}`) as `0x${string}`,
          BigInt(permitData.permit.price),
          BigInt(permitData.permit.deadline),
          BigInt(permitData.permit.nonce),
          (permitData.permit.signature.startsWith("0x") ? permitData.permit.signature : `0x${permitData.permit.signature}`) as `0x${string}`,
        ],
        value: BigInt(permitData.price_wei),
        chainId: bsc.id,
      });

      // Step 5: Wait for confirmation
      setMintStep(t("stepConfirm"));
      const receipt = await publicClient!.waitForTransactionReceipt({ hash: txHash });
      chainSuccess = receipt.status === "success";

      if (!chainSuccess) {
        throw new Error("On-chain transaction reverted");
      }

      let agentId = 0;
      for (const log of receipt.logs) {
        try {
          const decoded = decodeEventLog({
            abi: IDENTITY_REGISTRY_ABI,
            data: log.data,
            topics: log.topics,
          });
          if (decoded.eventName === "Registered") {
            agentId = Number((decoded.args as { agentId: bigint }).agentId);
            break;
          }
        } catch {}
      }
      if (agentId === 0) {
        for (const log of receipt.logs) {
          try {
            const decoded = decodeEventLog({
              abi: ENSOUL_MINTER_V2_ABI,
              data: log.data,
              topics: log.topics,
            });
            if (decoded.eventName === "Minted") {
              agentId = Number((decoded.args as { agentId: bigint }).agentId);
              break;
            }
          } catch {}
        }
      }

      await shellApi.confirm(preview.handle, txHash, address, signature, agentId);
      router.push(`/soul/${preview.handle}`);
    } catch (err: unknown) {
      if (dbCreated && !chainSuccess && preview && address && signature) {
        try {
          await shellApi.cancel(preview.handle, address, signature);
        } catch {}
      }
      setError(err instanceof Error ? err.message : "Minting failed");
      setMinting(false);
      setMintStep("");
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>
      <p className="mb-8 text-[#94a3b8]">{t("subtitle")}</p>

      {!isConnected ? (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
          <h3 className="mb-3 text-lg font-bold text-[#e2e8f0]">{t("connectToMint")}</h3>
          <ConnectButton />
        </div>
      ) : !isCorrectChain ? (
        <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-8 text-center">
          <h3 className="mb-3 text-lg font-bold text-[#e2e8f0]">{t("switchToBSC")}</h3>
          <button
            onClick={() => switchChain({ chainId: bsc.id })}
            className="rounded-lg bg-yellow-500 px-8 py-3 text-sm font-bold text-white transition-colors hover:bg-yellow-400"
          >
            {t("switchToBSC")}
          </button>
        </div>
      ) : (
      <>
      <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
        <label className="mb-2 block text-sm font-medium text-[#e2e8f0]">
          Twitter Handle
        </label>
        <div className="flex gap-3">
          <div className="flex flex-1 items-center rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-4">
            <span className="text-[#94a3b8]">@</span>
            <input
              type="text"
              placeholder={t("handlePlaceholder").replace("@", "")}
              value={handle}
              onChange={(e) => setHandle(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handlePreview()}
              className="w-full bg-transparent px-2 py-3 text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none"
              disabled={loading}
            />
          </div>
          <button
            onClick={handlePreview}
            disabled={loading || !handle.trim()}
            className="rounded-md bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {loading ? t("previewing") : t("preview")}
          </button>
        </div>
      </div>

      {error && (
        <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-400">
          {error}
        </div>
      )}

      {loading && (
        <div className="mt-8 flex flex-col items-center gap-3 py-12">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
          <p className="text-sm text-[#94a3b8]">{t("previewing")}</p>
        </div>
      )}

      {preview && !loading && (
        <div className="mt-8 space-y-6">
          <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
            <div className="mb-4 flex items-center gap-4">
              <div className="relative h-16 w-16 overflow-hidden rounded-full border border-[#1e1e2e]">
                {preview.avatar_url && !imgErr ? (
                  <Image
                    src={preview.avatar_url}
                    alt={preview.handle}
                    fill
                    className="object-cover"
                    onError={() => setImgErr(true)}
                    unoptimized
                  />
                ) : (
                  <div className="flex h-full w-full items-center justify-center bg-[#1e1e2e] text-xl text-[#94a3b8]">
                    {preview.handle[0]?.toUpperCase() || "?"}
                  </div>
                )}
              </div>
              <div>
                <h2 className="text-xl font-bold text-[#e2e8f0]">
                  {preview.display_name}
                </h2>
                <p className="text-sm text-[#94a3b8]">@{preview.handle}</p>
              </div>
            </div>
            <p className="text-sm leading-relaxed text-[#94a3b8]">
              {preview.seed_summary}
            </p>
          </div>

          {/* Price info card */}
          {priceInfo && (
            <div className="rounded-lg border border-[#8b5cf6]/20 bg-[#8b5cf6]/5 p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-sm text-[#94a3b8]">
                    {priceInfo.followers.toLocaleString()} {t("followers")} · {t("tier")}: {priceInfo.tier}
                  </span>
                </div>
                <span className="font-mono text-lg font-bold text-[#8b5cf6]">
                  {formatBNB(priceInfo.price_bnb)} BNB
                </span>
              </div>
            </div>
          )}

          <div className="grid gap-6 lg:grid-cols-2">
            <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
              <h3 className="mb-4 text-sm font-medium text-[#94a3b8]">
                Soul Dimensions
              </h3>
              <RadarChart dimensions={preview.dimensions} size={260} />
            </div>
            <div className="space-y-3">
              {Object.entries(preview.dimensions).map(([key, dim]) => (
                <div
                  key={key}
                  className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-4"
                >
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-[#e2e8f0]">
                      {dimensionLabels[key] || key}
                    </span>
                    <span className="font-mono text-sm font-bold text-[#8b5cf6]">
                      {dim.score}
                    </span>
                  </div>
                  <div className="mb-2 h-1 overflow-hidden rounded-full bg-[#1e1e2e]">
                    <div
                      className="h-full rounded-full bg-[#8b5cf6]"
                      style={{ width: `${Math.min(dim.score, 100)}%` }}
                    />
                  </div>
                  <p className="text-xs text-[#94a3b8]">{dim.summary}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-lg border border-[#8b5cf6]/30 bg-[#8b5cf6]/5 p-6 text-center">
            <button
              onClick={handleMint}
              disabled={minting}
              className="rounded-lg bg-[#8b5cf6] px-8 py-3 text-sm font-bold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {minting
                ? t("minting")
                : `${t("mintNow")} (${priceInfo ? formatBNB(priceInfo.price_bnb) : "..."} BNB)`}
            </button>
            {mintStep && (
              <p className="mt-3 text-sm text-[#8b5cf6]">{mintStep}</p>
            )}
          </div>
        </div>
      )}
      </>
      )}
    </div>
  );
}

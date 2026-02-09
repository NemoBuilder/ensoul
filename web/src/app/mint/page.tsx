"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { shellApi, SeedPreview } from "@/lib/api";
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

// ERC-8004 Identity Registry (for parsing Registered event from inner call)
const IDENTITY_REGISTRY_ABI = parseAbi([
  "event Registered(uint256 indexed agentId, string agentURI, address indexed owner)",
]);

// EnsoulMinter wrapper contract (charges BNB fee → calls register → transfers NFT to user)
const ENSOUL_MINTER_ADDRESS = (process.env.NEXT_PUBLIC_MINTER_ADDRESS || "0x0000000000000000000000000000000000000000") as `0x${string}`;
const ENSOUL_MINTER_ABI = parseAbi([
  "function mint(string agentURI) payable returns (uint256 agentId)",
  "function mintFee() view returns (uint256)",
  "event Minted(address indexed user, uint256 indexed agentId, uint256 fee)",
]);

// Default mint fee (~1 USD in BNB, fixed at 700 USD/BNB), overridden by on-chain read
const DEFAULT_MINT_FEE = BigInt("1430000000000000"); // 0.00143 BNB ≈ $1 @700U

export default function MintPage() {
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
  const [loading, setLoading] = useState(false);
  const [minting, setMinting] = useState(false);
  const [mintStep, setMintStep] = useState("");
  const [error, setError] = useState("");
  const [imgErr, setImgErr] = useState(false);
  const [mintFee, setMintFee] = useState<bigint>(DEFAULT_MINT_FEE);

  // Read the current mint fee from the contract on mount
  useEffect(() => {
    if (!publicClient || !isConnected || !isCorrectChain) return;
    if (ENSOUL_MINTER_ADDRESS === "0x0000000000000000000000000000000000000000") return;
    publicClient.readContract({
      address: ENSOUL_MINTER_ADDRESS,
      abi: ENSOUL_MINTER_ABI,
      functionName: "mintFee",
    }).then((fee) => {
      setMintFee(fee as bigint);
    }).catch(() => {
      // Use default fee if read fails
    });
  }, [publicClient, isConnected, isCorrectChain]);

  const formatBNB = (wei: bigint) => {
    const bnb = Number(wei) / 1e18;
    return bnb < 0.001 ? bnb.toFixed(6) : bnb.toFixed(4);
  };

  // Preview seed extraction
  async function handlePreview() {
    if (!handle.trim()) return;
    setError("");
    setPreview(null);
    setImgErr(false);
    // Validate handle: only ASCII letters, numbers, underscores (Twitter format)
    const cleanHandle = handle.trim().replace(/^@/, "");
    if (!/^[a-zA-Z0-9_]{1,15}$/.test(cleanHandle)) {
      setError("Invalid handle: only letters, numbers, and underscores are allowed (max 15 characters)");
      return;
    }
    setLoading(true);
    try {
      const data = await shellApi.preview(cleanHandle);
      setPreview(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Preview failed");
    } finally {
      setLoading(false);
    }
  }

  // Build ERC-8004 AgentRegistrationFile data URI
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
      // Custom Ensoul metadata (allowed by spec — extra fields are MAY)
      ensoul: { handle: p.handle, stage: "embryo", dnaVersion: 1 },
    };
    const json = JSON.stringify(regFile);
    const base64 = btoa(unescape(encodeURIComponent(json)));
    return `data:application/json;base64,${base64}`;
  }

  // Full mint flow: sign → save DB → on-chain register → confirm
  async function handleMint() {
    if (!preview || !address) return;
    setError("");
    setMinting(true);

    try {
      // Step 1: Sign message to prove wallet ownership
      setMintStep("Requesting wallet signature...");
      const message = `ensoul:mint:${preview.handle}`;
      const signature = await signMessageAsync({ message });

      // Step 2: Save to database (with preview data, no LLM re-fetch)
      setMintStep("Saving soul data...");
      await shellApi.mint(preview.handle, address, signature, preview);

      // Step 3: On-chain registration via EnsoulMinter (user pays mint fee + gas)
      setMintStep(`Minting on BNB Chain — ${formatBNB(mintFee)} BNB (confirm in wallet)...`);
      const agentURI = buildAgentURI(preview);

      const txHash = await writeContractAsync({
        address: ENSOUL_MINTER_ADDRESS,
        abi: ENSOUL_MINTER_ABI,
        functionName: "mint",
        args: [agentURI],
        value: mintFee,
        chainId: bsc.id,
      });

      // Step 4: Wait for tx confirmation and parse agentId from event log
      setMintStep("Waiting for transaction confirmation...");
      const receipt = await publicClient!.waitForTransactionReceipt({ hash: txHash });

      let agentId = 0;
      // Try parsing Registered event from IdentityRegistry (inner call)
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
        } catch {
          // not our event, skip
        }
      }
      // Fallback: try parsing Minted event from EnsoulMinter
      if (agentId === 0) {
        for (const log of receipt.logs) {
          try {
            const decoded = decodeEventLog({
              abi: ENSOUL_MINTER_ABI,
              data: log.data,
              topics: log.topics,
            });
            if (decoded.eventName === "Minted") {
              agentId = Number((decoded.args as { agentId: bigint }).agentId);
              break;
            }
          } catch {
            // not our event, skip
          }
        }
      }

      setMintStep("Confirming on-chain registration...");
      await shellApi.confirm(preview.handle, txHash, agentId);

      router.push(`/soul/${preview.handle}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Minting failed");
      setMinting(false);
      setMintStep("");
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">Mint a Shell</h1>
      <p className="mb-8 text-[#94a3b8]">
        Create a DNA NFT for a public figure. We analyze their Twitter presence
        to extract an initial soul seed.
      </p>

      {/* Wallet gate: must connect wallet on BSC before anything */}
      {!isConnected ? (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
          <h3 className="mb-3 text-lg font-bold text-[#e2e8f0]">Connect Your Wallet</h3>
          <p className="mb-5 text-sm text-[#94a3b8]">
            You need to connect your wallet on BNB Smart Chain before minting.
          </p>
          <ConnectButton />
        </div>
      ) : !isCorrectChain ? (
        <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-8 text-center">
          <h3 className="mb-3 text-lg font-bold text-[#e2e8f0]">Wrong Network</h3>
          <p className="mb-5 text-sm text-[#94a3b8]">
            Please switch to BNB Smart Chain to continue.
          </p>
          <button
            onClick={() => switchChain({ chainId: bsc.id })}
            className="rounded-lg bg-yellow-500 px-8 py-3 text-sm font-bold text-white transition-colors hover:bg-yellow-400"
          >
            Switch to BNB Smart Chain
          </button>
        </div>
      ) : (
      <>
      {/* Input form */}
      <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
        <label className="mb-2 block text-sm font-medium text-[#e2e8f0]">
          Twitter Handle
        </label>
        <div className="flex gap-3">
          <div className="flex flex-1 items-center rounded-md border border-[#1e1e2e] bg-[#0a0a0f] px-4">
            <span className="text-[#94a3b8]">@</span>
            <input
              type="text"
              placeholder="elonmusk"
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
            {loading ? "Analyzing..." : "Preview"}
          </button>
        </div>
        <p className="mt-3 text-xs text-[#94a3b8]">
          Our AI analyzes recent tweets to build an initial personality profile
          across 6 dimensions.
        </p>
      </div>

      {/* Error */}
      {error && (
        <div className="mt-4 rounded-lg border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Loading state */}
      {loading && (
        <div className="mt-8 flex flex-col items-center gap-3 py-12">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#8b5cf6] border-t-transparent" />
          <p className="text-sm text-[#94a3b8]">
            Fetching profile & analyzing soul dimensions...
          </p>
        </div>
      )}

      {/* Preview result */}
      {preview && !loading && (
        <div className="mt-8 space-y-6">
          {/* Profile header */}
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

          {/* Radar + Dimensions */}
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

          {/* Mint action */}
          <div className="rounded-lg border border-[#8b5cf6]/30 bg-[#8b5cf6]/5 p-6 text-center">
            <h3 className="mb-2 text-lg font-bold text-[#e2e8f0]">
              Ready to Mint?
            </h3>
            <p className="mb-4 text-sm text-[#94a3b8]">
              This will register an ERC-8004 identity on BNB Chain (≈$1 in BNB).
              The shell starts as an embryo and evolves as Claws contribute fragments.
            </p>
            <button
              onClick={handleMint}
              disabled={minting}
              className="rounded-lg bg-[#8b5cf6] px-8 py-3 text-sm font-bold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {minting ? "Minting..." : `Mint Shell (${formatBNB(mintFee)} BNB + Gas)`}
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

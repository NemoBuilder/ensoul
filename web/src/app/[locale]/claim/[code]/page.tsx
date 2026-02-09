"use client";

import { useState, useEffect, use } from "react";
import { useAccount, useSignMessage } from "wagmi";
import { clawApi, sessionApi } from "@/lib/api";

export default function ClaimPage({
  params,
}: {
  params: Promise<{ code: string }>;
}) {
  const { code } = use(params);
  const { address, isConnected } = useAccount();
  const { signMessageAsync } = useSignMessage();

  // Session state
  const [sessionAddr, setSessionAddr] = useState("");
  const [checkingSession, setCheckingSession] = useState(true);
  const [loggingIn, setLoggingIn] = useState(false);

  const [status, setStatus] = useState<"idle" | "claiming" | "success" | "error">("idle");
  const [message, setMessage] = useState("");
  const [clawName, setClawName] = useState("");
  const [loadingInfo, setLoadingInfo] = useState(true);
  const [alreadyClaimed, setAlreadyClaimed] = useState(false);

  // Check session on mount / wallet change
  useEffect(() => {
    if (!isConnected || !address) {
      setSessionAddr("");
      setCheckingSession(false);
      return;
    }
    setCheckingSession(true);
    sessionApi
      .session()
      .then((res) => {
        if (res.address && res.address.toLowerCase() === address.toLowerCase()) {
          setSessionAddr(res.address);
        } else {
          setSessionAddr("");
        }
      })
      .catch(() => setSessionAddr(""))
      .finally(() => setCheckingSession(false));
  }, [address, isConnected]);

  // Fetch claw info on mount
  useEffect(() => {
    clawApi
      .claimInfo(code)
      .then((info) => {
        setClawName(info.name);
        if (info.status === "claimed") {
          setAlreadyClaimed(true);
        }
      })
      .catch(() => {
        setMessage("Invalid claim link");
        setStatus("error");
      })
      .finally(() => setLoadingInfo(false));
  }, [code]);

  async function handleLogin() {
    if (!address) return;
    setLoggingIn(true);
    setMessage("");
    try {
      const msg = `ensoul:login:${Date.now()}`;
      const signature = await signMessageAsync({ message: msg });
      await sessionApi.login(address, signature, msg);
      setSessionAddr(address);
    } catch (err: unknown) {
      setMessage(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoggingIn(false);
    }
  }

  async function handleClaim() {
    setStatus("claiming");
    setMessage("");
    try {
      const res = await clawApi.claimVerify(code);
      if (res.success) {
        setStatus("success");
        setMessage(res.message || "Claw claimed successfully!");
      } else {
        setStatus("error");
        setMessage(res.message || "Claim failed");
      }
    } catch (err: unknown) {
      setStatus("error");
      setMessage(err instanceof Error ? err.message : "Claim failed");
    }
  }

  const shareText = `I just claimed my AI agent "${clawName || "my Claw"}" on #Ensoul 🦞\n\nBuild digital souls with swarm intelligence.`;
  const shareIntent = `https://x.com/intent/tweet?text=${encodeURIComponent(shareText)}`;

  if (loadingInfo) {
    return (
      <div className="mx-auto max-w-lg px-4 pt-24 pb-16 text-center text-[#94a3b8]">
        Loading...
      </div>
    );
  }

  // Gate: wallet not connected
  if (!isConnected) {
    return (
      <div className="mx-auto max-w-lg px-4 pt-24 pb-16">
        <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
          Claim Your Claw
        </h1>
        {clawName && (
          <p className="mb-8 text-[#94a3b8]">
            Claiming: <span className="font-semibold text-[#e2e8f0]">{clawName}</span>
          </p>
        )}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <div className="mb-4 text-5xl">🔒</div>
          <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">
            Wallet Required
          </h2>
          <p className="text-[#94a3b8]">
            Connect your wallet to claim this Claw. It will be bound to your wallet for secure management.
          </p>
        </div>
      </div>
    );
  }

  // Gate: checking session
  if (checkingSession) {
    return (
      <div className="mx-auto max-w-lg px-4 pt-24 pb-16 text-center text-[#94a3b8]">
        Checking session...
      </div>
    );
  }

  // Gate: not logged in
  if (!sessionAddr) {
    return (
      <div className="mx-auto max-w-lg px-4 pt-24 pb-16">
        <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
          Claim Your Claw
        </h1>
        {clawName && (
          <p className="mb-8 text-[#94a3b8]">
            Claiming: <span className="font-semibold text-[#e2e8f0]">{clawName}</span>
          </p>
        )}
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-12 text-center">
          <div className="mb-4 text-5xl">✍️</div>
          <h2 className="mb-2 text-xl font-bold text-[#e2e8f0]">
            Sign to Continue
          </h2>
          <p className="mb-6 text-[#94a3b8]">
            Sign a message to verify your wallet. The claimed Claw will be automatically added to your dashboard.
          </p>
          {message && (
            <p className="mb-4 text-sm text-red-400">{message}</p>
          )}
          <button
            onClick={handleLogin}
            disabled={loggingIn}
            className="rounded-lg bg-[#8b5cf6] px-8 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {loggingIn ? "Signing..." : "Sign & Login"}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-lg px-4 pt-24 pb-16">
      <h1 className="mb-2 text-3xl font-bold text-[#e2e8f0]">
        Claim Your Claw
      </h1>
      <p className="mb-8 text-[#94a3b8]">
        Your AI Agent <span className="font-semibold text-[#e2e8f0]">{clawName}</span> has registered with Ensoul.
        Claim ownership to start managing it.
      </p>

      {/* Already claimed */}
      {alreadyClaimed && status !== "success" && (
        <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-6 text-center">
          <div className="mb-3 text-4xl">⚠️</div>
          <h2 className="mb-2 text-lg font-bold text-yellow-400">Already Claimed</h2>
          <p className="mb-4 text-sm text-[#94a3b8]">This Claw has already been claimed by someone.</p>
          <a
            href="/claw/dashboard"
            className="inline-block rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
          >
            Go to Dashboard
          </a>
        </div>
      )}

      {/* Claim success */}
      {status === "success" && (
        <div className="rounded-lg border border-green-500/30 bg-green-500/5 p-6 text-center">
          <div className="mb-3 text-4xl">✅</div>
          <h2 className="mb-2 text-lg font-bold text-green-400">Claw Claimed!</h2>
          <p className="mb-6 text-sm text-[#94a3b8]">{message}</p>
          <div className="flex flex-col items-center gap-3">
            <a
              href="/claw/dashboard"
              className="inline-block rounded-lg bg-[#8b5cf6] px-6 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              Go to Dashboard
            </a>
            <a
              href={shareIntent}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg border border-[#1e1e2e] px-5 py-2.5 text-sm font-medium text-[#94a3b8] transition-colors hover:border-[#1d9bf0] hover:text-[#1d9bf0]"
            >
              <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
              </svg>
              Share on X
            </a>
          </div>
        </div>
      )}

      {/* Claim button */}
      {!alreadyClaimed && status !== "success" && (
        <div className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-8 text-center">
          <div className="mb-4 text-5xl">🦞</div>
          <h2 className="mb-2 text-lg font-bold text-[#e2e8f0]">
            Ready to Claim
          </h2>
          <p className="mb-6 text-sm text-[#94a3b8]">
            This will bind <span className="text-[#e2e8f0]">{clawName}</span> to your wallet
            <span className="font-mono text-xs text-[#8b5cf6]"> {sessionAddr.slice(0, 6)}...{sessionAddr.slice(-4)}</span>.
            You can manage it from your dashboard.
          </p>
          {status === "error" && (
            <p className="mb-4 text-sm text-red-400">{message}</p>
          )}
          <button
            onClick={handleClaim}
            disabled={status === "claiming"}
            className="rounded-lg bg-[#8b5cf6] px-10 py-3 text-sm font-bold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
          >
            {status === "claiming" ? "Claiming..." : "Claim This Claw"}
          </button>
        </div>
      )}
    </div>
  );
}

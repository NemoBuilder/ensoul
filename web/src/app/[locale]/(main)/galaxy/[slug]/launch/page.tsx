"use client";

// V4 公平发射 / Launch — 公共信息页。
//
// 显示这条 Galaxy 当前的发射状态、窗口剩余时间、已筹 / 目标、Token 地址等。
// 钱包接入：使用 wagmi + RainbowKit；用户可直签 deposit / claim / refund。

import { useEffect, useState, use as usePromise } from "react";
import Link from "next/link";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import {
  useAccount,
  useWriteContract,
  useWaitForTransactionReceipt,
} from "wagmi";
import { parseEther } from "viem";
import { launchApi, type Launch } from "@/lib/api";
import BscScanLink, { shortHash } from "@/components/BscScanLink";
import {
  FAIR_LAUNCH_ABI,
  fairLaunchAddress,
  galaxyIdToBytes32,
} from "@/lib/fair-launch";

type Props = { params: Promise<{ locale: string; slug: string }> };

// 把 wei (18 decimals) 显示成 BNB / 单位。保留 4 位小数；返回字符串。
function fmtWei(wei: string, unit = "BNB"): string {
  if (!wei || wei === "0") return `0 ${unit}`;
  // 用 BigInt 避免精度丢失。
  try {
    const E18 = BigInt("1000000000000000000");
    const v = BigInt(wei);
    const whole = v / E18;
    const frac = v % E18;
    const fracStr = frac.toString().padStart(18, "0").slice(0, 4);
    return `${whole.toString()}.${fracStr} ${unit}`;
  } catch {
    return `${wei} wei`;
  }
}

function statusBadge(s: Launch["status"]) {
  const map: Record<Launch["status"], string> = {
    draft: "bg-slate-500/20 text-slate-300",
    open: "bg-emerald-500/20 text-emerald-300",
    succeeded: "bg-[#8b5cf6]/20 text-[#8b5cf6]",
    failed: "bg-red-500/20 text-red-300",
  };
  return `rounded px-2 py-0.5 text-xs ${map[s] ?? map.draft}`;
}

function remaining(endIso: string): string {
  const ms = new Date(endIso).getTime() - Date.now();
  if (ms <= 0) return "已结束";
  const h = Math.floor(ms / 3_600_000);
  const m = Math.floor((ms % 3_600_000) / 60_000);
  if (h >= 24) return `剩余 ${Math.floor(h / 24)}d ${h % 24}h`;
  return `剩余 ${h}h ${m}m`;
}

export default function LaunchPage({ params }: Props) {
  const { slug } = usePromise(params);
  const [launch, setLaunch] = useState<Launch | null>(null);
  const [stage, setStage] = useState<string>("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    launchApi
      .get(slug)
      .then((r) => {
        setLaunch(r.launch);
        setStage(r.galaxy_stage);
      })
      .catch((e: Error) => setError(e.message));
  }, [slug]);

  if (error) {
    return (
      <div className="mx-auto max-w-3xl px-4 pt-24 pb-16">
        <div className="rounded-md border border-red-500/30 bg-red-500/10 p-4 text-red-300">
          {error}
        </div>
        <Link
          href={`../${slug}`}
          className="mt-4 inline-block text-sm text-[#8b5cf6] hover:underline"
        >
          ← 返回 Galaxy
        </Link>
      </div>
    );
  }
  if (!launch) {
    return <div className="mx-auto max-w-3xl px-4 pt-24 text-[#94a3b8]">加载中…</div>;
  }

  const cap = launch.max_raise_wei === "0" ? null : launch.max_raise_wei;
  const progressPct = (() => {
    try {
      const E15 = BigInt("1000000000000000");
      const cur = Number(BigInt(launch.total_raised_wei) / E15) / 1000;
      const tgt = Number(BigInt(launch.min_raise_wei) / E15) / 1000;
      if (tgt <= 0) return 0;
      return Math.min(100, (cur / tgt) * 100);
    } catch {
      return 0;
    }
  })();

  return (
    <div className="mx-auto max-w-3xl px-4 pt-24 pb-16">
      <Link
        href={`../${slug}`}
        className="mb-6 inline-block text-sm text-[#94a3b8] hover:text-[#8b5cf6]"
      >
        ← 返回 Galaxy
      </Link>

      <header className="mb-8 flex items-center gap-3">
        <h1 className="text-3xl font-bold text-[#e2e8f0]">公平发射</h1>
        <span className={statusBadge(launch.status)}>{launch.status}</span>
        <span className="text-xs text-[#64748b]">stage: {stage}</span>
      </header>

      {/* 进度条 */}
      <section className="mb-6 rounded-lg border border-[#1e293b] bg-[#0f172a] p-5">
        <div className="mb-2 flex items-baseline justify-between">
          <span className="text-sm text-[#94a3b8]">已筹 / 目标</span>
          <span className="text-sm font-medium text-[#e2e8f0]">
            {fmtWei(launch.total_raised_wei)} / {fmtWei(launch.min_raise_wei)}
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded bg-[#1e293b]">
          <div
            className="h-full bg-gradient-to-r from-[#8b5cf6] to-[#06b6d4]"
            style={{ width: `${progressPct}%` }}
          />
        </div>
        {cap && (
          <div className="mt-2 text-xs text-[#64748b]">
            硬顶 {fmtWei(cap)}（达到后不再接收存款）
          </div>
        )}
      </section>

      {/* 关键参数 */}
      <section className="mb-6 grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
        <Stat label="开始时间" value={new Date(launch.start_at).toLocaleString()} />
        <Stat label="结束时间" value={new Date(launch.end_at).toLocaleString()} />
        <Stat label="状态" value={launch.status === "open" ? remaining(launch.end_at) : "—"} />
        <Stat label="代币名称" value={launch.token_name || "—"} />
        <Stat label="代币符号" value={launch.token_symbol || "—"} />
        <Stat label="总供应" value={fmtWei(launch.supply_wei, launch.token_symbol || "TKN")} />
      </section>

      {/* 合约地址 */}
      {launch.token_addr && (
        <section className="mb-6 rounded-lg border border-[#1e293b] bg-[#0f172a] p-4 text-sm">
          <div className="mb-1 text-xs uppercase tracking-wider text-[#64748b]">Token 合约</div>
          <BscScanLink kind="token" value={launch.token_addr} className="break-all font-mono text-[#06b6d4] hover:underline">
            {launch.token_addr}
          </BscScanLink>
        </section>
      )}

      {/* 用户操作 — 钱包签名 */}
      <ParticipatePanel launch={launch} />

      {/* 取件 + 交易哈希审计 */}
      <MyDepositPanel slug={slug} launch={launch} />

      <section className="mt-6 space-y-1 text-xs text-[#64748b]">
        {launch.open_tx_hash && <TxLine label="open" hash={launch.open_tx_hash} />}
        {launch.set_token_tx_hash && <TxLine label="setToken" hash={launch.set_token_tx_hash} />}
        {launch.finalize_tx_hash && <TxLine label="finalize" hash={launch.finalize_tx_hash} />}
      </section>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-[#1e293b] bg-[#0f172a] p-3">
      <div className="text-xs text-[#64748b]">{label}</div>
      <div className="mt-0.5 text-[#e2e8f0]">{value}</div>
    </div>
  );
}

function TxLine({ label, hash }: { label: string; hash: string }) {
  return (
    <div>
      <span className="text-[#475569]">{label}:</span>{" "}
      <BscScanLink kind="tx" value={hash}>{shortHash(hash)}</BscScanLink>
    </div>
  );
}


// ─── ParticipatePanel — 钱包直签 deposit/claim/refund ────────────────────────

function ParticipatePanel({ launch }: { launch: Launch }) {
  const { isConnected } = useAccount();
  const addr = fairLaunchAddress();
  const [bnb, setBnb] = useState("0.05");
  const [err, setErr] = useState<string | null>(null);

  const { writeContract, data: txHash, isPending, reset } = useWriteContract();
  const { isLoading: mining, isSuccess: mined } = useWaitForTransactionReceipt({
    hash: txHash,
  });

  const gid = (() => {
    try {
      return galaxyIdToBytes32(launch.galaxy_id);
    } catch {
      return undefined;
    }
  })();

  const call = (method: "deposit" | "claim" | "refund", value?: bigint) => {
    setErr(null);
    if (!addr) {
      setErr("FairLaunch 合约地址未配置 (NEXT_PUBLIC_FAIR_LAUNCH_ADDR)");
      return;
    }
    if (!gid) {
      setErr("非法 galaxy id");
      return;
    }
    try {
      if (method === "deposit") {
        writeContract({
          address: addr,
          abi: FAIR_LAUNCH_ABI,
          functionName: "deposit",
          args: [gid],
          value: value ?? BigInt(0),
        });
      } else {
        writeContract({
          address: addr,
          abi: FAIR_LAUNCH_ABI,
          functionName: method,
          args: [gid],
        });
      }
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  return (
    <section className="rounded-lg border border-[#1e293b] bg-[#0f172a] p-5 text-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="font-medium text-[#e2e8f0]">参与</div>
        <ConnectButton chainStatus="icon" showBalance={false} accountStatus="address" />
      </div>

      {!isConnected ? (
        <p className="text-[#94a3b8]">请先连接钱包。</p>
      ) : !addr ? (
        <p className="text-red-300">
          缺少 <code>NEXT_PUBLIC_FAIR_LAUNCH_ADDR</code> 环境变量。
        </p>
      ) : launch.status === "draft" ? (
        <p className="text-[#94a3b8]">尚未开启，请等待管理员开启窗口。</p>
      ) : launch.status === "open" ? (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="flex-1">
            <span className="mb-1 block text-xs text-[#64748b]">金额 (BNB)</span>
            <input
              type="number"
              step="0.001"
              min="0"
              value={bnb}
              onChange={(e) => setBnb(e.target.value)}
              className="w-full rounded border border-[#1e293b] bg-[#0a0a14] px-3 py-2 text-[#e2e8f0]"
            />
          </label>
          <button
            disabled={isPending || mining}
            onClick={() => {
              try {
                call("deposit", parseEther(bnb));
              } catch {
                setErr("BNB 金额格式不合法");
              }
            }}
            className="rounded bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
          >
            {isPending ? "等待签名…" : mining ? "确认中…" : "Deposit"}
          </button>
        </div>
      ) : launch.status === "succeeded" ? (
        <button
          disabled={isPending || mining}
          onClick={() => call("claim")}
          className="rounded bg-[#8b5cf6] px-4 py-2 font-medium text-white hover:bg-[#7c3aed] disabled:opacity-50"
        >
          {isPending ? "等待签名…" : mining ? "确认中…" : "Claim 代币"}
        </button>
      ) : (
        <button
          disabled={isPending || mining}
          onClick={() => call("refund")}
          className="rounded bg-red-600 px-4 py-2 font-medium text-white hover:bg-red-500 disabled:opacity-50"
        >
          {isPending ? "等待签名…" : mining ? "确认中…" : "Refund BNB"}
        </button>
      )}

      {err && <p className="mt-3 text-xs text-red-300">{err}</p>}
      {txHash && (
        <p className="mt-3 text-xs text-[#64748b]">
          tx: <code className="text-[#06b6d4]">{txHash}</code>{" "}
          {mined && <span className="text-emerald-400">✓ 已确认</span>}
          {mined && (
            <button
              onClick={() => {
                reset();
                window.location.reload();
              }}
              className="ml-2 underline"
            >
              刷新
            </button>
          )}
        </p>
      )}
    </section>
  );
}
// ─── MyDepositPanel — 显示当前钱包的投入 + 预估代币份额 ──────────────────────

function MyDepositPanel({ slug, launch }: { slug: string; launch: Launch }) {
  const { address, isConnected } = useAccount();
  const [dep, setDep] = useState<{
    amount_wei: string;
    claimed: boolean;
    refunded: boolean;
    projected_tokens: string;
    projected_ratio_pp: string;
  } | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!address) {
      setDep(null);
      return;
    }
    launchApi
      .myDeposit(slug, address)
      .then((r) =>
        setDep({
          amount_wei: r.amount_wei,
          claimed: r.claimed,
          refunded: r.refunded,
          projected_tokens: r.projected_tokens,
          projected_ratio_pp: r.projected_ratio_pp,
        })
      )
      .catch((e: Error) => setErr(e.message));
  }, [slug, address, launch.status]);

  if (!isConnected || !address) return null;
  if (err) return null;
  if (!dep) return null;
  const has = dep.amount_wei && dep.amount_wei !== "0";

  return (
    <section className="mt-6 rounded-lg border border-[#1e293b] bg-[#0f172a] p-5 text-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="font-medium text-[#e2e8f0]">我的份额</div>
        <BscScanLink kind="address" value={address}>
          {shortHash(address)}
        </BscScanLink>
      </div>
      {!has ? (
        <p className="text-[#94a3b8]">尚未参与本次发射。</p>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="已投入" value={fmtWei(dep.amount_wei)} />
          <Stat
            label="预估占比"
            value={`${(Number(dep.projected_ratio_pp) / 100).toFixed(2)}%`}
          />
          <Stat
            label="预估代币"
            value={fmtWei(dep.projected_tokens, launch.token_symbol || "TKN")}
          />
          <Stat
            label="状态"
            value={dep.claimed ? "已 Claim" : dep.refunded ? "已 Refund" : "待发射结束"}
          />
        </div>
      )}
    </section>
  );
}

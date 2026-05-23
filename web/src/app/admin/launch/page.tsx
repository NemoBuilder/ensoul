"use client";

// V4 公平发射 — 管理员控制台。
//
// 三步流程（按钮逐个解锁）：
//   1. 开启窗口     POST /api/v4/launch/:slug/open
//   2. 写入 Token   POST /api/v4/launch/:slug/token   (Token 由 forge/Remix 部署)
//   3. 终结         POST /api/v4/launch/:slug/finalize
//
// 故意保持 minimal：只服务运维流程，不做 fancy 表单。

import { useState } from "react";
import { launchApi, type Launch } from "@/lib/api";

export default function AdminLaunchConsole() {
  const [slug, setSlug] = useState("");
  const [launch, setLaunch] = useState<Launch | null>(null);
  const [stage, setStage] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  // 表单状态
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [minRaise, setMinRaise] = useState("");
  const [maxRaise, setMaxRaise] = useState("");
  const [supply, setSupply] = useState("");
  const [tokenName, setTokenName] = useState("");
  const [tokenSymbol, setTokenSymbol] = useState("");
  const [tokenAddr, setTokenAddr] = useState("");

  const loadLaunch = async () => {
    setMsg(null);
    if (!slug.trim()) return;
    try {
      const r = await launchApi.get(slug.trim());
      setLaunch(r.launch);
      setStage(r.galaxy_stage);
    } catch (e) {
      setLaunch(null);
      setStage("");
      setMsg((e as Error).message);
    }
  };

  const openLaunch = async () => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await launchApi.open(slug.trim(), {
        start_at: Math.floor(new Date(startAt).getTime() / 1000),
        end_at: Math.floor(new Date(endAt).getTime() / 1000),
        min_raise_wei: minRaise.trim(),
        max_raise_wei: maxRaise.trim() || "0",
        supply_wei: supply.trim(),
        token_name: tokenName.trim(),
        token_symbol: tokenSymbol.trim(),
      });
      setLaunch(r.launch);
      setMsg("✓ 已开启");
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const wireToken = async () => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await launchApi.setToken(slug.trim(), tokenAddr.trim());
      setLaunch(r.launch);
      setMsg("✓ Token 已写入");
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const finalize = async () => {
    setBusy(true);
    setMsg(null);
    try {
      const r = await launchApi.finalize(slug.trim());
      setLaunch(r.launch);
      setMsg(`✓ 已终结 (${r.launch.status})`);
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="mb-6 text-2xl font-bold text-[#e2e8f0]">公平发射 控制台</h1>

      <div className="mb-6 flex gap-2">
        <input
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
          placeholder="galaxy slug, e.g. satoshi"
          className="flex-1 rounded border border-[#1e1e2e] bg-[#0f172a] px-3 py-2 text-[#e2e8f0]"
        />
        <button
          onClick={loadLaunch}
          className="rounded bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed]"
        >
          加载
        </button>
      </div>

      {msg && (
        <div className="mb-4 rounded border border-[#1e1e2e] bg-[#0f172a] px-3 py-2 text-sm text-[#e2e8f0]">
          {msg}
        </div>
      )}

      {launch && (
        <div className="mb-6 rounded border border-[#1e1e2e] bg-[#0f172a] p-4 text-sm text-[#94a3b8]">
          <div>状态: <span className="text-[#e2e8f0]">{launch.status}</span> · stage: {stage}</div>
          <div>总募: {launch.total_raised_wei} wei · 目标: {launch.min_raise_wei} wei</div>
          <div>Token: <code className="text-[#06b6d4]">{launch.token_addr || "未写入"}</code></div>
        </div>
      )}

      {/* Step 1 — open */}
      <Section title="1. 开启募资窗口" disabled={!!launch && launch.status !== "draft"}>
        <Grid>
          <Field label="开始 (本地时间)" type="datetime-local" value={startAt} onChange={setStartAt} />
          <Field label="结束 (本地时间)" type="datetime-local" value={endAt} onChange={setEndAt} />
          <Field label="最低募资 (wei)" value={minRaise} onChange={setMinRaise} placeholder="e.g. 10000000000000000000  (=10 BNB)" />
          <Field label="硬顶 (wei, 留空=无限)" value={maxRaise} onChange={setMaxRaise} />
          <Field label="代币总供应 (wei)" value={supply} onChange={setSupply} placeholder="e.g. 1000000000000000000000000000  (=1B*1e18)" />
          <Field label="代币名" value={tokenName} onChange={setTokenName} />
          <Field label="代币符号" value={tokenSymbol} onChange={setTokenSymbol} />
        </Grid>
        <button
          disabled={busy || !slug}
          onClick={openLaunch}
          className="mt-3 rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
        >
          openLaunch
        </button>
      </Section>

      {/* Step 2 — set token */}
      <Section
        title="2. 写入 Token 合约地址"
        hint="先用 forge/Remix 部署 EnsoulCommunityToken，把 mintTo 设为 FairLaunch 地址，再把代币地址粘到这里。"
        disabled={!launch || launch.status !== "open" || !!launch.token_addr}
      >
        <Field label="Token 地址" value={tokenAddr} onChange={setTokenAddr} placeholder="0x…" />
        <button
          disabled={busy}
          onClick={wireToken}
          className="mt-3 rounded bg-[#06b6d4] px-4 py-2 text-sm font-medium text-white hover:bg-[#0891b2] disabled:opacity-50"
        >
          setToken
        </button>
      </Section>

      {/* Step 3 — finalize */}
      <Section title="3. 终结" disabled={!launch || launch.status !== "open"}>
        <p className="mb-3 text-sm text-[#94a3b8]">
          只能在窗口关闭后调用。达到最低募资 + Token 已写入 → succeeded（自动分账）；否则 failed（开放退款）。
        </p>
        <button
          disabled={busy}
          onClick={finalize}
          className="rounded bg-[#8b5cf6] px-4 py-2 text-sm font-medium text-white hover:bg-[#7c3aed] disabled:opacity-50"
        >
          finalize
        </button>
      </Section>
    </div>
  );
}

function Section({
  title,
  hint,
  disabled,
  children,
}: {
  title: string;
  hint?: string;
  disabled: boolean;
  children: React.ReactNode;
}) {
  return (
    <section
      className={`mb-5 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-5 ${
        disabled ? "opacity-40 pointer-events-none" : ""
      }`}
    >
      <h2 className="mb-2 text-lg font-semibold text-[#e2e8f0]">{title}</h2>
      {hint && <p className="mb-3 text-xs text-[#64748b]">{hint}</p>}
      {children}
    </section>
  );
}

function Grid({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">{children}</div>;
}

function Field({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-xs text-[#94a3b8]">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full rounded border border-[#1e1e2e] bg-[#0f172a] px-3 py-2 text-[#e2e8f0]"
      />
    </label>
  );
}

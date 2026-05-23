"use client";

// V4 Epoch 详情 — 列出 epoch 内所有 atom (leaves) + 单个 atom 的 Merkle proof
// 验证小工具（纯客户端 sha256 校验）。

import { useEffect, useState, use as usePromise } from "react";
import Link from "next/link";
import BscScanLink, { shortHash } from "@/components/BscScanLink";
import { epochApi, type Epoch, type EpochLeafRow, type AtomProof } from "@/lib/api";

type Props = { params: Promise<{ locale: string; id: string }> };

export default function EpochDetailPage({ params }: Props) {
  const { id } = usePromise(params);
  const [epoch, setEpoch] = useState<Epoch | null>(null);
  const [atoms, setAtoms] = useState<EpochLeafRow[]>([]);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    epochApi
      .get(id)
      .then((r) => {
        setEpoch(r.epoch);
        setAtoms(r.atoms || []);
      })
      .catch((e: Error) => setErr(e.message));
  }, [id]);

  if (err) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
        <div className="rounded border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {err}
        </div>
      </div>
    );
  }
  if (!epoch) {
    return <div className="mx-auto max-w-4xl px-4 pt-24 text-[#94a3b8]">加载中…</div>;
  }

  return (
    <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
      <Link
        href="../epoch"
        className="mb-6 inline-block text-sm text-[#94a3b8] hover:text-[#8b5cf6]"
      >
        ← 所有 epoch
      </Link>

      <header className="mb-6">
        <h1 className="text-3xl font-bold text-[#e2e8f0]">Epoch #{epoch.index}</h1>
        <p className="mt-1 text-sm text-[#94a3b8]">
          {epoch.atom_count} atoms · 关闭于 {new Date(epoch.closed_at).toLocaleString()}
        </p>
      </header>

      <section className="mb-6 rounded-lg border border-[#1e293b] bg-[#0f172a] p-4 text-sm">
        <div className="mb-2 text-xs uppercase tracking-wider text-[#64748b]">Merkle Root</div>
        <code className="break-all text-[#06b6d4]">0x{epoch.root}</code>
        {epoch.chain_tx_hash && (
          <div className="mt-3 text-xs text-[#64748b]">
            on-chain: <BscScanLink kind="tx" value={epoch.chain_tx_hash}>{shortHash(epoch.chain_tx_hash)}</BscScanLink>{" "}
            <span className="ml-2">@ block {epoch.chain_block ?? "—"}</span>
          </div>
        )}
      </section>

      <h2 className="mb-3 text-lg font-semibold text-[#e2e8f0]">Atoms</h2>
      <ul className="space-y-2">
        {atoms.map((a, i) => (
          <li
            key={a.id}
            className="rounded border border-[#1e293b] bg-[#0a0a14] p-3 text-sm"
          >
            <div className="flex items-center justify-between">
              <div>
                <span className="rounded bg-[#8b5cf6]/10 px-2 py-0.5 text-xs text-[#8b5cf6]">
                  {a.kind}
                </span>{" "}
                <span className="text-[#e2e8f0]">
                  {a.kind === "node" ? a.node_label : a.edge_label || "(edge)"}
                </span>
                <span className="ml-2 text-xs text-[#64748b]">#{i}</span>
              </div>
              <ProofToggle atomID={a.id} expectedRoot={epoch.root} />
            </div>
            <code className="mt-1 block break-all text-xs text-[#475569]">
              leaf 0x{a.merkle_leaf}
            </code>
          </li>
        ))}
      </ul>
    </div>
  );
}

// ─── Proof widget ───────────────────────────────────────────────────────────

function ProofToggle({ atomID, expectedRoot }: { atomID: string; expectedRoot: string }) {
  const [proof, setProof] = useState<AtomProof | null>(null);
  const [verified, setVerified] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const load = async () => {
    setBusy(true);
    setErr(null);
    try {
      const r = await epochApi.atomProof(atomID);
      setProof(r);
      const ok = await verifyProof(r.leaf, r.path, r.index, expectedRoot);
      setVerified(ok);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="text-xs">
      {!proof ? (
        <button
          onClick={load}
          disabled={busy}
          className="rounded border border-[#1e293b] px-2 py-1 text-[#94a3b8] hover:text-[#8b5cf6]"
        >
          {busy ? "校验中…" : "验证 Proof"}
        </button>
      ) : verified === null ? (
        "—"
      ) : verified ? (
        <span className="text-emerald-400">✓ {proof.path.length}-step proof OK</span>
      ) : (
        <span className="text-red-400">✗ 不匹配根</span>
      )}
      {err && <div className="mt-1 text-red-400">{err}</div>}
    </div>
  );
}

// ─── 纯客户端 Merkle 校验 (sha256, Bitcoin-style 奇数复制) ───────────────────

async function sha256(bytes: Uint8Array): Promise<Uint8Array> {
  // Copy into a fresh ArrayBuffer to satisfy strict BufferSource typing.
  const ab = new ArrayBuffer(bytes.length);
  new Uint8Array(ab).set(bytes);
  const buf = await crypto.subtle.digest("SHA-256", ab);
  return new Uint8Array(buf);
}

function hexToBytes(hex: string): Uint8Array {
  const h = hex.replace(/^0x/, "");
  const out = new Uint8Array(h.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(h.slice(i * 2, i * 2 + 2), 16);
  }
  return out;
}

function bytesToHex(b: Uint8Array): string {
  return Array.from(b)
    .map((x) => x.toString(16).padStart(2, "0"))
    .join("");
}

async function pair(left: Uint8Array, right: Uint8Array): Promise<Uint8Array> {
  const merged = new Uint8Array(64);
  merged.set(left, 0);
  merged.set(right, 32);
  return sha256(merged);
}

async function verifyProof(
  leafHex: string,
  pathHex: string[],
  index: number,
  rootHex: string
): Promise<boolean> {
  let h = hexToBytes(leafHex);
  let i = index;
  for (const sibHex of pathHex) {
    const sib = hexToBytes(sibHex);
    // Bitcoin style: 偶数索引 ⇒ 当前是左子；奇数 ⇒ 当前是右子。
    h = i % 2 === 0 ? await pair(h, sib) : await pair(sib, h);
    i = Math.floor(i / 2);
  }
  return bytesToHex(h) === rootHex.replace(/^0x/, "").toLowerCase();
}

"use client";

// V4 Epoch 探索器 — 列表。
// 公开访问。可选按 ?galaxy=slug 过滤。

import { useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { epochApi, type Epoch } from "@/lib/api";

export default function EpochListPage() {
  const sp = useSearchParams();
  const galaxy = sp.get("galaxy") || "";
  const [rows, setRows] = useState<Epoch[]>([]);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    epochApi
      .list(galaxy || undefined)
      .then((r) => setRows(r.epochs || []))
      .catch((e: Error) => setErr(e.message));
  }, [galaxy]);

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <header className="mb-6">
        <h1 className="text-3xl font-bold text-[#e2e8f0]">Epoch 探索器</h1>
        <p className="mt-1 text-sm text-[#94a3b8]">
          每个 Epoch 把若干 atoms 卷成一棵 Merkle 树，根写到 BSC 上链可审计。
          {galaxy && (
            <>
              {" "}过滤: <code className="text-[#06b6d4]">{galaxy}</code>{" "}
              <Link href="./" className="underline text-[#94a3b8]">清除</Link>
            </>
          )}
        </p>
      </header>

      {err && (
        <div className="mb-4 rounded border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {err}
        </div>
      )}

      {rows.length === 0 ? (
        <div className="text-[#94a3b8]">暂无 epoch。</div>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wider text-[#64748b]">
            <tr>
              <th className="p-2">#</th>
              <th className="p-2">Atoms</th>
              <th className="p-2">Root</th>
              <th className="p-2">链上</th>
              <th className="p-2">关闭时间</th>
            </tr>
          </thead>
          <tbody className="text-[#e2e8f0]">
            {rows.map((e) => (
              <tr key={e.id} className="border-t border-[#1e293b] hover:bg-[#0f172a]/50">
                <td className="p-2">
                  <Link href={`./epoch/${e.id}`} className="text-[#8b5cf6] hover:underline">
                    #{e.index}
                  </Link>
                </td>
                <td className="p-2">{e.atom_count}</td>
                <td className="p-2 font-mono text-xs text-[#06b6d4]">
                  {e.root.slice(0, 12)}…{e.root.slice(-8)}
                </td>
                <td className="p-2">
                  <span
                    className={
                      e.chain_status === "confirmed"
                        ? "text-emerald-400"
                        : e.chain_status === "failed"
                        ? "text-red-400"
                        : "text-[#64748b]"
                    }
                  >
                    {e.chain_status}
                  </span>
                </td>
                <td className="p-2 text-xs text-[#64748b]">
                  {new Date(e.closed_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

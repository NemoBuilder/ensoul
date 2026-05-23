"use client";

// 「我的贡献」仪表盘 — 按 galaxy 聚合用户贡献的 atoms 统计。
// 需要登录（email session 或钱包 session）。

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  meApi,
  type ContribGalaxyRow,
  type ContribSummary,
  ApiError,
} from "@/lib/api";

export default function MyContributionsPage() {
  const [summary, setSummary] = useState<ContribSummary | null>(null);
  const [rows, setRows] = useState<ContribGalaxyRow[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [needLogin, setNeedLogin] = useState(false);

  useEffect(() => {
    meApi
      .contributions()
      .then((r) => {
        setSummary(r.summary);
        setRows(r.galaxies || []);
      })
      .catch((e: unknown) => {
        if (e instanceof ApiError && e.status === 401) {
          setNeedLogin(true);
        } else {
          setErr((e as Error).message);
        }
      });
  }, []);

  if (needLogin) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
        <h1 className="text-2xl font-bold text-[#e2e8f0]">我的贡献</h1>
        <p className="mt-4 text-[#94a3b8]">
          请先登录（顶部 Login 按钮）以查看你的贡献统计。
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 pt-24 pb-16">
      <header className="mb-6">
        <h1 className="text-3xl font-bold text-[#e2e8f0]">我的贡献</h1>
        <p className="mt-1 text-sm text-[#94a3b8]">
          你贡献到各 Galaxy 的 atoms 状态分布。Curator 接受的越多 → 链上 Merkle 根承载得越多。
        </p>
      </header>

      {err && (
        <div className="mb-4 rounded border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {err}
        </div>
      )}

      {summary && (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-5">
          <Stat label="参与 Galaxy" value={summary.galaxy_count} accent="cyan" />
          <Stat label="Accepted" value={summary.total_accepted} accent="emerald" />
          <Stat label="Pending" value={summary.total_pending} accent="amber" />
          <Stat label="Disputed" value={summary.total_disputed} accent="rose" />
          <Stat
            label="平均置信度"
            value={summary.global_avg_confidence.toFixed(3)}
            accent="violet"
          />
        </div>
      )}

      {rows.length === 0 ? (
        <div className="rounded border border-[#1e293b] bg-[#0f172a]/50 p-8 text-center text-[#94a3b8]">
          还没有贡献。去{" "}
          <Link href="../galaxy" className="text-[#8b5cf6] hover:underline">
            Galaxy 列表
          </Link>{" "}
          挑一个主题开始吧。
        </div>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wider text-[#64748b]">
            <tr>
              <th className="p-2">Galaxy</th>
              <th className="p-2 text-right">Accepted</th>
              <th className="p-2 text-right">Pending</th>
              <th className="p-2 text-right">Disputed</th>
              <th className="p-2 text-right">Total</th>
              <th className="p-2 text-right">Avg conf.</th>
            </tr>
          </thead>
          <tbody className="text-[#e2e8f0]">
            {rows.map((r) => (
              <tr key={r.galaxy_id} className="border-t border-[#1e293b] hover:bg-[#0f172a]/50">
                <td className="p-2">
                  <Link
                    href={`../galaxy/${r.galaxy_slug}`}
                    className="text-[#8b5cf6] hover:underline"
                  >
                    {r.galaxy_title}
                  </Link>
                  <span className="ml-2 text-xs text-[#64748b]">{r.galaxy_slug}</span>
                </td>
                <td className="p-2 text-right text-emerald-400">{r.accepted}</td>
                <td className="p-2 text-right text-amber-400">{r.pending}</td>
                <td className="p-2 text-right text-rose-400">{r.disputed}</td>
                <td className="p-2 text-right">{r.total}</td>
                <td className="p-2 text-right text-[#94a3b8]">
                  {r.avg_confidence.toFixed(3)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function Stat({
  label,
  value,
  accent,
}: {
  label: string;
  value: number | string;
  accent: "cyan" | "emerald" | "amber" | "rose" | "violet";
}) {
  const color =
    accent === "cyan"
      ? "text-cyan-400"
      : accent === "emerald"
      ? "text-emerald-400"
      : accent === "amber"
      ? "text-amber-400"
      : accent === "rose"
      ? "text-rose-400"
      : "text-violet-400";
  return (
    <div className="rounded border border-[#1e293b] bg-[#0a0a14] p-3">
      <div className="text-xs uppercase tracking-wider text-[#64748b]">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${color}`}>{value}</div>
    </div>
  );
}

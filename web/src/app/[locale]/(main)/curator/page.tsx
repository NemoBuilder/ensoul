"use client";

// Curator 工作台 — 列出某个 Galaxy 下所有 disputed atoms，行内可
// accept/reject。需要管理员登录。
//
// 路径：/curator?galaxy=<slug>     按 slug 过滤
//      /curator                     展示所有 disputed
//
// 后端没有「全平台 disputed atoms」聚合接口，所以前端先抓一份 galaxy
// list，再串行/并行各取 status=disputed 的 atoms。规模不大时 ok；后期
// 高频出现冲突再加专用接口。

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  galaxyApi,
  curatorApi,
  type Galaxy,
  type Atom,
  ApiError,
} from "@/lib/api";

type Row = Atom & { galaxy_slug: string; galaxy_title: string };

export default function CuratorPage() {
  const sp = useSearchParams();
  const slug = sp.get("galaxy") || "";

  const [rows, setRows] = useState<Row[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [needAdmin, setNeedAdmin] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setErr(null);
    try {
      let galaxies: Galaxy[] = [];
      if (slug) {
        const g = await galaxyApi.get(slug);
        galaxies = [g];
      } else {
        const list = await galaxyApi.list();
        galaxies = list.galaxies || [];
      }

      const all: Row[] = [];
      for (const g of galaxies) {
        try {
          const r = await galaxyApi.atoms(g.slug, { status: "disputed" });
          for (const a of r.atoms || []) {
            all.push({ ...a, galaxy_slug: g.slug, galaxy_title: g.title });
          }
        } catch {
          // 单个 galaxy 失败不影响整体
        }
      }
      // 按 created_at desc 排序
      all.sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
      setRows(all);
    } catch (e) {
      if (e instanceof ApiError && (e.status === 401 || e.status === 403)) {
        setNeedAdmin(true);
      } else {
        setErr((e as Error).message);
      }
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleResolve = async (atomID: string, action: "accept" | "reject") => {
    if (!confirm(`确认 ${action.toUpperCase()} 这个 atom？`)) return;
    try {
      await curatorApi.resolve(atomID, action);
      // 本地立即移除
      setRows((prev) => prev.filter((r) => r.id !== atomID));
    } catch (e) {
      alert((e as Error).message);
    }
  };

  if (needAdmin) {
    return (
      <div className="mx-auto max-w-4xl px-4 pt-24 pb-16">
        <h1 className="text-2xl font-bold text-[#e2e8f0]">Curator 工作台</h1>
        <p className="mt-4 text-rose-400">
          需要管理员权限。请先在 /admin/login 登录。
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 pt-24 pb-16">
      <header className="mb-6 flex items-end justify-between">
        <div>
          <h1 className="text-3xl font-bold text-[#e2e8f0]">Curator 工作台</h1>
          <p className="mt-1 text-sm text-[#94a3b8]">
            处理被社区标记 (disputed) 的 atoms。
            {slug && (
              <>
                {" · "}过滤: <code className="text-cyan-400">{slug}</code>{" "}
                <Link href="/curator" className="underline text-[#94a3b8]">清除</Link>
              </>
            )}
          </p>
        </div>
        <button
          onClick={refresh}
          disabled={loading}
          className="rounded border border-[#1e293b] px-3 py-1.5 text-sm text-[#94a3b8] hover:text-[#8b5cf6]"
        >
          {loading ? "加载中…" : "↻ 刷新"}
        </button>
      </header>

      {err && (
        <div className="mb-4 rounded border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {err}
        </div>
      )}

      {!loading && rows.length === 0 ? (
        <div className="rounded border border-[#1e293b] bg-[#0f172a]/50 p-8 text-center text-[#94a3b8]">
          ✓ 没有待处理的 disputed atoms。
        </div>
      ) : (
        <ul className="space-y-3">
          {rows.map((a) => (
            <li
              key={a.id}
              className="rounded-lg border border-[#1e293b] bg-[#0a0a14] p-4 text-sm"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="mb-1 flex items-center gap-2">
                    <span className="rounded bg-rose-500/10 px-2 py-0.5 text-xs text-rose-400">
                      {a.kind}
                    </span>
                    <Link
                      href={`/galaxy/${a.galaxy_slug}`}
                      className="text-xs text-[#8b5cf6] hover:underline"
                    >
                      {a.galaxy_title}
                    </Link>
                    <span className="text-xs text-[#64748b]">
                      conf {a.confidence.toFixed(3)} · {new Date(a.created_at).toLocaleString()}
                    </span>
                  </div>
                  <div className="text-[#e2e8f0]">
                    {a.kind === "node"
                      ? a.node_label || "(no label)"
                      : `${a.head_node_id?.slice(0, 8) || "?"} ─ ${a.edge_label} → ${a.tail_node_id?.slice(0, 8) || "?"}`}
                  </div>
                  {a.kind === "node" && a.node_summary && (
                    <div className="mt-1 line-clamp-2 text-xs text-[#94a3b8]">
                      {a.node_summary}
                    </div>
                  )}
                </div>
                <div className="flex shrink-0 gap-2">
                  <button
                    onClick={() => handleResolve(a.id, "accept")}
                    className="rounded bg-emerald-500/10 px-3 py-1.5 text-xs text-emerald-400 hover:bg-emerald-500/20"
                  >
                    ✓ Accept
                  </button>
                  <button
                    onClick={() => handleResolve(a.id, "reject")}
                    className="rounded bg-rose-500/10 px-3 py-1.5 text-xs text-rose-400 hover:bg-rose-500/20"
                  >
                    ✗ Reject
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

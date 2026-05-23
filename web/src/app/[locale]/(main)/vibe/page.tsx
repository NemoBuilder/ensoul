"use client";

// Vibe Apps Hub — 静态门户页，把围绕 Galaxy 数据建立的几个内部应用 +
// MCP 接入指引集中在一个入口。后续第三方应用上架后扩展为动态目录。

import Link from "next/link";

const APPS: AppCard[] = [
  {
    title: "Vibe Explore",
    desc: "全屏 3D 星图浏览所有 Galaxy 的知识图谱。",
    href: "/galaxy",
    icon: "🌌",
    accent: "violet",
    status: "live",
  },
  {
    title: "Vibe Build",
    desc: "上传 markdown / PDF / web 链接，蒸馏成 atoms 入图。",
    href: "/galaxy",
    icon: "🔨",
    accent: "cyan",
    status: "live",
  },
  {
    title: "Vibe Curate",
    desc: "Curator 工作台，处理社区标记的 disputed atoms。",
    href: "/curator",
    icon: "⚖️",
    accent: "amber",
    status: "live",
  },
  {
    title: "Vibe Write",
    desc: "原 V3 写作工坊。Phase 5 计划接入 Galaxy 知识库做 RAG。",
    href: "/vibe-write",
    icon: "✍️",
    accent: "emerald",
    status: "live",
  },
  {
    title: "Epoch 探索器",
    desc: "查看每个 epoch 的 Merkle 根 + 浏览器本地校验 atom proof。",
    href: "/epoch",
    icon: "🔗",
    accent: "cyan",
    status: "live",
  },
  {
    title: "公平发射",
    desc: "成熟 Galaxy 的 72h 公平发射页 + 钱包 deposit/claim/refund。",
    href: "/galaxy",
    icon: "🚀",
    accent: "rose",
    status: "live",
  },
  {
    title: "MCP API",
    desc: "把 Galaxy 数据暴露给外部 LLM 应用。GET /api/mcp/galaxy/:slug",
    href: "#mcp",
    icon: "🔌",
    accent: "violet",
    status: "beta",
  },
  {
    title: "持有者分润",
    desc: "Phase 4：发币毕业 Galaxy 的现金流回流仪表盘。",
    href: "#",
    icon: "💰",
    accent: "amber",
    status: "soon",
  },
];

type Accent = "violet" | "cyan" | "amber" | "emerald" | "rose";
type Status = "live" | "beta" | "soon";

interface AppCard {
  title: string;
  desc: string;
  href: string;
  icon: string;
  accent: Accent;
  status: Status;
}

const accentClasses: Record<Accent, string> = {
  violet: "border-violet-500/30 hover:border-violet-500/60 hover:bg-violet-500/5",
  cyan: "border-cyan-500/30 hover:border-cyan-500/60 hover:bg-cyan-500/5",
  amber: "border-amber-500/30 hover:border-amber-500/60 hover:bg-amber-500/5",
  emerald: "border-emerald-500/30 hover:border-emerald-500/60 hover:bg-emerald-500/5",
  rose: "border-rose-500/30 hover:border-rose-500/60 hover:bg-rose-500/5",
};

const statusClasses: Record<Status, string> = {
  live: "bg-emerald-500/10 text-emerald-400",
  beta: "bg-amber-500/10 text-amber-400",
  soon: "bg-[#1e293b] text-[#64748b]",
};

export default function VibeHubPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 pt-24 pb-16">
      <header className="mb-10">
        <h1 className="text-4xl font-bold text-[#e2e8f0]">Vibe Hub</h1>
        <p className="mt-2 max-w-2xl text-[#94a3b8]">
          围绕 Ensoul Galaxy 知识图谱协议构建的 Vibe 应用门户。
          每个应用都把图谱当成一等公民数据源使用。
        </p>
      </header>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {APPS.map((app) => {
          const inner = (
            <div
              className={`group h-full rounded-xl border bg-[#0a0a14] p-5 transition-colors ${accentClasses[app.accent]}`}
            >
              <div className="mb-3 flex items-start justify-between">
                <div className="text-3xl">{app.icon}</div>
                <span
                  className={`rounded px-2 py-0.5 text-[10px] uppercase tracking-wider ${statusClasses[app.status]}`}
                >
                  {app.status}
                </span>
              </div>
              <h3 className="text-lg font-semibold text-[#e2e8f0] group-hover:text-white">
                {app.title}
              </h3>
              <p className="mt-1 text-sm text-[#94a3b8]">{app.desc}</p>
            </div>
          );
          return app.status === "soon" ? (
            <div key={app.title} className="cursor-not-allowed opacity-50">
              {inner}
            </div>
          ) : (
            <Link key={app.title} href={app.href}>
              {inner}
            </Link>
          );
        })}
      </div>

      {/* MCP usage block */}
      <section
        id="mcp"
        className="mt-16 rounded-xl border border-[#1e293b] bg-[#0a0a14] p-6"
      >
        <h2 className="text-2xl font-bold text-[#e2e8f0]">MCP 接入</h2>
        <p className="mt-2 text-sm text-[#94a3b8]">
          所有 Galaxy 数据通过稳定的只读 HTTP API 暴露，方便外部 MCP server / LLM 应用消费：
        </p>
        <div className="mt-4 space-y-3 font-mono text-xs text-[#cbd5e1]">
          <div className="rounded bg-[#0f172a] p-3">
            <span className="text-emerald-400">GET</span>{" "}
            <span className="text-cyan-400">/api/mcp/galaxy/list</span>
            <span className="ml-2 text-[#64748b]"># Top 200 galaxies</span>
          </div>
          <div className="rounded bg-[#0f172a] p-3">
            <span className="text-emerald-400">GET</span>{" "}
            <span className="text-cyan-400">/api/mcp/galaxy/:slug</span>
            <span className="ml-2 text-[#64748b]"># 完整快照（≤5000 atoms）</span>
          </div>
          <div className="rounded bg-[#0f172a] p-3">
            <span className="text-emerald-400">GET</span>{" "}
            <span className="text-cyan-400">
              /api/mcp/galaxy/:slug/nodes?limit=&amp;cursor=
            </span>
            <span className="ml-2 text-[#64748b]"># 分页节点</span>
          </div>
        </div>
        <p className="mt-4 text-xs text-[#64748b]">
          Schema:{" "}
          <code className="text-[#06b6d4]">ensoul.galaxy/v1</code>。无需鉴权。
        </p>
      </section>
    </div>
  );
}

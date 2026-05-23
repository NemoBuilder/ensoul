"use client";

// V4 Atom 卡片 + 仲裁弹窗。
// 复用于 Galaxy 详情页与（未来的）Atom 详情页。
// 点击 dispute 按钮 → 打开 Modal，输入理由 → POST /api/v4/atom/:id/dispute。

import { useState } from "react";
import { atomApi, type Atom } from "@/lib/api";

export default function AtomCard({
  atom,
  onChanged,
}: {
  atom: Atom;
  onChanged?: () => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <li className="flex items-start justify-between rounded-md border border-[#1e1e2e] bg-[#0a0a14] px-4 py-3">
        <div className="min-w-0 flex-1">
          {atom.kind === "node" ? (
            <>
              <div className="font-medium text-[#e2e8f0]">{atom.node_label}</div>
              <div className="text-xs text-[#64748b]">
                {atom.node_type} · conf {(atom.confidence * 100).toFixed(0)}%
                {atom.ambiguous && (
                  <span className="ml-2 text-amber-400">ambiguous</span>
                )}
                {atom.status === "disputed" && (
                  <span className="ml-2 rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-400">
                    disputed
                  </span>
                )}
              </div>
              {atom.node_summary && (
                <div className="mt-1 line-clamp-2 text-sm text-[#94a3b8]">
                  {atom.node_summary}
                </div>
              )}
            </>
          ) : (
            <>
              <div className="font-medium text-[#e2e8f0]">{atom.edge_label}</div>
              <div className="text-xs text-[#64748b]">
                {atom.edge_dir} · conf {(atom.confidence * 100).toFixed(0)}%
                {atom.status === "disputed" && (
                  <span className="ml-2 rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-400">
                    disputed
                  </span>
                )}
              </div>
            </>
          )}
        </div>
        <button
          onClick={() => setOpen(true)}
          disabled={atom.status !== "accepted"}
          className="ml-4 rounded border border-[#1e1e2e] px-2 py-1 text-xs text-[#94a3b8] hover:border-amber-500/50 hover:text-amber-400 disabled:opacity-40"
        >
          {atom.status === "disputed" ? "disputed" : "dispute"}
        </button>
      </li>

      {open && (
        <DisputeModal
          atom={atom}
          onClose={() => setOpen(false)}
          onDone={() => {
            setOpen(false);
            onChanged?.();
          }}
        />
      )}
    </>
  );
}

function DisputeModal({
  atom,
  onClose,
  onDone,
}: {
  atom: Atom;
  onClose: () => void;
  onDone: () => void;
}) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async () => {
    if (reason.trim().length < 4) {
      setErr("理由至少 4 个字符");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await atomApi.dispute(atom.id, reason.trim());
      onDone();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "提交失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-lg border border-[#1e1e2e] bg-[#0a0a14] p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="mb-1 text-lg font-semibold text-[#e2e8f0]">
          Dispute atom
        </h3>
        <p className="mb-4 text-xs text-[#64748b]">
          {atom.kind === "node" ? atom.node_label : atom.edge_label}
        </p>
        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
          placeholder="说明这个 atom 为什么有问题（事实错误/缺乏来源/重复/越界…）"
          className="w-full rounded-md border border-[#1e1e2e] bg-[#0a0a14] p-3 text-sm text-[#e2e8f0] focus:border-amber-500 focus:outline-none"
        />
        {err && <p className="mt-2 text-xs text-red-300">{err}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="rounded px-3 py-1.5 text-sm text-[#94a3b8] hover:text-[#e2e8f0]"
          >
            取消
          </button>
          <button
            onClick={submit}
            disabled={busy}
            className="rounded bg-amber-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-amber-500 disabled:opacity-50"
          >
            {busy ? "提交中…" : "提交 dispute"}
          </button>
        </div>
      </div>
    </div>
  );
}

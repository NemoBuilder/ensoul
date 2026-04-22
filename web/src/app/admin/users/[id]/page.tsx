"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter, useParams } from "next/navigation";
import {
  adminUserApi,
  adminGiftProApi,
  type AdminUserDetailResponse,
  type GiftProLog,
  type AuthType,
} from "@/lib/admin-api";

// ── Confirm Modal ──────────────────────────────────────────────

function ConfirmModal({
  title,
  children,
  confirmLabel,
  confirmColor,
  onConfirm,
  onCancel,
  loading,
}: {
  title: string;
  children: React.ReactNode;
  confirmLabel: string;
  confirmColor?: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading: boolean;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-md rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-6">
        <h3 className="mb-4 text-lg font-bold text-[#e2e8f0]">{title}</h3>
        <div className="mb-6">{children}</div>
        <div className="flex justify-end gap-3">
          <button
            onClick={onCancel}
            disabled={loading}
            className="rounded-lg border border-[#1e1e2e] px-4 py-2 text-sm text-[#94a3b8] hover:bg-[#1e1e2e]"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={loading}
            className={`rounded-lg px-4 py-2 text-sm font-medium text-white ${
              confirmColor || "bg-[#8b5cf6] hover:bg-[#7c3aed]"
            } disabled:opacity-50`}
          >
            {loading ? "Processing..." : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Helpers ────────────────────────────────────────────────────

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString();
}

function shortAddr(addr: string): string {
  if (addr.length <= 12) return addr;
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

function InfoRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2 py-1.5">
      <span className="w-28 shrink-0 text-xs text-[#64748b]">{label}</span>
      <span className="text-sm text-[#e2e8f0]">{children}</span>
    </div>
  );
}

function AuthTypeBadge({ type }: { type: AuthType }) {
  const map: Record<AuthType, { icon: string; label: string; cls: string }> = {
    email:   { icon: "✉️", label: "Email",   cls: "bg-blue-500/10 text-blue-400 border-blue-500/30" },
    wallet:  { icon: "🔁", label: "Wallet",  cls: "bg-amber-500/10 text-amber-400 border-amber-500/30" },
    linked:  { icon: "🔗", label: "Linked",  cls: "bg-emerald-500/10 text-emerald-400 border-emerald-500/30" },
    unknown: { icon: "❓", label: "Unknown", cls: "bg-[#1e1e2e] text-[#64748b] border-[#1e1e2e]" },
  };
  const { icon, label, cls } = map[type] ?? map.unknown;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${cls}`}>
      <span>{icon}</span> {label}
    </span>
  );
}

// ── Main Page ──────────────────────────────────────────────────

export default function AdminUserDetailPage() {
  const router = useRouter();
  const params = useParams();
  const userID = params.id as string;

  const [data, setData] = useState<AdminUserDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [modal, setModal] = useState<
    | null
    | "ban"
    | "unban"
    | "grant"
    | "extend"
    | "revoke"
    | "note"
    | "giftPro"
  >(null);
  const [modalLoading, setModalLoading] = useState(false);
  const [banReason, setBanReason] = useState("");
  const [revokeReason, setRevokeReason] = useState("");
  const [grantDays, setGrantDays] = useState(30);
  const [grantReason, setGrantReason] = useState("");
  const [extendDays, setExtendDays] = useState(30);
  const [extendReason, setExtendReason] = useState("");
  const [noteText, setNoteText] = useState("");
  const [giftMonths, setGiftMonths] = useState(1);
  const [giftReason, setGiftReason] = useState("");
  const [giftLogs, setGiftLogs] = useState<GiftProLog[]>([]);

  const fetchDetail = useCallback(async () => {
    try {
      setLoading(true);
      setError("");
      const res = await adminUserApi.detail(userID);
      setData(res);
      setNoteText(res.user.note || "");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load user");
    } finally {
      setLoading(false);
    }
  }, [userID]);

  const fetchGiftLogs = useCallback(async () => {
    try {
      const res = await adminGiftProApi.listLogs({ user: userID, page_size: 20 });
      setGiftLogs(res.items);
    } catch {
      setGiftLogs([]);
    }
  }, [userID]);

  useEffect(() => {
    fetchDetail();
    fetchGiftLogs();
  }, [fetchDetail, fetchGiftLogs]);

  const handleAction = async (action: () => Promise<unknown>) => {
    try {
      setModalLoading(true);
      await action();
      setModal(null);
      fetchDetail();
      fetchGiftLogs();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Action failed");
    } finally {
      setModalLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <p className="text-[#64748b]">Loading user detail...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="flex min-h-[40vh] flex-col items-center justify-center gap-4">
        <p className="text-red-400">{error || "User not found"}</p>
        <button
          onClick={() => router.push("/admin/users")}
          className="rounded-lg border border-[#1e1e2e] px-4 py-2 text-sm text-[#94a3b8] hover:bg-[#1e1e2e]"
        >
          ← Back to Users
        </button>
      </div>
    );
  }

  const { user, auth_type, subscription, subscription_history, persona, stats } = data;
  const selected_tags = data.selected_tags ?? [];
  const muted_accounts = data.muted_accounts ?? [];
  const isBanned = user.status === "banned";
  const hasSub = !!subscription;
  const hasEmail = !!user.email;
  const hasWallet = !!user.wallet_addr;

  return (
    <div className="space-y-6">
      {/* ── Back button ────────────────────────────────── */}
      <button
        onClick={() => router.push("/admin/users")}
        className="flex items-center gap-1 text-sm text-[#94a3b8] hover:text-[#e2e8f0]"
      >
        ← Back to Users
      </button>

      {/* ── User Info + Stats ──────────────────────────── */}
      <div className="grid gap-4 lg:grid-cols-2">
        {/* User Info Card */}
        <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-5">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-[#e2e8f0]">
              <span>👤</span> User Info
            </h2>
            <AuthTypeBadge type={auth_type} />
          </div>

          {/* Identity — email primary */}
          {hasEmail ? (
            <InfoRow label="Email">
              <span className="flex items-center gap-2">
                <span className="text-[#e2e8f0]">{user.email}</span>
                {user.email_verified && (
                  <span className="rounded-full bg-emerald-500/10 px-1.5 py-0.5 text-[10px] text-emerald-400 border border-emerald-500/30">
                    ✓ Verified
                  </span>
                )}
              </span>
            </InfoRow>
          ) : (
            <InfoRow label="Email">
              <span className="text-xs text-[#475569]">— (wallet-only account)</span>
            </InfoRow>
          )}

          {hasWallet ? (
            <InfoRow label="Wallet">
              <code className="rounded bg-[#1e1e2e] px-1.5 py-0.5 text-xs text-[#8b5cf6]">
                {user.wallet_addr}
              </code>
            </InfoRow>
          ) : (
            <InfoRow label="Wallet">
              <span className="text-xs text-[#475569]">— (not bound)</span>
            </InfoRow>
          )}

          <InfoRow label="User ID">
            <code className="text-[10px] text-[#64748b]">{user.id}</code>
          </InfoRow>

          <InfoRow label="Status">
            {isBanned ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-red-500/10 px-2 py-0.5 text-xs text-red-400 border border-red-500/30">
                🔴 Banned
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-400 border border-emerald-500/30">
                🟢 Active
              </span>
            )}
          </InfoRow>
          {isBanned && user.ban_reason && (
            <InfoRow label="Ban Reason">
              <span className="text-red-400">{user.ban_reason}</span>
            </InfoRow>
          )}
          <InfoRow label="First Seen">{formatDate(user.first_seen_at)}</InfoRow>
          <InfoRow label="Last Seen">{timeAgo(user.last_seen_at)}</InfoRow>
          <InfoRow label="Login Count">{user.login_count}</InfoRow>
          <InfoRow label="Note">
            <span className="text-[#94a3b8]">{user.note || "—"}</span>
          </InfoRow>

          {/* Action buttons */}
          <div className="mt-4 flex flex-wrap gap-2">
            <button
              onClick={() => { setNoteText(user.note || ""); setModal("note"); }}
              className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e]"
            >
              ✏️ Edit Note
            </button>
            {isBanned ? (
              <button
                onClick={() => setModal("unban")}
                className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-700"
              >
                ✅ Unban User
              </button>
            ) : (
              <button
                onClick={() => { setBanReason(""); setModal("ban"); }}
                className="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700"
              >
                🚫 Ban User
              </button>
            )}
          </div>
        </div>

        {/* Stats Card */}
        <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-5">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-[#e2e8f0]">
              <span>📊</span> Stats
            </h2>
            {!hasWallet && (
              <span className="text-[10px] text-[#475569]">
                Wallet-bound stats unavailable
              </span>
            )}
          </div>
          {hasWallet ? (
            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Total Snipes</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.total_snipes}</p>
              </div>
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Today Snipes</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.today_snipes}</p>
              </div>
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Total Chats</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.total_chats}</p>
              </div>
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Shells Owned</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.shells_owned}</p>
              </div>
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Claws Bound</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.claws_bound}</p>
              </div>
              <div className="rounded-lg bg-[#1e1e2e]/50 p-3">
                <p className="text-xs text-[#64748b]">Withdrawals</p>
                <p className="mt-1 text-xl font-bold text-[#e2e8f0]">{stats.total_withdrawals.toFixed(2)}</p>
              </div>
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-[#1e1e2e] bg-[#0a0a0f] p-6 text-center">
              <p className="text-xs text-[#64748b]">
                This user has no wallet bound. On-chain stats (snipes, shells, claws, withdrawals)
                are not applicable.
              </p>
              <p className="mt-2 text-sm text-[#94a3b8]">
                Vibe Write Chats: <span className="font-bold text-[#e2e8f0]">{stats.total_chats}</span>
              </p>
            </div>
          )}
        </div>
      </div>

      {/* ── Subscription ───────────────────────────────── */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-5">
        <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-[#e2e8f0]">
          <span>💳</span> Subscription
        </h2>

        {hasSub ? (
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-3">
              <span className="inline-flex items-center gap-1 rounded-full bg-purple-500/10 px-2 py-0.5 text-xs font-medium text-purple-400 border border-purple-500/30">
                ⭐ {subscription.tier.toUpperCase()}
              </span>
              <span className="text-xs text-[#94a3b8]">
                Expires: {formatDate(subscription.expires_at)}
              </span>
              <span className="text-xs text-[#64748b]">
                Model: {subscription.llm_model}
              </span>
            </div>
            <div className="text-xs text-[#64748b]">
              Payment: {subscription.payment_amount} {subscription.payment_token}
              {subscription.payment_tx_hash && subscription.payment_tx_hash !== "admin_grant" && (
                <span className="ml-1">
                  (tx: <code className="text-[#8b5cf6]">{subscription.payment_tx_hash.slice(0, 10)}...</code>)
                </span>
              )}
              {subscription.payment_tx_hash === "admin_grant" && (
                <span className="ml-1 text-yellow-400">(Admin Grant)</span>
              )}
            </div>
            <div className="flex flex-wrap gap-2 pt-1">
              <button
                onClick={() => { setExtendDays(30); setExtendReason(""); setModal("extend"); }}
                className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e]"
              >
                +30 Days
              </button>
              <button
                onClick={() => { setExtendDays(15); setExtendReason(""); setModal("extend"); }}
                className="rounded-lg border border-[#1e1e2e] px-3 py-1.5 text-xs text-[#94a3b8] hover:bg-[#1e1e2e]"
              >
                +15 Days
              </button>
              <button
                onClick={() => { setRevokeReason(""); setModal("revoke"); }}
                className="rounded-lg bg-red-600/80 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700"
              >
                Revoke
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-[#64748b]">No active subscription (Free tier)</p>
            <button
              onClick={() => { setGrantDays(30); setGrantReason(""); setModal("grant"); }}
              className="rounded-lg bg-[#8b5cf6] px-3 py-1.5 text-xs font-medium text-white hover:bg-[#7c3aed]"
            >
              Grant Pro Subscription
            </button>
          </div>
        )}

        {subscription_history && subscription_history.length > 0 && (
          <div className="mt-4 border-t border-[#1e1e2e] pt-3">
            <h3 className="mb-2 text-xs font-medium text-[#64748b]">History</h3>
            <div className="space-y-1.5">
              {subscription_history.map((h) => (
                <div key={h.id} className="flex items-center gap-3 text-xs text-[#94a3b8]">
                  <span className={`rounded-full px-1.5 py-0.5 text-[10px] ${
                    h.status === "active"
                      ? "bg-emerald-500/10 text-emerald-400"
                      : h.status === "cancelled"
                      ? "bg-red-500/10 text-red-400"
                      : "bg-yellow-500/10 text-yellow-400"
                  }`}>
                    {h.status}
                  </span>
                  <span>{h.tier}</span>
                  <span>→ {new Date(h.expires_at).toLocaleDateString()}</span>
                  <span className="text-[#475569]">
                    {h.payment_amount} {h.payment_token}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* ── Gift Pro (email-only) ───────────────────────── */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-5">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-[#e2e8f0]">
            <span>🎁</span> Gift Pro
          </h2>
          <button
            onClick={() => { setGiftMonths(1); setGiftReason(""); setModal("giftPro"); }}
            disabled={!hasEmail}
            title={hasEmail ? "Gift Pro months to this user" : "Only email accounts can receive Pro gifts"}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors ${
              hasEmail
                ? "bg-[#8b5cf6] hover:bg-[#7c3aed]"
                : "bg-[#1e1e2e] text-[#475569] cursor-not-allowed"
            }`}
          >
            🎁 Gift Pro
          </button>
        </div>
        {!hasEmail && (
          <p className="mb-3 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-400">
            ⚠ Pro gifts can only be sent to accounts with an email. This wallet-only user cannot receive gifts.
          </p>
        )}
        <p className="mb-3 text-xs text-[#64748b]">
          Pro expires at: <span className="text-[#94a3b8]">
            {user.pro_expires_at ? new Date(user.pro_expires_at).toLocaleString() : "—"}
          </span>
        </p>
        {giftLogs.length === 0 ? (
          <p className="text-xs text-[#475569]">No gift records yet.</p>
        ) : (
          <div className="space-y-1.5">
            {giftLogs.map((g) => (
              <div key={g.id} className="flex flex-wrap items-center gap-2 text-xs text-[#94a3b8]">
                <span className="rounded-full bg-purple-500/10 px-1.5 py-0.5 text-[10px] text-purple-400">
                  +{g.months}mo
                </span>
                <span className="text-[#64748b]">by {g.admin_name}</span>
                <span className="text-[#475569]">
                  → {new Date(g.new_expires_at).toLocaleDateString()}
                </span>
                {g.reason && <span className="text-[#64748b] italic">— {g.reason}</span>}
                <span className="ml-auto text-[10px] text-[#475569]">
                  {new Date(g.created_at).toLocaleDateString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* ── Vibe Write Settings ────────────────────────────── */}
      <div className="rounded-xl border border-[#1e1e2e] bg-[#0d0d14] p-5">
        <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-[#e2e8f0]">
          <span>🎯</span> Vibe Write Settings
        </h2>
        {persona ? (
          <div className="space-y-2 text-xs text-[#94a3b8]">
            {persona.bio && <InfoRow label="Bio">{persona.bio}</InfoRow>}
            {persona.style && <InfoRow label="Style">{persona.style}</InfoRow>}
            <InfoRow label="Language">{persona.language || "en"}</InfoRow>
          </div>
        ) : (
          <p className="text-xs text-[#64748b]">No persona configured</p>
        )}
        <div className="mt-3">
          <p className="text-xs text-[#64748b]">
            Tags: {selected_tags.length > 0 ? selected_tags.map((t) => (
              <span key={t} className="mr-1 inline-block rounded bg-[#1e1e2e] px-1.5 py-0.5 text-[#8b5cf6]">{t}</span>
            )) : "None selected"}
          </p>
        </div>
        {muted_accounts.length > 0 && (
          <div className="mt-2">
            <p className="text-xs text-[#64748b]">
              Muted: {muted_accounts.map((h) => (
                <span key={h} className="mr-1 inline-block rounded bg-[#1e1e2e] px-1.5 py-0.5 text-red-400">@{h}</span>
              ))}
            </p>
          </div>
        )}
      </div>

      {/* ═══ Modals ═══════════════════════════════════════ */}

      {modal === "ban" && (
        <ConfirmModal
          title="🚫 Ban User"
          confirmLabel="Ban User"
          confirmColor="bg-red-600 hover:bg-red-700"
          onConfirm={() => handleAction(() => adminUserApi.ban(userID, banReason))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <p className="mb-3 text-sm text-[#94a3b8]">
            This will ban <strong>{user.email || (hasWallet ? shortAddr(user.wallet_addr) : user.id)}</strong>,
            force logout, and cancel any active subscription.
          </p>
          <textarea
            placeholder="Reason for ban (optional)"
            value={banReason}
            onChange={(e) => setBanReason(e.target.value)}
            className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
            rows={2}
          />
        </ConfirmModal>
      )}

      {modal === "unban" && (
        <ConfirmModal
          title="✅ Unban User"
          confirmLabel="Unban User"
          confirmColor="bg-emerald-600 hover:bg-emerald-700"
          onConfirm={() => handleAction(() => adminUserApi.unban(userID))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <p className="text-sm text-[#94a3b8]">
            This will restore access for <strong>{user.email || (hasWallet ? shortAddr(user.wallet_addr) : user.id)}</strong>.
            Note: subscription will NOT be automatically restored.
          </p>
        </ConfirmModal>
      )}

      {modal === "note" && (
        <ConfirmModal
          title="✏️ Edit Note"
          confirmLabel="Save Note"
          onConfirm={() => handleAction(() => adminUserApi.updateNote(userID, noteText))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <textarea
            placeholder="Admin note about this user..."
            value={noteText}
            onChange={(e) => setNoteText(e.target.value)}
            className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
            rows={3}
          />
        </ConfirmModal>
      )}

      {modal === "grant" && (
        <ConfirmModal
          title="⭐ Grant Pro Subscription"
          confirmLabel="Grant"
          confirmColor="bg-purple-600 hover:bg-purple-700"
          onConfirm={() => handleAction(() => adminUserApi.grantSubscription(userID, "pro", grantDays, grantReason))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Duration (days)</label>
              <input
                type="number"
                min={1}
                max={365}
                value={grantDays}
                onChange={(e) => setGrantDays(Number(e.target.value))}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Reason</label>
              <input
                type="text"
                placeholder="e.g. Promotional reward"
                value={grantReason}
                onChange={(e) => setGrantReason(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
          </div>
        </ConfirmModal>
      )}

      {modal === "extend" && (
        <ConfirmModal
          title="📅 Extend Subscription"
          confirmLabel={`Extend +${extendDays} Days`}
          onConfirm={() => handleAction(() => adminUserApi.extendSubscription(userID, extendDays, extendReason))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <div className="space-y-3">
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Days to add</label>
              <input
                type="number"
                min={1}
                max={365}
                value={extendDays}
                onChange={(e) => setExtendDays(Number(e.target.value))}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Reason</label>
              <input
                type="text"
                placeholder="e.g. Service disruption compensation"
                value={extendReason}
                onChange={(e) => setExtendReason(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
          </div>
        </ConfirmModal>
      )}

      {modal === "revoke" && (
        <ConfirmModal
          title="⚠️ Revoke Subscription"
          confirmLabel="Revoke"
          confirmColor="bg-red-600 hover:bg-red-700"
          onConfirm={() => handleAction(() => adminUserApi.revokeSubscription(userID, revokeReason))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <div className="space-y-3">
            <p className="text-sm text-[#94a3b8]">
              This will immediately cancel the active subscription. No refund will be issued.
            </p>
            <textarea
              placeholder="Reason for revocation"
              value={revokeReason}
              onChange={(e) => setRevokeReason(e.target.value)}
              className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              rows={2}
            />
          </div>
        </ConfirmModal>
      )}

      {modal === "giftPro" && (
        <ConfirmModal
          title="🎁 Gift Pro"
          confirmLabel={`Gift +${giftMonths}mo`}
          confirmColor="bg-purple-600 hover:bg-purple-700"
          onConfirm={() => handleAction(() => adminGiftProApi.gift(userID, giftMonths, giftReason))}
          onCancel={() => setModal(null)}
          loading={modalLoading}
        >
          <div className="space-y-3">
            <p className="text-xs text-[#64748b]">
              Adds months on top of current expiry (or starts from now if expired). Idempotent — every call extends.
            </p>
            <p className="rounded-lg bg-blue-500/5 border border-blue-500/20 px-3 py-2 text-xs text-blue-300">
              Recipient: <span className="font-medium">{user.email}</span>
            </p>
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Months</label>
              <div className="flex gap-2">
                {[1, 3, 6, 12].map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setGiftMonths(m)}
                    className={`rounded-lg border px-3 py-1.5 text-xs ${
                      giftMonths === m
                        ? "border-[#8b5cf6] bg-[#8b5cf6]/20 text-[#a78bfa]"
                        : "border-[#1e1e2e] text-[#94a3b8] hover:bg-[#1e1e2e]"
                    }`}
                  >
                    {m}mo
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="mb-1 block text-xs text-[#64748b]">Reason</label>
              <input
                type="text"
                placeholder="e.g. KOL partnership / support compensation"
                value={giftReason}
                onChange={(e) => setGiftReason(e.target.value)}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none"
              />
            </div>
          </div>
        </ConfirmModal>
      )}
    </div>
  );
}

"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useAccount, useSignMessage } from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { Link } from "@/i18n/navigation";
import {
  bindApi,
  billingApi,
  cryptoBillingApi,
  emailAuthApi,
  type BillingStatus,
  type CryptoPayment,
  type EmailSessionInfo,
} from "@/lib/api";
import PaymentMethodModal from "@/components/PaymentMethodModal";

export default function SettingsPage() {
  const t = useTranslations("Settings");
  const tSub = useTranslations("Subscription");
  const [user, setUser] = useState<EmailSessionInfo | null>(null);
  const [loading, setLoading] = useState(true);

  // Subscription state
  const [billing, setBilling] = useState<BillingStatus | null>(null);
  const [payments, setPayments] = useState<CryptoPayment[] | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [methodOpen, setMethodOpen] = useState(false);

  // Bind-wallet state (when user is email-only)
  const { address: connectedAddr, isConnected } = useAccount();
  const { signMessageAsync } = useSignMessage();
  const [bindingWallet, setBindingWallet] = useState(false);
  const [walletMsg, setWalletMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  // Bind-email state (when user is wallet-only — i.e. no email session, just a wallet user)
  // Note: The current /api/auth/email/session endpoint only returns user info for *email* sessions.
  // For wallet-only users, we'd need a unified /api/auth/me — for now, this page focuses on
  // the email-logged-in flow (the dominant path) and offers wallet binding from there.

  const [bindEmail, setBindEmail] = useState("");
  const [bindCode, setBindCode] = useState("");
  const [codeSent, setCodeSent] = useState(false);
  const [bindingEmail, setBindingEmail] = useState(false);
  const [emailMsg, setEmailMsg] = useState<{ kind: "ok" | "err"; text: string } | null>(null);

  useEffect(() => {
    emailAuthApi
      .session()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!user) return;
    billingApi.status().then(setBilling).catch(() => setBilling(null));
  }, [user]);

  useEffect(() => {
    if (!historyOpen || payments !== null) return;
    cryptoBillingApi
      .history()
      .then((r) => setPayments(r.items))
      .catch(() => setPayments([]));
  }, [historyOpen, payments]);

  function formatExpires(): string | null {
    if (!billing?.pro_expires_at) return null;
    try {
      return new Date(billing.pro_expires_at).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
    } catch {
      return null;
    }
  }

  function daysRemaining(): number | null {
    if (!billing?.pro_expires_at) return null;
    try {
      const diff = new Date(billing.pro_expires_at).getTime() - Date.now();
      if (diff <= 0) return 0;
      return Math.ceil(diff / (1000 * 60 * 60 * 24));
    } catch {
      return null;
    }
  }

  async function handleBindWallet() {
    if (!isConnected || !connectedAddr) {
      setWalletMsg({ kind: "err", text: t("walletNotConnected") });
      return;
    }
    setBindingWallet(true);
    setWalletMsg(null);
    try {
      const message = `ensoul:bind:${Date.now()}`;
      const signature = await signMessageAsync({ message });
      const r = await bindApi.wallet(connectedAddr, signature, message);
      setWalletMsg({ kind: "ok", text: t("bindingSuccess") });
      setUser((u) => (u ? { ...u, wallet_addr: r.wallet_addr } : u));
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setWalletMsg({ kind: "err", text: `${t("bindingFailed")}: ${msg}` });
    } finally {
      setBindingWallet(false);
    }
  }

  async function handleSendCode() {
    setEmailMsg(null);
    try {
      await bindApi.emailSend(bindEmail.trim());
      setCodeSent(true);
      setEmailMsg({ kind: "ok", text: t("sendCode") + " ✓" });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setEmailMsg({ kind: "err", text: msg });
    }
  }

  async function handleBindEmail() {
    setBindingEmail(true);
    setEmailMsg(null);
    try {
      const r = await bindApi.email(bindEmail.trim(), bindCode.trim());
      setEmailMsg({ kind: "ok", text: t("bindingSuccess") });
      setUser((u) => (u ? { ...u, email: r.email } : u));
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setEmailMsg({ kind: "err", text: `${t("bindingFailed")}: ${msg}` });
    } finally {
      setBindingEmail(false);
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-[#0a0a0f] pt-24 pb-16">
        <div className="mx-auto max-w-2xl px-4 text-center text-[#94a3b8]">…</div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="min-h-screen bg-[#0a0a0f] pt-24 pb-16">
        <div className="mx-auto max-w-2xl px-4 text-center text-[#94a3b8]">
          {t("loginRequired")}
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0a0a0f] pt-24 pb-16">
      <div className="mx-auto max-w-2xl px-4 sm:px-6 lg:px-8">
        <h1 className="mb-8 text-3xl font-bold text-[#e2e8f0]">{t("title")}</h1>

        {/* Account overview */}
        <section className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <h2 className="mb-4 text-lg font-semibold text-[#e2e8f0]">{t("accountTab")}</h2>

          <Row label={t("boundEmail")} value={user.email || t("notBound")} />
          <Row
            label={t("boundWallet")}
            value={user.wallet_addr ? shorten(user.wallet_addr) : t("notBound")}
          />
          <Row label={t("twitterHandle")} value={user.twitter_handle ? "@" + user.twitter_handle : t("notSet")} />
          <Row label={t("proStatus")} value={user.is_pro ? t("proActive") : t("proInactive")} />
          <Row label={t("creditsRemaining")} value={String(user.credits)} />
        </section>

        {/* Subscription */}
        <section className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-[#e2e8f0]">{tSub("title")}</h2>
            {billing?.is_pro && (
              <span className="rounded bg-[#8b5cf6] px-2 py-0.5 text-[10px] font-bold text-white">
                PRO
              </span>
            )}
          </div>

          {billing?.is_pro ? (
            <>
              <Row label={tSub("plan")} value={tSub("planPro")} />
              <Row label={tSub("expiresAt")} value={formatExpires() ?? "—"} />
              <Row
                label={tSub("daysRemainingLabel")}
                value={daysRemaining() !== null ? tSub("daysValue", { n: daysRemaining()! }) : "—"}
              />
              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  onClick={() => setMethodOpen(true)}
                  className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
                >
                  {tSub("renewNow")}
                </button>
                <Link
                  href="/pricing"
                  className="rounded-lg border border-[#1e1e2e] px-4 py-2 text-sm text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
                >
                  {tSub("viewPlan")}
                </Link>
              </div>
            </>
          ) : (
            <>
              <Row label={tSub("plan")} value={tSub("planFree")} />
              <p className="mt-3 text-sm text-[#94a3b8]">{tSub("freeHint")}</p>
              <div className="mt-4">
                <button
                  onClick={() => setMethodOpen(true)}
                  className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
                >
                  {tSub("upgradeNow")}
                </button>
              </div>
            </>
          )}

          {/* Payment history toggle */}
          <div className="mt-5 border-t border-[#1e1e2e] pt-4">
            <button
              onClick={() => setHistoryOpen((v) => !v)}
              className="flex w-full items-center justify-between text-sm text-[#94a3b8] hover:text-[#e2e8f0]"
            >
              <span>{tSub("history")}</span>
              <span className={`transition-transform ${historyOpen ? "rotate-180" : ""}`}>▾</span>
            </button>

            {historyOpen && (
              <div className="mt-3 space-y-2">
                {payments === null && (
                  <p className="text-xs text-[#64748b]">{tSub("loading")}</p>
                )}
                {payments && payments.length === 0 && (
                  <p className="text-xs text-[#64748b]">{tSub("historyEmpty")}</p>
                )}
                {payments &&
                  payments.map((p) => (
                    <div
                      key={p.id}
                      className="rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] p-3 text-xs"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono text-[#94a3b8]">
                          {new Date(p.created_at).toLocaleString()}
                        </span>
                        <span
                          className={
                            p.status === "confirmed"
                              ? "text-emerald-400"
                              : p.status === "rejected"
                              ? "text-red-400"
                              : "text-yellow-300"
                          }
                        >
                          {tSub(`status_${p.status}`)}
                        </span>
                      </div>
                      <div className="mt-1 flex items-center justify-between text-[#94a3b8]">
                        <span>
                          {p.token} · {tSub("via", { method: tSub("methodCrypto") })}
                        </span>
                        <a
                          href={`https://bscscan.com/tx/${p.tx_hash}`}
                          target="_blank"
                          rel="noreferrer"
                          className="text-[#a78bfa] hover:underline"
                        >
                          {p.tx_hash.slice(0, 8)}…{p.tx_hash.slice(-6)}
                        </a>
                      </div>
                    </div>
                  ))}
              </div>
            )}
          </div>
        </section>

        {/* Bind wallet */}
        {!user.wallet_addr && (
          <section className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
            <h2 className="mb-2 text-lg font-semibold text-[#e2e8f0]">{t("bindWallet")}</h2>
            <p className="mb-4 text-sm text-[#94a3b8]">{t("bindWalletDesc")}</p>

            <div className="mb-3">
              <ConnectButton showBalance={false} chainStatus="icon" accountStatus="address" />
            </div>

            <button
              onClick={handleBindWallet}
              disabled={!isConnected || bindingWallet}
              className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {bindingWallet ? "…" : t("sign")}
            </button>

            {walletMsg && (
              <p className={`mt-3 text-sm ${walletMsg.kind === "ok" ? "text-emerald-400" : "text-red-400"}`}>
                {walletMsg.text}
              </p>
            )}
          </section>
        )}

        {/* Bind email — shown only if email already set; informational
            (the typical email-session user already has one).
            For wallet-only users we'd need a separate /me endpoint. */}
        {!user.email && (
          <section className="mb-8 rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6">
            <h2 className="mb-2 text-lg font-semibold text-[#e2e8f0]">{t("bindEmail")}</h2>
            <p className="mb-4 text-sm text-[#94a3b8]">{t("bindEmailDesc")}</p>

            <input
              type="email"
              value={bindEmail}
              onChange={(e) => setBindEmail(e.target.value)}
              placeholder="you@example.com"
              className="mb-2 w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
            />

            <div className="mb-3 flex gap-2">
              <button
                onClick={handleSendCode}
                disabled={!bindEmail}
                className="rounded-lg border border-[#1e1e2e] px-3 py-2 text-sm text-[#e2e8f0] hover:border-[#8b5cf6] disabled:opacity-50"
              >
                {t("sendCode")}
              </button>
              {codeSent && (
                <input
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  value={bindCode}
                  onChange={(e) => setBindCode(e.target.value)}
                  placeholder={t("code")}
                  className="flex-1 rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
                />
              )}
            </div>

            <button
              onClick={handleBindEmail}
              disabled={!bindEmail || bindCode.length !== 6 || bindingEmail}
              className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {bindingEmail ? "…" : t("bind")}
            </button>

            {emailMsg && (
              <p className={`mt-3 text-sm ${emailMsg.kind === "ok" ? "text-emerald-400" : "text-red-400"}`}>
                {emailMsg.text}
              </p>
            )}
          </section>
        )}
      </div>
      <PaymentMethodModal
        open={methodOpen}
        onClose={() => {
          setMethodOpen(false);
          // Refresh status after the modal closes (LemonSqueezy webhook may have run).
          billingApi.status().then(setBilling).catch(() => {});
        }}
      />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between border-b border-[#1e1e2e] py-3 last:border-b-0">
      <span className="text-sm text-[#94a3b8]">{label}</span>
      <span className="font-mono text-sm text-[#e2e8f0]">{value}</span>
    </div>
  );
}

function shorten(addr: string): string {
  return addr.length > 10 ? `${addr.slice(0, 6)}…${addr.slice(-4)}` : addr;
}

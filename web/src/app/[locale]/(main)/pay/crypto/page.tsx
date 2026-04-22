"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/navigation";
import { useSearchParams } from "next/navigation";
import {
  useAccount,
  useBalance,
  useChainId,
  useReadContract,
  useSendTransaction,
  useSwitchChain,
  useWriteContract,
} from "wagmi";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import { bsc } from "wagmi/chains";
import { parseAbi } from "viem";
import { cryptoBillingApi, type CryptoQuote, type CryptoPayment } from "@/lib/api";

const ERC20_ABI = parseAbi([
  "function balanceOf(address) view returns (uint256)",
  "function transfer(address to, uint256 amount) returns (bool)",
]);

type TokenChoice = "USDT" | "BNB";

function fmt(weiStr: string | undefined, decimals: number, display: number): string {
  if (!weiStr) return "—";
  try {
    const wei = BigInt(weiStr);
    const div = BigInt(10) ** BigInt(decimals);
    const intPart = wei / div;
    const frac = wei % div;
    const fracStr = frac.toString().padStart(decimals, "0").slice(0, display);
    return display > 0 ? `${intPart.toString()}.${fracStr}` : intPart.toString();
  } catch {
    return weiStr;
  }
}

export default function CryptoPayPage() {
  return (
    <Suspense fallback={<main className="min-h-screen bg-[#0a0a0f] pt-24 pb-20 text-[#e2e8f0]" />}>
      <CryptoPayPageInner />
    </Suspense>
  );
}

function CryptoPayPageInner() {
  const t = useTranslations("CryptoPay");
  const router = useRouter();
  const searchParams = useSearchParams();
  const { address, isConnected } = useAccount();
  const chainId = useChainId();
  const { switchChain } = useSwitchChain();
  const { writeContractAsync } = useWriteContract();
  const { sendTransactionAsync } = useSendTransaction();
  const isCorrectChain = chainId === bsc.id;

  const MONTH_OPTIONS = [1, 3, 6, 12] as const;
  const initialMonths = (() => {
    const raw = parseInt(searchParams.get("months") || "1", 10);
    if (!Number.isFinite(raw) || raw < 1) return 1;
    if (raw > 24) return 24;
    return raw;
  })();
  const [months, setMonths] = useState<number>(initialMonths);

  const [quote, setQuote] = useState<CryptoQuote | null>(null);
  const [quoteErr, setQuoteErr] = useState("");
  const [manualToken, setManualToken] = useState<TokenChoice | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [payment, setPayment] = useState<CryptoPayment | null>(null);

  // Fetch quote on mount + when months change + every 30s.
  useEffect(() => {
    let alive = true;
    setQuoteErr("");
    const load = () => {
      cryptoBillingApi
        .quote(months)
        .then((q) => alive && setQuote(q))
        .catch((e) => alive && setQuoteErr(e?.message || "quote failed"));
    };
    load();
    const t = setInterval(load, 30_000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, [months]);

  const usdtAddr = (quote?.usdt_contract || "0x55d398326f99059fF775485246999027B3197955") as `0x${string}`;
  const recipient = (quote?.recipient_addr || "0x0000000000000000000000000000000000000000") as `0x${string}`;

  // Balances.
  const { data: bnbBal } = useBalance({
    address,
    chainId: bsc.id,
    query: { enabled: !!address && isCorrectChain },
  });
  const { data: usdtBalRaw } = useReadContract({
    abi: ERC20_ABI,
    address: usdtAddr,
    functionName: "balanceOf",
    args: address ? [address] : undefined,
    chainId: bsc.id,
    query: { enabled: !!address && isCorrectChain && !!quote },
  });

  const usdtBalWei = (usdtBalRaw as bigint | undefined) ?? BigInt(0);
  const bnbBalWei = bnbBal?.value ?? BigInt(0);

  // Auto-select: prefer USDT if enough, else BNB if enough, else USDT (will show insufficient).
  const autoToken: TokenChoice = useMemo(() => {
    if (!quote) return "USDT";
    const needUSDT = BigInt(quote.usdt_wei || "0");
    const needBNB = quote.bnb_wei ? BigInt(quote.bnb_wei) : BigInt(0);
    if (usdtBalWei >= needUSDT) return "USDT";
    if (needBNB > BigInt(0) && bnbBalWei >= needBNB) return "BNB";
    return "USDT";
  }, [quote, usdtBalWei, bnbBalWei]);

  const token: TokenChoice = manualToken ?? autoToken;
  const needWei = token === "USDT" ? BigInt(quote?.usdt_wei || "0") : BigInt(quote?.bnb_wei || "0");
  const balWei = token === "USDT" ? usdtBalWei : bnbBalWei;
  const sufficient = quote ? balWei >= needWei : false;

  // Poll payment status until terminal.
  useEffect(() => {
    if (!payment || payment.status !== "pending") return;
    const id = payment.id;
    const t = setInterval(async () => {
      try {
        const next = await cryptoBillingApi.status(id);
        setPayment(next);
        if (next.status !== "pending") clearInterval(t);
      } catch {}
    }, 5000);
    return () => clearInterval(t);
  }, [payment]);

  async function handlePay() {
    if (!quote || !isConnected || !address) return;
    setError("");
    setSubmitting(true);
    try {
      let txHash: `0x${string}`;
      if (token === "USDT") {
        txHash = await writeContractAsync({
          abi: ERC20_ABI,
          address: usdtAddr,
          functionName: "transfer",
          args: [recipient, BigInt(quote.usdt_wei)],
          chainId: bsc.id,
        });
      } else {
        txHash = await sendTransactionAsync({
          to: recipient,
          value: BigInt(quote.bnb_wei),
          chainId: bsc.id,
        });
      }
      const row = await cryptoBillingApi.submit(txHash, token, months);
      setPayment(row);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="min-h-screen bg-[#0a0a0f] pt-24 pb-20 text-[#e2e8f0]">
      <div className="mx-auto max-w-xl px-4">
        <h1 className="mb-2 text-2xl font-bold">{t("title")}</h1>
        <p className="mb-6 text-sm text-[#94a3b8]">{t("subtitle")}</p>

        {/* Month selector */}
        <section className="mb-4 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-sm text-[#94a3b8]">{t("selectMonths")}</span>
            <span className="text-xs text-[#64748b]">{t("monthsHint")}</span>
          </div>
          <div className="grid grid-cols-4 gap-2">
            {MONTH_OPTIONS.map((m) => {
              const active = months === m;
              return (
                <button
                  key={m}
                  onClick={() => { setManualToken(null); setPayment(null); setMonths(m); }}
                  disabled={!!payment}
                  className={`rounded-xl border p-3 text-center transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
                    active
                      ? "border-[#8b5cf6] bg-[#8b5cf6]/10"
                      : "border-[#1e1e2e] hover:border-[#475569]"
                  }`}
                >
                  <div className="text-base font-semibold">{m}</div>
                  <div className="mt-0.5 text-[10px] text-[#94a3b8]">{t("monthsUnit")}</div>
                </button>
              );
            })}
          </div>
        </section>

        {/* Quote card */}
        <section className="mb-6 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5">
          <div className="flex items-baseline justify-between">
            <span className="text-sm text-[#94a3b8]">{t("price")}</span>
            <span className="text-2xl font-bold">${quote?.price_usdt ?? "—"} USDT</span>
          </div>
          {quote && quote.months > 1 && (
            <div className="mt-1 text-right text-xs text-[#64748b]">
              {t("priceBreakdown", { perMonth: quote.price_per_month_usdt, months: quote.months })}
            </div>
          )}
          <div className="mt-2 flex items-baseline justify-between text-sm">
            <span className="text-[#94a3b8]">{t("duration")}</span>
            <span>{quote?.duration_days ?? 30} {t("days")}</span>
          </div>
          {quote?.bnb_human && (
            <div className="mt-1 flex items-baseline justify-between text-sm">
              <span className="text-[#94a3b8]">{t("bnbEquiv")}</span>
              <span>≈ {quote.bnb_human} BNB ({t("withBuffer", { bps: quote.buffer_bps })})</span>
            </div>
          )}
          {quoteErr && <p className="mt-2 text-xs text-red-400">{quoteErr}</p>}
        </section>

        {/* Wallet connect */}
        {!isConnected ? (
          <section className="mb-6 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5 text-center">
            <p className="mb-3 text-sm text-[#94a3b8]">{t("connectPrompt")}</p>
            <div className="flex justify-center"><ConnectButton /></div>
          </section>
        ) : !isCorrectChain ? (
          <section className="mb-6 rounded-2xl border border-yellow-700/40 bg-yellow-900/10 p-5 text-center">
            <p className="mb-3 text-sm text-yellow-200">{t("wrongChain")}</p>
            <button
              onClick={() => switchChain({ chainId: bsc.id })}
              className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white hover:bg-[#a78bfa]"
            >
              {t("switchChain")}
            </button>
          </section>
        ) : (
          <>
            {/* Token selector */}
            <section className="mb-6 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5">
              <div className="mb-3 flex items-center justify-between">
                <span className="text-sm text-[#94a3b8]">{t("payWith")}</span>
                <span className="text-xs text-[#64748b]">
                  {manualToken == null ? t("auto") : t("manual")}
                </span>
              </div>
              <div className="grid grid-cols-2 gap-2">
                {(["USDT", "BNB"] as const).map((tk) => {
                  const active = token === tk;
                  const tkBal = tk === "USDT" ? usdtBalWei : bnbBalWei;
                  return (
                    <button
                      key={tk}
                      onClick={() => setManualToken(tk)}
                      className={`rounded-xl border p-3 text-left transition-colors ${
                        active
                          ? "border-[#8b5cf6] bg-[#8b5cf6]/10"
                          : "border-[#1e1e2e] hover:border-[#475569]"
                      }`}
                    >
                      <div className="text-sm font-semibold">{tk}</div>
                      <div className="mt-1 text-xs text-[#94a3b8]">
                        {t("balance")}: {fmt(tkBal.toString(), 18, tk === "BNB" ? 4 : 2)}
                      </div>
                    </button>
                  );
                })}
              </div>
              {manualToken != null && (
                <button
                  onClick={() => setManualToken(null)}
                  className="mt-2 text-xs text-[#94a3b8] underline-offset-2 hover:text-[#a78bfa] hover:underline"
                >
                  {t("resetAuto")}
                </button>
              )}
            </section>

            {/* Pay action */}
            {!payment && (
              <section className="mb-6 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5">
                <div className="mb-3 flex items-baseline justify-between text-sm">
                  <span className="text-[#94a3b8]">{t("youPay")}</span>
                  <span className="font-mono text-lg">
                    {fmt(needWei.toString(), 18, token === "BNB" ? 5 : 2)} {token}
                  </span>
                </div>
                {!sufficient && (
                  <p className="mb-3 text-xs text-red-400">{t("insufficient", { token })}</p>
                )}
                <button
                  onClick={handlePay}
                  disabled={submitting || !sufficient || !quote}
                  className="w-full rounded-lg bg-[#8b5cf6] px-4 py-3 text-sm font-semibold text-white hover:bg-[#a78bfa] disabled:opacity-50"
                >
                  {submitting ? t("submitting") : t("payNow")}
                </button>
                {error && <p className="mt-2 text-xs text-red-400">{error}</p>}
              </section>
            )}

            {/* Payment status */}
            {payment && (
              <section className="mb-6 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-5">
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm text-[#94a3b8]">{t("status")}</span>
                  <span
                    className={`text-sm font-semibold ${
                      payment.status === "confirmed"
                        ? "text-green-400"
                        : payment.status === "rejected"
                        ? "text-red-400"
                        : "text-yellow-300"
                    }`}
                  >
                    {t(`status_${payment.status}`)}
                  </span>
                </div>
                <div className="mb-2 text-xs break-all text-[#94a3b8]">
                  {t("txHash")}:{" "}
                  <a
                    href={`https://bscscan.com/tx/${payment.tx_hash}`}
                    target="_blank"
                    rel="noreferrer"
                    className="text-[#a78bfa] hover:underline"
                  >
                    {payment.tx_hash}
                  </a>
                </div>
                {payment.status === "pending" && (
                  <p className="text-xs text-[#94a3b8]">
                    {t("confirming", {
                      n: payment.confirmations,
                      total: 2,
                    })}
                  </p>
                )}
                {payment.status === "rejected" && payment.reject_reason && (
                  <p className="text-xs text-red-400">
                    {t("rejectReason")}: {t(`reject_${payment.reject_reason}`, { fallback: payment.reject_reason } as never)}
                  </p>
                )}
                {payment.status === "confirmed" && (
                  <button
                    onClick={() => router.push("/pricing")}
                    className="mt-3 w-full rounded-lg bg-[#8b5cf6] px-4 py-2.5 text-sm font-semibold text-white hover:bg-[#a78bfa]"
                  >
                    {t("done")}
                  </button>
                )}
              </section>
            )}
          </>
        )}

        <p className="mt-6 text-center text-xs text-[#64748b]">{t("note")}</p>
      </div>
    </main>
  );
}

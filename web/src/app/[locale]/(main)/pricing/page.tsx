"use client";

import { Fragment, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { billingApi, emailAuthApi, type BillingStatus, type EmailSessionInfo } from "@/lib/api";
import { PRICING } from "@/lib/pricing";
import PaymentMethodModal from "@/components/PaymentMethodModal";

const ACCENT = "#8b5cf6";

type RowVal = string | number | boolean;

type Row = { key: string; free: RowVal; pro: RowVal };
type Section = { key: string; rows: Row[] };

const SECTIONS: Section[] = [
  {
    key: "quota",
    rows: [
      { key: "monthlyCredits", free: PRICING.free.credits, pro: PRICING.pro.credits },
      { key: "creditsReset", free: "creditsResetValue", pro: "creditsResetValue" },
    ],
  },
  {
    key: "workspaces",
    rows: [
      { key: "workspaceCount", free: PRICING.free.workspaces, pro: PRICING.pro.workspaces },
    ],
  },
  {
    key: "generation",
    rows: [
      { key: "variants", free: PRICING.free.variants, pro: PRICING.pro.variants },
      { key: "longTweet", free: true, pro: true },
      { key: "batchGenerate", free: false, pro: true },
    ],
  },
  {
    key: "soul",
    rows: [
      { key: "soulBoost", free: false, pro: true },
    ],
  },
  {
    key: "memory",
    rows: [
      { key: "memoryAll", free: true, pro: true },
      { key: "twitterImport", free: true, pro: true },
      { key: "textImport", free: true, pro: true },
      { key: "autoAccept", free: false, pro: true },
      { key: "selfPortrait", free: true, pro: true },
    ],
  },
  {
    key: "support",
    rows: [
      { key: "supportLevel", free: "supportFree", pro: "supportPro" },
    ],
  },
];

function CellValue({ value, tRow }: { value: RowVal; tRow: (k: string) => string }) {
  if (typeof value === "boolean") {
    return value ? (
      <span className="text-[#22c55e]">✓</span>
    ) : (
      <span className="text-[#475569]">—</span>
    );
  }
  if (typeof value === "number") {
    return <span>{value.toLocaleString()}</span>;
  }
  // string referencing a translation key inside matrix.row
  return <span>{tRow(value)}</span>;
}

export default function PricingPage() {
  const t = useTranslations("Pricing");
  const tMatrix = useTranslations("Pricing.matrix");
  const tRow = useTranslations("Pricing.matrix.row");
  const tCat = useTranslations("Pricing.matrix.category");
  const tCalc = useTranslations("Pricing.calc");
  const tFaq = useTranslations("Pricing.faq");

  const [user, setUser] = useState<EmailSessionInfo | null>(null);
  const [status, setStatus] = useState<BillingStatus | null>(null);
  const [methodOpen, setMethodOpen] = useState(false);

  useEffect(() => {
    emailAuthApi.session().then(setUser).catch(() => setUser(null));
  }, []);

  useEffect(() => {
    if (!user) return;
    billingApi.status().then(setStatus).catch(() => setStatus(null));
  }, [user]);

  const isPro = status?.is_pro ?? false;
  const isLoggedIn = !!user;

  function handleUpgrade() {
    setMethodOpen(true);
  }

  function formatExpires(): string | null {
    if (!status?.pro_expires_at) return null;
    try {
      const d = new Date(status.pro_expires_at);
      return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
    } catch {
      return null;
    }
  }

  function daysRemaining(): number | null {
    if (!status?.pro_expires_at) return null;
    try {
      const d = new Date(status.pro_expires_at).getTime();
      const diff = d - Date.now();
      if (diff <= 0) return 0;
      return Math.ceil(diff / (1000 * 60 * 60 * 24));
    } catch {
      return null;
    }
  }

  return (
    <main className="min-h-screen bg-[#0a0a0f] pt-24 pb-20 text-[#e2e8f0]">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
        {/* Hero */}
        <div className="mb-12 text-center">
          <h1 className="text-3xl font-bold sm:text-4xl">{t("heroTitle")}</h1>
          <p className="mx-auto mt-4 max-w-2xl text-base text-[#94a3b8]">{t("heroSubtitle")}</p>
        </div>

        {/* Plan cards */}
        <div className="mx-auto grid max-w-4xl gap-6 sm:grid-cols-2">
          {/* Free card */}
          <div className="flex flex-col rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6">
            <div className="mb-4">
              <h2 className="text-lg font-semibold">{t("free.name")}</h2>
              <p className="mt-1 text-sm text-[#94a3b8]">{t("free.tagline")}</p>
            </div>
            <div className="mb-6">
              <span className="text-4xl font-bold">${PRICING.free.priceUSD}</span>
              <span className="ml-1 text-sm text-[#64748b]">{t("perMonth")}</span>
            </div>
            <ul className="mb-6 space-y-2 text-sm">
              <li>• {PRICING.free.credits} {tRow("monthlyCredits").toLowerCase()}</li>
              <li>• {PRICING.free.workspaces} {tRow("workspaceCount").toLowerCase()}</li>
              <li>• {tRow("variants")}: {PRICING.free.variants}</li>
              <li>• {tRow("memoryAll")}</li>
            </ul>
            {isLoggedIn ? (
              <Link
                href="/vibe-write"
                className="mt-auto block rounded-lg border border-[#1e1e2e] px-4 py-2.5 text-center text-sm font-semibold text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
              >
                {t("free.ctaLoggedIn")}
              </Link>
            ) : (
              <Link
                href="/vibe-write"
                className="mt-auto block rounded-lg border border-[#1e1e2e] px-4 py-2.5 text-center text-sm font-semibold text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
              >
                {t("free.cta")}
              </Link>
            )}
          </div>

          {/* Pro card */}
          <div
            className="relative flex flex-col rounded-2xl border-2 bg-[#14141f] p-6"
            style={{ borderColor: ACCENT }}
          >
            <div className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-[#8b5cf6] px-3 py-1 text-xs font-semibold text-white">
              {isPro ? t("currentPlan") : t("mostPopular")}
            </div>
            <div className="mb-4">
              <h2 className="text-lg font-semibold">{t("pro.name")}</h2>
              <p className="mt-1 text-sm text-[#94a3b8]">{t("pro.tagline")}</p>
            </div>
            <div className="mb-6">
              <span className="text-4xl font-bold">${PRICING.pro.priceUSD}</span>
              <span className="ml-1 text-sm text-[#64748b]">{t("perMonth")}</span>
            </div>
            <ul className="mb-6 space-y-2 text-sm">
              <li>• {PRICING.pro.credits.toLocaleString()} {tRow("monthlyCredits").toLowerCase()}</li>
              <li>• {PRICING.pro.workspaces} {tRow("workspaceCount").toLowerCase()}</li>
              <li>• {tRow("variants")}: {PRICING.pro.variants}</li>
              <li>• {tRow("autoAccept")}</li>
              <li>• {tRow("soulBoost")}</li>
              <li>• {tRow("batchGenerate")}</li>
            </ul>
            {isPro ? (
              <div className="mt-auto space-y-2">
                <button
                  onClick={handleUpgrade}
                  className="block w-full rounded-lg bg-[#8b5cf6] px-4 py-2.5 text-center text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
                >
                  {t("pro.ctaRenew")}
                </button>
                {formatExpires() && (
                  <p className="text-center text-xs text-[#64748b]">
                    {t("proExpiresAt", { date: formatExpires() ?? "" })}
                    {daysRemaining() !== null && (
                      <> · {t("daysRemaining", { n: daysRemaining()! })}</>
                    )}
                  </p>
                )}
              </div>
            ) : (
              <button
                onClick={handleUpgrade}
                className="mt-auto block rounded-lg bg-[#8b5cf6] px-4 py-2.5 text-center text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
              >
                {t("pro.cta")}
              </button>
            )}
          </div>
        </div>

        {/* Comparison matrix */}
        <section className="mt-20">
          <h2 className="mb-6 text-center text-2xl font-bold">{tMatrix("title")}</h2>
          <div className="overflow-hidden rounded-2xl border border-[#1e1e2e]">
            <table className="w-full text-sm">
              <thead className="bg-[#14141f] text-[#94a3b8]">
                <tr>
                  <th className="px-4 py-3 text-left font-medium">&nbsp;</th>
                  <th className="px-4 py-3 text-center font-medium">{t("free.name")}</th>
                  <th className="px-4 py-3 text-center font-medium" style={{ color: ACCENT }}>
                    {t("pro.name")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {SECTIONS.map((section) => (
                  <Fragment key={section.key}>
                    <tr className="bg-[#0e0e17]">
                      <td colSpan={3} className="px-4 py-2 text-xs font-semibold uppercase tracking-wider text-[#64748b]">
                        {tCat(section.key)}
                      </td>
                    </tr>
                    {section.rows.map((row) => (
                      <tr key={`${section.key}-${row.key}`} className="border-t border-[#1e1e2e]">
                        <td className="px-4 py-3 text-[#e2e8f0]">{tRow(row.key)}</td>
                        <td className="px-4 py-3 text-center text-[#94a3b8]">
                          <CellValue value={row.free} tRow={tRow} />
                        </td>
                        <td className="px-4 py-3 text-center text-[#e2e8f0]">
                          <CellValue value={row.pro} tRow={tRow} />
                        </td>
                      </tr>
                    ))}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Credits calculator */}
        <section className="mt-16 rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-6 sm:p-8">
          <h2 className="mb-4 text-xl font-semibold">{tCalc("title")}</h2>
          <ul className="space-y-2 text-sm text-[#94a3b8]">
            <li>• {tCalc("line1")}</li>
            <li>• {tCalc("line2")}</li>
            <li>• {tCalc("line3")}</li>
            <li>• {tCalc("line4")}</li>
          </ul>
          <p className="mt-4 border-t border-[#1e1e2e] pt-4 text-sm text-[#e2e8f0]">{tCalc("outro")}</p>
        </section>

        {/* FAQ */}
        <section className="mt-16">
          <h2 className="mb-6 text-center text-2xl font-bold">{tFaq("title")}</h2>
          <div className="mx-auto max-w-3xl space-y-4">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
                <p className="font-semibold text-[#e2e8f0]">{tFaq(`q${i}`)}</p>
                <p className="mt-2 text-sm text-[#94a3b8]">{tFaq(`a${i}`)}</p>
              </div>
            ))}
          </div>
        </section>

        {/* Final CTA */}
        {!isPro ? (
          <section className="mt-20 rounded-2xl border border-[#1e1e2e] bg-gradient-to-br from-[#14141f] to-[#0e0e17] p-8 text-center">
            <h2 className="text-2xl font-bold">{t("finalCtaTitle")}</h2>
            <p className="mx-auto mt-2 max-w-xl text-sm text-[#94a3b8]">{t("finalCtaSubtitle")}</p>
            <button
              onClick={handleUpgrade}
              className="mt-6 rounded-lg bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              {t("finalCtaButton")}
            </button>
          </section>
        ) : (
          <section className="mt-20 rounded-2xl border border-[#1e1e2e] bg-gradient-to-br from-[#14141f] to-[#0e0e17] p-8 text-center">
            <h2 className="text-2xl font-bold">{t("renewCtaTitle")}</h2>
            <p className="mx-auto mt-2 max-w-xl text-sm text-[#94a3b8]">
              {formatExpires() && (
                <>
                  {t("proExpiresAt", { date: formatExpires() ?? "" })}
                  {daysRemaining() !== null && (
                    <> · {t("daysRemaining", { n: daysRemaining()! })}</>
                  )}
                </>
              )}
            </p>
            <button
              onClick={handleUpgrade}
              className="mt-6 rounded-lg bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              {t("renewCtaButton")}
            </button>
          </section>
        )}
      </div>
      <PaymentMethodModal open={methodOpen} onClose={() => setMethodOpen(false)} />
    </main>
  );
}

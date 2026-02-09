"use client";

import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import StatsBar from "@/components/StatsBar";
import FeaturedSouls from "@/components/FeaturedSouls";

export default function Home() {
  const t = useTranslations("Home");

  return (
    <div className="pt-16">
      {/* Hero Section */}
      <section className="flex min-h-[70vh] flex-col items-center justify-center px-4 text-center">
        <h1 className="mb-4 text-5xl font-bold tracking-tight text-[#e2e8f0] sm:text-6xl lg:text-7xl">
          {t("heroTitle")}{" "}
          <span className="text-[#8b5cf6]">{t("heroHighlight")}</span>
        </h1>
        <p className="mb-8 max-w-xl text-lg text-[#94a3b8]">
          {t("heroDesc")}
        </p>
        <div className="flex gap-4">
          <Link
            href="/explore"
            className="rounded-lg bg-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
          >
            {t("exploreSouls")}
          </Link>
          <Link
            href="/mint"
            className="rounded-lg border border-[#1e1e2e] px-6 py-3 text-sm font-semibold text-[#e2e8f0] transition-colors hover:border-[#8b5cf6] hover:text-[#8b5cf6]"
          >
            {t("mintShell")}
          </Link>
        </div>
      </section>

      {/* Stats Bar */}
      <StatsBar />

      {/* How It Works */}
      <section className="mx-auto max-w-5xl px-4 py-20">
        <h2 className="mb-12 text-center text-3xl font-bold text-[#e2e8f0]">
          {t("howItWorks")}
        </h2>
        <div className="grid gap-8 md:grid-cols-3">
          {[
            {
              step: "01",
              titleKey: "step01Title" as const,
              descKey: "step01Desc" as const,
              stepKey: "step01" as const,
              icon: "🥚",
            },
            {
              step: "02",
              titleKey: "step02Title" as const,
              descKey: "step02Desc" as const,
              stepKey: "step02" as const,
              icon: "🦞",
            },
            {
              step: "03",
              titleKey: "step03Title" as const,
              descKey: "step03Desc" as const,
              stepKey: "step03" as const,
              icon: "✨",
            },
          ].map((item) => (
            <div
              key={item.step}
              className="rounded-lg border border-[#1e1e2e] bg-[#14141f] p-6 transition-colors hover:border-[#8b5cf6]/30"
            >
              <div className="mb-3 text-4xl">{item.icon}</div>
              <div className="mb-1 font-mono text-xs text-[#8b5cf6]">
                {t(item.stepKey)}
              </div>
              <h3 className="mb-2 text-xl font-semibold text-[#e2e8f0]">
                {t(item.titleKey)}
              </h3>
              <p className="text-sm leading-relaxed text-[#94a3b8]">
                {t(item.descKey)}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Featured Souls */}
      <section className="mx-auto max-w-5xl px-4 py-10 pb-20">
        <h2 className="mb-8 text-center text-3xl font-bold text-[#e2e8f0]">
          {t("featuredSouls")}
        </h2>
        <FeaturedSouls />
      </section>
    </div>
  );
}

"use client";

import Image from "next/image";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";

export default function Footer() {
  const t = useTranslations("Footer");

  return (
    <footer className="border-t border-[#1e1e2e] bg-[#0a0a0f] py-8">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col items-center justify-between gap-4 sm:flex-row">
          <div className="flex flex-col items-center gap-1 sm:items-start">
            <div className="flex items-center gap-2">
              <Image src="/logo.png" alt="Ensoul" width={48} height={48} className="rounded-md" />
              <span className="text-lg font-bold text-[#8b5cf6]">Ensoul</span>
            </div>
            <span className="text-sm text-[#94a3b8]">
              {t("tagline")}
            </span>
          </div>
          <div className="flex items-center gap-6">
            <a
              href="https://x.com/NemoBuilder"
              target="_blank"
              className="text-sm text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
            >
              𝕏
            </a>
            <a
              href="https://github.com/NemoBuilder/ensoul"
              target="_blank"
              className="text-sm text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
            >
              GitHub
            </a>
            <Link
              href="/docs"
              className="text-sm text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
            >
              {t("docs")}
            </Link>
          </div>
        </div>
        <div className="mt-4 text-center text-xs text-[#94a3b8]/60">
          {t("copyright")}
        </div>
      </div>
    </footer>
  );
}

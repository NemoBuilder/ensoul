"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { ConnectButton } from "@rainbow-me/rainbowkit";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import LoginModal from "@/components/LoginModal";
import { emailAuthApi, type EmailSessionInfo } from "@/lib/api";

export default function Navbar() {
  const t = useTranslations("Navbar");
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Email auth state
  const [loginOpen, setLoginOpen] = useState(false);
  const [emailUser, setEmailUser] = useState<EmailSessionInfo | null>(null);
  const [checkingEmail, setCheckingEmail] = useState(true);

  // Check email session on mount
  useEffect(() => {
    emailAuthApi
      .session()
      .then((user) => setEmailUser(user))
      .catch(() => setEmailUser(null))
      .finally(() => setCheckingEmail(false));
  }, []);

  const handleEmailLogin = useCallback((user: EmailSessionInfo) => {
    setEmailUser(user);
  }, []);

  const handleEmailLogout = useCallback(async () => {
    setMenuOpen(false);
    try {
      await emailAuthApi.logout();
    } catch {
      // ignore
    }
    setEmailUser(null);
  }, []);

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Close menu on route change
  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  return (
    <>
    <nav className="fixed top-0 left-0 right-0 z-50 border-b border-[#1e1e2e] bg-[#0a0a0f]/80 backdrop-blur-md">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2">
            <Image src="/logo.png" alt="Ensoul" width={32} height={32} className="rounded-md" />
            <span className="text-xl font-bold text-[#8b5cf6]">Ensoul</span>
          </Link>

          {/* Navigation links */}
          <div className="flex items-center gap-6">
            {[
              { href: "/vibe-write" as const, label: t("vibe-write") },
              { href: "/pricing" as const, label: t("pricing") },
              { href: "/explore" as const, label: t("souls") },
              { href: "/protocol" as const, label: t("protocol") },
            ].map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={`hidden text-sm font-medium transition-colors sm:block ${
                  pathname === item.href
                    ? "text-[#8b5cf6]"
                    : "text-[#94a3b8] hover:text-[#e2e8f0]"
                }`}
              >
                {item.label}
              </Link>
            ))}

            <LanguageSwitcher />

            {/* Email login — primary auth method */}
            {!checkingEmail && !emailUser && (
              <button
                onClick={() => setLoginOpen(true)}
                className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
              >
                {t("login")}
              </button>
            )}

            {/* Logged-in email user menu */}
            {emailUser && (
              <div className="relative" ref={menuRef}>
                <button
                  onClick={() => setMenuOpen(!menuOpen)}
                  className="flex items-center gap-2 rounded-lg border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
                >
                  <span className="text-base">👤</span>
                  <span className="max-w-[120px] truncate">
                    {emailUser.email.split("@")[0]}
                  </span>
                  {emailUser.is_pro && (
                    <span className="rounded bg-[#8b5cf6] px-1.5 py-0.5 text-[10px] font-bold text-white">PRO</span>
                  )}
                  <svg
                    className={`h-3 w-3 text-[#94a3b8] transition-transform ${menuOpen ? "rotate-180" : ""}`}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>

                {menuOpen && (
                  <div className="absolute right-0 mt-2 w-56 overflow-hidden rounded-lg border border-[#1e1e2e] bg-[#14141f] shadow-xl">
                    <div className="border-b border-[#1e1e2e] px-4 py-3">
                      <p className="truncate text-sm text-[#e2e8f0]">{emailUser.email}</p>
                      <p className="text-xs text-[#94a3b8]">
                        {emailUser.is_pro ? "Pro" : "Free"} · {emailUser.credits} {t("credits")}
                      </p>
                      {emailUser.is_pro && emailUser.pro_expires_at && (
                        <p className="mt-1 text-[10px] text-[#64748b]">
                          {t("proExpiresOn", {
                            date: new Date(emailUser.pro_expires_at).toLocaleDateString(undefined, {
                              year: "numeric",
                              month: "short",
                              day: "numeric",
                            }),
                          })}
                        </p>
                      )}
                    </div>
                    <Link
                      href="/pricing"
                      className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                    >
                      <span>{emailUser.is_pro ? "🔄" : "⭐"}</span>
                      <span>{emailUser.is_pro ? t("renewSubscription") : t("upgradeToPro")}</span>
                    </Link>
                    <Link
                      href="/my-souls"
                      className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                    >
                      <span>🧬</span>
                      <span>{t("mySouls")}</span>
                    </Link>
                    <Link
                      href="/protocol"
                      className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                    >
                      <span>📊</span>
                      <span>{t("protocol")}</span>
                    </Link>
                    <Link
                      href="/settings"
                      className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                    >
                      <span>⚙️</span>
                      <span>{t("settings")}</span>
                    </Link>
                    <div className="border-t border-[#1e1e2e]" />
                    <button
                      onClick={handleEmailLogout}
                      className="flex w-full items-center gap-2 px-4 py-3 text-sm text-red-400 transition-colors hover:bg-[#1e1e2e]"
                    >
                      <span>🚪</span>
                      <span>{t("logout")}</span>
                    </button>
                  </div>
                )}
              </div>
            )}

            {/* Wallet connect — secondary, for Protocol features */}
            <ConnectButton.Custom>
              {({
                account,
                chain,
                openAccountModal,
                openChainModal,
                openConnectModal,
                mounted,
              }) => {
                const connected = mounted && account && chain;
                if (!mounted) return <div aria-hidden="true" style={{ opacity: 0, pointerEvents: "none", userSelect: "none" }} />;
                if (!connected) {
                  return (
                    <button
                      onClick={openConnectModal}
                      className="rounded-lg border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                      title={t("connectWallet")}
                    >
                      🔗
                    </button>
                  );
                }
                if (chain.unsupported) {
                  return (
                    <button onClick={openChainModal} className="rounded-lg bg-red-500 px-3 py-2 text-sm font-semibold text-white">
                      {t("wrongNetwork")}
                    </button>
                  );
                }
                return (
                  <button
                    onClick={openAccountModal}
                    className="flex items-center gap-1.5 rounded-lg border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#94a3b8] transition-colors hover:border-[#8b5cf6] hover:text-[#e2e8f0]"
                    title={account.displayName}
                  >
                    {chain.hasIcon && chain.iconUrl && (
                      <Image src={chain.iconUrl} alt={chain.name ?? "Chain"} width={14} height={14} className="rounded-full" />
                    )}
                    <span className="font-mono text-xs">{account.displayName}</span>
                  </button>
                );
              }}
            </ConnectButton.Custom>
          </div>
        </div>
      </div>
    </nav>

    {/* Login modal — rendered outside nav to avoid stacking context issues */}
    <LoginModal open={loginOpen} onClose={() => setLoginOpen(false)} onLogin={handleEmailLogin} />
    </>
  );
}

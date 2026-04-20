"use client";

import { useState, useEffect, useCallback } from "react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { emailAuthApi, type EmailSessionInfo } from "@/lib/api";
import LoginModal from "@/components/LoginModal";
import UpgradeModal from "@/components/UpgradeModal";
import SetPasswordModal from "@/components/SetPasswordModal";
import LanguageSwitcher from "@/components/LanguageSwitcher";

export default function WorkspaceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = useTranslations("Workspace");
  const [user, setUser] = useState<EmailSessionInfo | null>(null);
  const [loginOpen, setLoginOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [setPasswordOpen, setSetPasswordOpen] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    emailAuthApi
      .session()
      .then((u) => setUser(u))
      .catch(() => setUser(null))
      .finally(() => setChecking(false));
  }, []);

  // Listen for auth changes from child pages (e.g. vibe-write demo login)
  useEffect(() => {
    function onAuthChanged() {
      emailAuthApi.session()
        .then((u) => setUser(u))
        .catch(() => setUser(null));
    }
    window.addEventListener("ensoul:auth-changed", onAuthChanged);
    return () => window.removeEventListener("ensoul:auth-changed", onAuthChanged);
  }, []);

  const handleLogin = useCallback((u: EmailSessionInfo) => {
    setUser(u);
    window.dispatchEvent(new Event("ensoul:auth-changed"));
  }, []);

  const handleLogout = useCallback(async () => {
    try {
      await emailAuthApi.logout();
    } catch {
      // ignore
    }
    setUser(null);
    window.dispatchEvent(new Event("ensoul:auth-changed"));
  }, []);

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-[#0a0a0f]">
      {/* Top bar — minimal, no global nav */}
      <header className="flex h-12 shrink-0 items-center justify-between border-b border-[#1e1e2e] px-4">
        {/* Left: Logo → back to main site */}
        <Link href="/" className="flex items-center gap-2">
          <Image src="/logo.png" alt="Ensoul" width={24} height={24} className="rounded-md" />
          <span className="text-sm font-bold text-[#8b5cf6]">Ensoul</span>
        </Link>

        {/* Right: User info or login */}
        <div className="flex items-center gap-3">
          <LanguageSwitcher />
          {checking ? (
            <div className="h-4 w-20 animate-pulse rounded bg-[#1e1e2e]" />
          ) : user ? (
            <div className="flex items-center gap-2">
              <span className="text-xs text-[#94a3b8]">
                {user.credits} {t("credits")}
              </span>
              {user.is_pro ? (
                <span className="rounded bg-[#8b5cf6] px-1.5 py-0.5 text-[10px] font-bold text-white">PRO</span>
              ) : (
                <button
                  onClick={() => setUpgradeOpen(true)}
                  className="rounded bg-gradient-to-r from-[#8b5cf6] to-[#a78bfa] px-2 py-0.5 text-[10px] font-bold text-white transition-opacity hover:opacity-90"
                >
                  ↑ PRO
                </button>
              )}
              <button
                onClick={handleLogout}
                className="text-xs text-[#94a3b8] transition-colors hover:text-red-400"
                title={t("logout")}
              >
                {user.email.split("@")[0]}
              </button>
              {!user.has_password && (
                <button
                  onClick={() => setSetPasswordOpen(true)}
                  className="rounded border border-[#8b5cf6]/30 px-1.5 py-0.5 text-[10px] text-[#8b5cf6] transition-colors hover:bg-[#8b5cf6]/10"
                  title={t("setPassword")}
                >
                  🔑
                </button>
              )}
            </div>
          ) : (
            <button
              onClick={() => setLoginOpen(true)}
              className="rounded-md bg-[#8b5cf6] px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#a78bfa]"
            >
              {t("login")}
            </button>
          )}
        </div>
      </header>

      {/* Workspace content — full remaining height */}
      <div className="flex-1 overflow-hidden">
        {children}
      </div>

      {/* Login modal */}
      <LoginModal open={loginOpen} onClose={() => setLoginOpen(false)} onLogin={handleLogin} />
      <UpgradeModal open={upgradeOpen} onClose={() => setUpgradeOpen(false)} reason="feature" />
      {user && (
        <SetPasswordModal
          open={setPasswordOpen}
          onClose={() => {
            setSetPasswordOpen(false);
            // Refresh user info to update has_password
            emailAuthApi.session().then((u) => setUser(u)).catch(() => {});
          }}
          hasPassword={user.has_password}
        />
      )}
    </div>
  );
}

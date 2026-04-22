"use client";

import { useState, useEffect, useCallback, useRef } from "react";
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
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

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
    if (!window.confirm(t("logoutConfirm"))) return;
    try {
      await emailAuthApi.logout();
    } catch {
      // ignore
    }
    setMenuOpen(false);
    setUser(null);
    window.dispatchEvent(new Event("ensoul:auth-changed"));
  }, [t]);

  // Click-outside to close menu
  useEffect(() => {
    if (!menuOpen) return;
    function onDocClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setMenuOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

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
              {!user.is_pro && (
                <button
                  onClick={() => setUpgradeOpen(true)}
                  className="rounded bg-gradient-to-r from-[#8b5cf6] to-[#a78bfa] px-2 py-0.5 text-[10px] font-bold text-white transition-opacity hover:opacity-90"
                >
                  ↑ PRO
                </button>
              )}
              <div className="relative" ref={menuRef}>
                <button
                  onClick={() => setMenuOpen((v) => !v)}
                  aria-haspopup="menu"
                  aria-expanded={menuOpen}
                  className="flex items-center gap-1 rounded-md px-1.5 py-1 text-xs text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                  title={user.email}
                >
                  <span className="flex h-6 w-6 items-center justify-center rounded-full bg-[#8b5cf6]/20 text-[10px] font-bold text-[#8b5cf6]">
                    {user.email.charAt(0).toUpperCase()}
                  </span>
                  <span className="max-w-[120px] truncate">{user.email.split("@")[0]}</span>
                  <span className="text-[#64748b]">{menuOpen ? "▴" : "▾"}</span>
                </button>
                {menuOpen && (
                  <div
                    role="menu"
                    className="absolute right-0 top-full z-50 mt-1 w-64 overflow-hidden rounded-lg border border-[#1e1e2e] bg-[#14141f] shadow-2xl"
                  >
                    <div className="border-b border-[#1e1e2e] px-3 py-2.5">
                      <div className="truncate text-xs text-[#e2e8f0]" title={user.email}>{user.email}</div>
                      <div className="mt-1 flex items-center gap-2 text-[11px] text-[#64748b]">
                        {user.is_pro && (
                          <span className="rounded bg-[#8b5cf6] px-1.5 py-0.5 text-[10px] font-bold text-white">PRO</span>
                        )}
                        <span>{user.credits} {t("credits")}</span>
                      </div>
                    </div>
                    <div className="py-1">
                      {!user.has_password && (
                        <button
                          role="menuitem"
                          onClick={() => { setMenuOpen(false); setSetPasswordOpen(true); }}
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-[#e2e8f0] hover:bg-[#1e1e2e]"
                        >
                          <span>🔑</span><span>{t("setPassword")}</span>
                        </button>
                      )}
                      {!user.is_pro && (
                        <button
                          role="menuitem"
                          onClick={() => { setMenuOpen(false); setUpgradeOpen(true); }}
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-[#e2e8f0] hover:bg-[#1e1e2e]"
                        >
                          <span>💎</span><span>{t("upgradeToPro")}</span>
                        </button>
                      )}
                    </div>
                    <div className="border-t border-[#1e1e2e] py-1">
                      <button
                        role="menuitem"
                        onClick={handleLogout}
                        className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-[#94a3b8] transition-colors hover:bg-red-500/10 hover:text-red-400"
                      >
                        <span>🚪</span><span>{t("logout")}</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
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

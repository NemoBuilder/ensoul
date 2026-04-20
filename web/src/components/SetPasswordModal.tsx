"use client";

import { useState, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import { emailAuthApi } from "@/lib/api";

interface SetPasswordModalProps {
  open: boolean;
  onClose: () => void;
  hasPassword: boolean;
}

export default function SetPasswordModal({ open, onClose, hasPassword }: SetPasswordModalProps) {
  const t = useTranslations("Auth");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 100);
    } else {
      setPassword("");
      setConfirm("");
      setError("");
      setSuccess(false);
      setShowPassword(false);
    }
  }, [open]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (loading) return;

    if (password.length < 8) {
      setError(t("passwordTooShort"));
      return;
    }
    if (password !== confirm) {
      setError(t("passwordMismatch"));
      return;
    }

    setError("");
    setLoading(true);
    try {
      await emailAuthApi.setPassword(password);
      setSuccess(true);
      setTimeout(() => onClose(), 1500);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setLoading(false);
    }
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="relative mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-8 shadow-2xl">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
          aria-label="Close"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        <div className="mb-6 text-center">
          <div className="mb-3 text-4xl">🔑</div>
          <h2 className="text-xl font-bold text-[#e2e8f0]">
            {hasPassword ? t("changePassword") : t("setPassword")}
          </h2>
          <p className="mt-1 text-sm text-[#94a3b8]">
            {t("setPasswordDesc")}
          </p>
        </div>

        {success ? (
          <div className="text-center">
            <div className="mb-2 text-4xl">✅</div>
            <p className="text-sm text-green-400">{t("passwordSetSuccess")}</p>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            <div className="relative mb-3">
              <input
                ref={inputRef}
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("newPasswordPlaceholder")}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 pr-10 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none transition-colors focus:border-[#8b5cf6]"
                autoComplete="new-password"
                required
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute top-1/2 right-3 -translate-y-1/2 text-[#94a3b8] hover:text-[#e2e8f0]"
              >
                {showPassword ? (
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                  </svg>
                ) : (
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                  </svg>
                )}
              </button>
            </div>
            <input
              type={showPassword ? "text" : "password"}
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              placeholder={t("confirmPasswordPlaceholder")}
              className="mb-4 w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none transition-colors focus:border-[#8b5cf6]"
              autoComplete="new-password"
              required
            />
            {error && <p className="mb-3 text-sm text-red-400">{error}</p>}
            <button
              type="submit"
              disabled={loading || !password || !confirm}
              className="w-full rounded-lg bg-[#8b5cf6] py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {loading ? "..." : hasPassword ? t("changePassword") : t("setPassword")}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

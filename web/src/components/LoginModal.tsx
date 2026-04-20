"use client";

import { useState, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import { emailAuthApi, type EmailSessionInfo } from "@/lib/api";

interface LoginModalProps {
  open: boolean;
  onClose: () => void;
  onLogin: (user: EmailSessionInfo) => void;
}

export default function LoginModal({ open, onClose, onLogin }: LoginModalProps) {
  const t = useTranslations("Auth");
  const [step, setStep] = useState<"email" | "password" | "code">("email");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [showPassword, setShowPassword] = useState(false);

  const codeInputRef = useRef<HTMLInputElement>(null);
  const emailInputRef = useRef<HTMLInputElement>(null);
  const passwordInputRef = useRef<HTMLInputElement>(null);

  // Auto-focus email input when modal opens
  useEffect(() => {
    if (open && step === "email") {
      setTimeout(() => emailInputRef.current?.focus(), 100);
    }
  }, [open, step]);

  // Auto-focus code input when switching to code step
  useEffect(() => {
    if (step === "code") {
      setTimeout(() => codeInputRef.current?.focus(), 100);
    }
  }, [step]);

  // Auto-focus password input
  useEffect(() => {
    if (step === "password") {
      setTimeout(() => passwordInputRef.current?.focus(), 100);
    }
  }, [step]);

  // Cooldown timer
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((c) => c - 1), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  // Reset state when modal closes
  useEffect(() => {
    if (!open) {
      setStep("email");
      setPassword("");
      setCode("");
      setError("");
      setLoading(false);
      setShowPassword(false);
    }
  }, [open]);

  async function handleEmailSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim() || loading) return;

    setError("");
    setLoading(true);
    try {
      const trimmedEmail = email.trim().toLowerCase();
      // Check if user has a password
      const { has_password } = await emailAuthApi.hasPassword(trimmedEmail);
      if (has_password) {
        setStep("password");
      } else {
        // No password — send verification code
        await emailAuthApi.sendCode(trimmedEmail);
        setStep("code");
        setCooldown(60);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setLoading(false);
    }
  }

  async function handlePasswordLogin(e: React.FormEvent) {
    e.preventDefault();
    if (!password.trim() || loading) return;

    setError("");
    setLoading(true);
    try {
      await emailAuthApi.passwordLogin(email.trim().toLowerCase(), password);
      const session = await emailAuthApi.session();
      onLogin(session);
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("loginFailed"));
    } finally {
      setLoading(false);
    }
  }

  async function handleSwitchToCode() {
    if (loading) return;
    setError("");
    setLoading(true);
    try {
      await emailAuthApi.sendCode(email.trim().toLowerCase());
      setStep("code");
      setCooldown(60);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to send code");
    } finally {
      setLoading(false);
    }
  }

  async function handleVerify(e: React.FormEvent) {
    e.preventDefault();
    if (!code.trim() || loading) return;

    setError("");
    setLoading(true);
    try {
      await emailAuthApi.verify(email.trim().toLowerCase(), code.trim());
      // Fetch full session info
      const session = await emailAuthApi.session();
      onLogin(session);
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleResend() {
    if (cooldown > 0 || loading) return;
    setError("");
    setLoading(true);
    try {
      await emailAuthApi.sendCode(email.trim().toLowerCase());
      setCooldown(60);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to resend code");
    } finally {
      setLoading(false);
    }
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div className="relative mx-4 w-full max-w-md rounded-2xl border border-[#1e1e2e] bg-[#14141f] p-8 shadow-2xl" onClick={(e) => e.stopPropagation()}>
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
          aria-label="Close"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        {/* Header */}
        <div className="mb-6 text-center">
          <div className="mb-3 text-4xl">✉️</div>
          <h2 className="text-xl font-bold text-[#e2e8f0]">
            {step === "email" ? t("loginTitle") : step === "password" ? t("passwordTitle") : t("verifyTitle")}
          </h2>
          <p className="mt-1 text-sm text-[#94a3b8]">
            {step === "email" ? t("loginDesc") : step === "password" ? t("passwordDesc") : t("verifyDesc")}
          </p>
        </div>

        {/* Email step */}
        {step === "email" && (
          <form onSubmit={handleEmailSubmit}>
            <input
              ref={emailInputRef}
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("emailPlaceholder")}
              className="mb-4 w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none transition-colors focus:border-[#8b5cf6]"
              autoComplete="email"
              required
            />
            {error && <p className="mb-3 text-sm text-red-400">{error}</p>}
            <button
              type="submit"
              disabled={loading || !email.trim()}
              className="w-full rounded-lg bg-[#8b5cf6] py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {loading ? t("sending") : t("sendCode")}
            </button>
            <p className="mt-3 text-center text-xs text-[#94a3b8]/70">
              {t("noAccountHint")}
            </p>
          </form>
        )}

        {/* Password step */}
        {step === "password" && (
          <form onSubmit={handlePasswordLogin}>
            <p className="mb-3 text-center text-sm text-[#94a3b8]">
              <span className="font-medium text-[#e2e8f0]">{email}</span>
            </p>
            <div className="relative mb-4">
              <input
                ref={passwordInputRef}
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("passwordPlaceholder")}
                className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 pr-10 text-sm text-[#e2e8f0] placeholder-[#94a3b8]/50 outline-none transition-colors focus:border-[#8b5cf6]"
                autoComplete="current-password"
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
            {error && <p className="mb-3 text-sm text-red-400">{error}</p>}
            <button
              type="submit"
              disabled={loading || !password.trim()}
              className="w-full rounded-lg bg-[#8b5cf6] py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {loading ? t("verifying") : t("loginButton")}
            </button>
            <div className="mt-4 flex items-center justify-between text-sm">
              <button
                type="button"
                onClick={() => { setStep("email"); setPassword(""); setError(""); }}
                className="text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
              >
                {t("changeEmail")}
              </button>
              <button
                type="button"
                onClick={handleSwitchToCode}
                disabled={loading}
                className="text-[#8b5cf6] transition-colors hover:text-[#a78bfa] disabled:text-[#94a3b8]/50"
              >
                {t("useCode")}
              </button>
            </div>
          </form>
        )}

        {/* Code step */}
        {step === "code" && (
          <form onSubmit={handleVerify}>
            <p className="mb-3 text-center text-sm text-[#94a3b8]">
              {t("codeSentTo")} <span className="font-medium text-[#e2e8f0]">{email}</span>
            </p>
            <input
              ref={codeInputRef}
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
              placeholder="000000"
              className="mb-4 w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-4 py-3 text-center font-mono text-2xl tracking-[0.5em] text-[#e2e8f0] placeholder-[#94a3b8]/30 outline-none transition-colors focus:border-[#8b5cf6]"
              maxLength={6}
              inputMode="numeric"
              autoComplete="one-time-code"
              required
            />
            {error && <p className="mb-3 text-sm text-red-400">{error}</p>}
            <button
              type="submit"
              disabled={loading || code.length !== 6}
              className="w-full rounded-lg bg-[#8b5cf6] py-3 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50"
            >
              {loading ? t("verifying") : t("verify")}
            </button>
            <div className="mt-4 flex items-center justify-between text-sm">
              <button
                type="button"
                onClick={() => { setStep("email"); setCode(""); setError(""); }}
                className="text-[#94a3b8] transition-colors hover:text-[#e2e8f0]"
              >
                {t("changeEmail")}
              </button>
              <button
                type="button"
                onClick={handleResend}
                disabled={cooldown > 0 || loading}
                className="text-[#8b5cf6] transition-colors hover:text-[#a78bfa] disabled:text-[#94a3b8]/50"
              >
                {cooldown > 0 ? `${t("resend")} (${cooldown}s)` : t("resend")}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}

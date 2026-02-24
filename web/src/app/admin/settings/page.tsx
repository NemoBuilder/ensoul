"use client";

import { useState } from "react";
import { adminAuthApi } from "@/lib/admin-api";

export default function SettingsPage() {
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSuccess("");

    if (newPassword.length < 8) {
      setError("New password must be at least 8 characters");
      return;
    }

    if (newPassword !== confirmPassword) {
      setError("New passwords do not match");
      return;
    }

    setLoading(true);
    try {
      const res = await adminAuthApi.changePassword(oldPassword, newPassword);
      setSuccess(res.message);
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
      // After password change, server invalidates sessions
      // Redirect to login after a delay
      setTimeout(() => {
        window.location.href = "/admin/login";
      }, 2000);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to change password");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-md space-y-6">
      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-4 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          Change Password
        </h2>

        {error && (
          <div className="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
            {error}
          </div>
        )}
        {success && (
          <div className="mb-4 rounded-lg border border-green-500/30 bg-green-500/10 px-4 py-3 text-sm text-green-400">
            {success}
          </div>
        )}

        <form onSubmit={handleChangePassword} className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-[#94a3b8]">
              Current Password
            </label>
            <input
              type="password"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              required
              autoComplete="current-password"
              className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium text-[#94a3b8]">
              New Password
            </label>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              minLength={8}
              autoComplete="new-password"
              className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
            />
            <p className="mt-1 text-xs text-[#4a4a5a]">Minimum 8 characters</p>
          </div>

          <div>
            <label className="mb-1.5 block text-sm font-medium text-[#94a3b8]">
              Confirm New Password
            </label>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full rounded-lg border border-[#1e1e2e] bg-[#0a0a0f] px-3 py-2.5 text-sm text-[#e2e8f0] outline-none focus:border-[#8b5cf6]"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-[#8b5cf6] px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa] disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? "Changing..." : "Change Password"}
          </button>
        </form>
      </div>

      <div className="rounded-xl border border-[#1e1e2e] bg-[#14141f] p-5">
        <h2 className="mb-3 text-sm font-semibold text-[#94a3b8] uppercase tracking-wider">
          About
        </h2>
        <div className="space-y-2 text-sm text-[#4a4a5a]">
          <p>Ensoul Admin Panel v1.0</p>
          <p>
            Admin authentication supports both API key (for scripts/CI) and
            username/password login (for this admin UI).
          </p>
        </div>
      </div>
    </div>
  );
}

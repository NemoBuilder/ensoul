"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { adminAuthApi, type AdminUser } from "@/lib/admin-api";
import "../globals.css";

// ── Sidebar navigation items ──────────────────────────────────

const navItems = [
  { href: "/admin/dashboard", label: "Dashboard", icon: "📊" },
  { href: "/admin/users", label: "Users", icon: "👥" },
  { href: "/admin/claws", label: "Claws", icon: "🦞" },
  { href: "/admin/candidates", label: "Candidates", icon: "🎯" },
  { href: "/admin/sniper-tags", label: "Sniper Tags", icon: "🏷️" },
  { href: "/admin/tax-wallet", label: "Tax Wallet", icon: "💰" },
  { href: "/admin/mining", label: "Mining Pool", icon: "⛏️" },
  { href: "/admin/settings", label: "Settings", icon: "⚙️" },
];

// ── Admin Layout ──────────────────────────────────────────────

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const [admin, setAdmin] = useState<AdminUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  // Check auth on mount
  const checkAuth = useCallback(async () => {
    try {
      const user = await adminAuthApi.me();
      setAdmin(user);
    } catch {
      // Not logged in → redirect to login (unless already there)
      if (pathname !== "/admin/login") {
        router.replace("/admin/login");
      }
    } finally {
      setLoading(false);
    }
  }, [pathname, router]);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // If on login page, render children directly (no sidebar)
  if (pathname === "/admin/login") {
    return (
      <html lang="en">
        <body className="antialiased" style={{ background: "#0a0a0f", color: "#e2e8f0" }}>
          {children}
        </body>
      </html>
    );
  }

  // Loading state
  if (loading) {
    return (
      <html lang="en">
        <body className="antialiased" style={{ background: "#0a0a0f", color: "#e2e8f0" }}>
          <div className="flex min-h-screen items-center justify-center">
            <div className="text-lg text-[#94a3b8]">Loading...</div>
          </div>
        </body>
      </html>
    );
  }

  // Not authenticated (redirect happening)
  if (!admin) {
    return (
      <html lang="en">
        <body className="antialiased" style={{ background: "#0a0a0f", color: "#e2e8f0" }}>
          <div className="flex min-h-screen items-center justify-center">
            <div className="text-lg text-[#94a3b8]">Redirecting to login...</div>
          </div>
        </body>
      </html>
    );
  }

  const handleLogout = async () => {
    try {
      await adminAuthApi.logout();
    } catch {
      // ignore
    }
    router.replace("/admin/login");
  };

  return (
    <html lang="en">
      <body className="antialiased" style={{ background: "#0a0a0f", color: "#e2e8f0" }}>
        <div className="flex min-h-screen">
          {/* Mobile overlay */}
          {sidebarOpen && (
            <div
              className="fixed inset-0 z-30 bg-black/50 lg:hidden"
              onClick={() => setSidebarOpen(false)}
            />
          )}

          {/* Sidebar */}
          <aside
            className={`fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-[#1e1e2e] bg-[#0d0d14] transition-transform lg:static lg:translate-x-0 ${
              sidebarOpen ? "translate-x-0" : "-translate-x-full"
            }`}
          >
            {/* Logo */}
            <div className="flex h-16 items-center gap-2 border-b border-[#1e1e2e] px-6">
              <span className="text-xl">🔮</span>
              <span className="text-lg font-bold text-[#8b5cf6]">Ensoul Admin</span>
            </div>

            {/* Nav */}
            <nav className="flex-1 overflow-y-auto p-4">
              <ul className="space-y-1">
                {navItems.map((item) => {
                  const isActive = pathname === item.href || pathname.startsWith(item.href + "/");
                  return (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        onClick={() => setSidebarOpen(false)}
                        className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                          isActive
                            ? "bg-[#8b5cf6]/10 text-[#8b5cf6]"
                            : "text-[#94a3b8] hover:bg-[#1e1e2e] hover:text-[#e2e8f0]"
                        }`}
                      >
                        <span className="text-base">{item.icon}</span>
                        <span>{item.label}</span>
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </nav>

            {/* User footer */}
            <div className="border-t border-[#1e1e2e] p-4">
              <div className="mb-2 flex items-center gap-2">
                <span className="text-sm">👤</span>
                <span className="text-sm font-medium text-[#e2e8f0]">
                  {admin.username}
                </span>
                <span className="rounded-full bg-[#8b5cf6]/20 px-2 py-0.5 text-xs text-[#a78bfa]">
                  {admin.role}
                </span>
              </div>
              <button
                onClick={handleLogout}
                className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm text-red-400 transition-colors hover:bg-[#1e1e2e]"
              >
                <span>🚪</span>
                <span>Logout</span>
              </button>
            </div>
          </aside>

          {/* Main content */}
          <div className="flex flex-1 flex-col">
            {/* Top bar (mobile menu button) */}
            <header className="flex h-16 items-center border-b border-[#1e1e2e] px-4 lg:px-6">
              <button
                onClick={() => setSidebarOpen(true)}
                className="mr-4 rounded-lg p-2 text-[#94a3b8] hover:bg-[#1e1e2e] lg:hidden"
              >
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>
              <h1 className="text-lg font-semibold text-[#e2e8f0]">
                {navItems.find((n) => pathname.startsWith(n.href))?.label || "Admin"}
              </h1>
            </header>

            {/* Page content */}
            <main className="flex-1 overflow-y-auto p-4 lg:p-6">
              {children}
            </main>
          </div>
        </div>
      </body>
    </html>
  );
}

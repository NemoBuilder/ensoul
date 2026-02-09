"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { ConnectButton } from "@rainbow-me/rainbowkit";

const navItems = [
  { href: "/explore", label: "Explore" },
  { href: "/mint", label: "Mint" },
  { href: "/claw", label: "Claws" },
  { href: "/leaderboard", label: "Leaderboard" },
];

export default function Navbar() {
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

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
    <nav className="fixed top-0 left-0 right-0 z-50 border-b border-[#1e1e2e] bg-[#0a0a0f]/80 backdrop-blur-md">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-2">
            <Image src="/logo.png" alt="Ensoul" width={52} height={52} className="rounded-md" />
            <span className="text-xl font-bold text-[#8b5cf6]">Ensoul</span>
          </Link>

          {/* Navigation links */}
          <div className="flex items-center gap-6">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={`text-sm font-medium transition-colors ${
                  pathname === item.href
                    ? "text-[#8b5cf6]"
                    : "text-[#94a3b8] hover:text-[#e2e8f0]"
                }`}
              >
                {item.label}
              </Link>
            ))}

            {/* Wallet button with user menu */}
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

                if (!mounted) {
                  return (
                    <div
                      aria-hidden="true"
                      style={{ opacity: 0, pointerEvents: "none", userSelect: "none" }}
                    />
                  );
                }

                if (!connected) {
                  return (
                    <button
                      onClick={openConnectModal}
                      className="rounded-lg bg-[#8b5cf6] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#a78bfa]"
                    >
                      Connect Wallet
                    </button>
                  );
                }

                if (chain.unsupported) {
                  return (
                    <button
                      onClick={openChainModal}
                      className="rounded-lg bg-red-500 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-red-400"
                    >
                      Wrong Network
                    </button>
                  );
                }

                return (
                  <div className="relative" ref={menuRef}>
                    <button
                      onClick={() => setMenuOpen(!menuOpen)}
                      className="flex items-center gap-2 rounded-lg border border-[#1e1e2e] bg-[#14141f] px-3 py-2 text-sm text-[#e2e8f0] transition-colors hover:border-[#8b5cf6]"
                    >
                      {chain.hasIcon && chain.iconUrl && (
                        <Image
                          src={chain.iconUrl}
                          alt={chain.name ?? "Chain"}
                          width={16}
                          height={16}
                          className="rounded-full"
                        />
                      )}
                      <span className="font-mono">
                        {account.displayName}
                      </span>
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

                    {/* Dropdown menu */}
                    {menuOpen && (
                      <div className="absolute right-0 mt-2 w-52 overflow-hidden rounded-lg border border-[#1e1e2e] bg-[#14141f] shadow-xl">
                        <Link
                          href="/my-souls"
                          className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                        >
                          <span>🧬</span>
                          <span>My Souls</span>
                        </Link>
                        <Link
                          href="/claw/dashboard"
                          className="flex items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                        >
                          <span>🦞</span>
                          <span>Claw Dashboard</span>
                        </Link>
                        <button
                          onClick={() => { setMenuOpen(false); openChainModal(); }}
                          className="flex w-full items-center gap-2 px-4 py-3 text-sm text-[#e2e8f0] transition-colors hover:bg-[#1e1e2e]"
                        >
                          <span>🔗</span>
                          <span>Switch Network</span>
                        </button>
                        <div className="border-t border-[#1e1e2e]" />
                        <button
                          onClick={() => { setMenuOpen(false); openAccountModal(); }}
                          className="flex w-full items-center gap-2 px-4 py-3 text-sm text-red-400 transition-colors hover:bg-[#1e1e2e]"
                        >
                          <span>🚪</span>
                          <span>Disconnect</span>
                        </button>
                      </div>
                    )}
                  </div>
                );
              }}
            </ConnectButton.Custom>
          </div>
        </div>
      </div>
    </nav>
  );
}

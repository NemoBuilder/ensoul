import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Ensoul — Souls aren't born. They're built.",
  description:
    "A decentralized protocol for soul construction. Mint a shell, contribute fragments, watch a soul emerge.",
  icons: {
    icon: "/logo.png",
    apple: "/logo.png",
  },
};

// Root layout is a minimal pass-through; the real layout lives in [locale]/layout.tsx
export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return children;
}

// V4 通用 BscScan 链接组件。链 ID 来自 NEXT_PUBLIC_CHAIN_ID（默认 56 mainnet）。
// 用于 tx hash / address / block number / token id 的统一外链。

import React from "react";

const CHAIN_ID = Number(process.env.NEXT_PUBLIC_CHAIN_ID || 56);

const BASE = CHAIN_ID === 97 ? "https://testnet.bscscan.com" : "https://bscscan.com";

type Kind = "tx" | "address" | "block" | "token" | "nft";

export function bscScanUrl(kind: Kind, value: string | number, tokenId?: string | number): string {
  switch (kind) {
    case "tx":
      return `${BASE}/tx/${value}`;
    case "address":
      return `${BASE}/address/${value}`;
    case "block":
      return `${BASE}/block/${value}`;
    case "token":
      return `${BASE}/token/${value}`;
    case "nft":
      return tokenId !== undefined
        ? `${BASE}/token/${value}?a=${tokenId}`
        : `${BASE}/token/${value}`;
  }
}

export function shortHash(s: string, head = 6, tail = 4): string {
  if (!s) return "";
  if (s.length <= head + tail + 2) return s;
  return `${s.slice(0, head)}…${s.slice(-tail)}`;
}

export default function BscScanLink({
  kind,
  value,
  tokenId,
  className,
  children,
}: {
  kind: Kind;
  value: string | number;
  tokenId?: string | number;
  className?: string;
  children?: React.ReactNode;
}) {
  if (!value) return null;
  return (
    <a
      href={bscScanUrl(kind, value, tokenId)}
      target="_blank"
      rel="noreferrer"
      className={
        className ||
        "font-mono text-xs text-[#06b6d4] hover:text-[#22d3ee] hover:underline"
      }
      title={String(value)}
    >
      {children ?? shortHash(String(value))}
    </a>
  );
}

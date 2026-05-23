// FairLaunch 合约 ABI — 只需要前端用到的 3 个用户函数 + 1 个读函数 +
// galaxyId 编码工具。
import { type Abi } from "viem";

export const FAIR_LAUNCH_ABI = [
  {
    type: "function",
    name: "deposit",
    stateMutability: "payable",
    inputs: [{ name: "gid", type: "bytes32" }],
    outputs: [],
  },
  {
    type: "function",
    name: "claim",
    stateMutability: "nonpayable",
    inputs: [{ name: "gid", type: "bytes32" }],
    outputs: [],
  },
  {
    type: "function",
    name: "refund",
    stateMutability: "nonpayable",
    inputs: [{ name: "gid", type: "bytes32" }],
    outputs: [],
  },
  {
    type: "function",
    name: "deposits",
    stateMutability: "view",
    inputs: [
      { name: "", type: "bytes32" },
      { name: "", type: "address" },
    ],
    outputs: [{ name: "", type: "uint256" }],
  },
] as const satisfies Abi;

// galaxyId UUID 编码 — 后端用 UUID 的 16 字节左对齐成 bytes32（高 16 字节是
// UUID，低 16 字节填 0）。这里把 "xxxxxxxx-xxxx-..." 转成 0x… 32字节。
export function galaxyIdToBytes32(galaxyUuid: string): `0x${string}` {
  const hex = galaxyUuid.replace(/-/g, "").toLowerCase();
  if (hex.length !== 32) {
    throw new Error(`bad galaxy uuid: ${galaxyUuid}`);
  }
  return ("0x" + hex + "0".repeat(32)) as `0x${string}`;
}

// FAIR_LAUNCH 合约地址 — 来自 NEXT_PUBLIC env。前端不强校验，合约调用失败
// 时由 wagmi 抛错；UI 层友好提示。
export function fairLaunchAddress(): `0x${string}` | undefined {
  const v = process.env.NEXT_PUBLIC_FAIR_LAUNCH_ADDR;
  if (!v || !/^0x[0-9a-fA-F]{40}$/.test(v)) return undefined;
  return v as `0x${string}`;
}

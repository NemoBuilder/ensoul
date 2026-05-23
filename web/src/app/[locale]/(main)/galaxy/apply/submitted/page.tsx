"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";

export default function SubmittedPage() {
  const params = useSearchParams();
  const id = params.get("id");
  return (
    <div className="mx-auto max-w-2xl px-4 pt-24 pb-16">
      <h1 className="mb-4 text-3xl font-bold text-[#e2e8f0]">Application submitted</h1>
      <p className="mb-2 text-[#94a3b8]">
        A curator will review your proposal. You&apos;ll be notified once it&apos;s approved.
      </p>
      {id && (
        <p className="mb-6 text-xs text-[#64748b]">
          Application ID: <span className="font-mono">{id}</span>
        </p>
      )}
      <Link
        href="../../galaxy"
        className="inline-block rounded-md border border-[#8b5cf6] bg-[#8b5cf6]/10 px-4 py-2 text-sm text-[#8b5cf6] hover:bg-[#8b5cf6]/20"
      >
        Back to galaxies
      </Link>
    </div>
  );
}

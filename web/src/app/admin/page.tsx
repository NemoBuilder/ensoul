"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function AdminIndexPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/admin/dashboard");
  }, [router]);

  return (
    <div className="flex min-h-[50vh] items-center justify-center text-[#94a3b8]">
      Redirecting to dashboard...
    </div>
  );
}

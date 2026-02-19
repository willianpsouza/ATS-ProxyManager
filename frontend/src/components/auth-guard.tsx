'use client';

import { useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/stores/auth-store';
import { api } from '@/lib/api';

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const token = useAuthStore((s) => s.token);
  const logout = useAuthStore((s) => s.logout);
  const beaconRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!isAuthenticated) {
      router.replace('/login');
      return;
    }

    beaconRef.current = setInterval(async () => {
      try {
        await api.auth.beacon();
      } catch {
        logout();
        router.replace('/login');
      }
    }, 25_000);

    return () => {
      if (beaconRef.current) clearInterval(beaconRef.current);
    };
  }, [isAuthenticated, token, router, logout]);

  if (!isAuthenticated) return null;

  return <>{children}</>;
}

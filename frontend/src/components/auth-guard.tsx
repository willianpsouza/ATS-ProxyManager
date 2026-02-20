'use client';

import { useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/stores/auth-store';
import { api } from '@/lib/api';

const BEACON_INTERVAL = 25_000; // 25s
const MAX_BEACON_FAILURES = 3;

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const logout = useAuthStore((s) => s.logout);
  const beaconRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const failCountRef = useRef(0);

  useEffect(() => {
    if (!isAuthenticated) {
      router.replace('/login');
      return;
    }

    beaconRef.current = setInterval(async () => {
      try {
        await api.auth.beacon();
        failCountRef.current = 0;
      } catch {
        failCountRef.current++;
        if (failCountRef.current >= MAX_BEACON_FAILURES) {
          logout();
          router.replace('/login');
        }
      }
    }, BEACON_INTERVAL);

    return () => {
      if (beaconRef.current) clearInterval(beaconRef.current);
    };
  }, [isAuthenticated, router, logout]);

  if (!isAuthenticated) return null;

  return <>{children}</>;
}

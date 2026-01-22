/**
 * Calendar store hooks - Simple singleton access to the calendar store
 */
import { useStore } from 'zustand';
import { CalendarStoreSingleton, CalendarStore } from './calendarStore';
import { ThemeMode } from '../types/calendarTypes';

/**
 * Hook to access the calendar store
 */
export function useCalendarStore() {
  return CalendarStoreSingleton.get();
}

/**
 * Hook to access calendar store with selector
 */
export function useCalendarStoreSelector<T>(selector: (state: CalendarStore) => T): T {
  const store = CalendarStoreSingleton.get();
  return useStore(store, selector);
}

/**
 * Theme hook interface
 */
export interface CalendarTheme {
  theme: ThemeMode;
  effectiveTheme: 'light' | 'dark';
  setTheme: (mode: ThemeMode) => void;
}

/**
 * Hook to access calendar theme
 */
export function useCalendarTheme(): CalendarTheme {
  const store = CalendarStoreSingleton.get();

  const theme = useStore(store, state => state.theme);
  const effectiveTheme = useStore(store, state => state.effectiveTheme);
  const setTheme = store.getState().setTheme;

  return { theme, effectiveTheme, setTheme };
}

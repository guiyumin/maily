# Calendar State Management Refactor: useCalendarApp → Zustand

## Summary

Replace the current `useCalendarApp` hook + `CalendarApp` class pattern with a Zustand store for better performance (selective subscriptions) and cleaner code.

## Current Problems

1. **Full re-renders**: Every state change triggers `forceUpdate()`, re-rendering entire tree
2. **Manual method wrapping**: 13+ methods wrapped in useEffect to trigger re-renders
3. **Duplicate state**: State exists in both CalendarApp class AND React useState hooks
4. **No granular subscriptions**: Components re-render even when their data didn't change

## Solution: Zustand Store Factory

### New Files to Create

**1. `/src/lib/calendar/core/calendarStore.ts`** - Main store
```typescript
import { create, StoreApi, UseBoundStore } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

interface CalendarStoreState {
  // Reactive state
  currentView: ViewType;
  currentDate: Date;
  events: Event[];
  highlightedEventId: string | null;
  visibleMonth: Date;
  switcherMode: ViewSwitcherMode;
  locale: string | Locale;

  // Internal (non-reactive direct access)
  _calendarRegistry: CalendarRegistry;
  _callbacks: CalendarCallbacks;
  _views: Map<ViewType, CalendarView>;
  _plugins: Map<string, CalendarPlugin>;
  _sidebarConfig: SidebarConfig;
}

interface CalendarStoreActions {
  // View
  changeView: (view: ViewType) => void;
  getCurrentView: () => CalendarView;

  // Date navigation
  setCurrentDate: (date: Date) => void;
  goToToday: () => void;
  goToPrevious: () => void;
  goToNext: () => void;

  // Events
  addEvent: (event: Event) => void;
  updateEvent: (id: string, updates: Partial<Event>) => void;
  deleteEvent: (id: string) => void;
  setEvents: (events: Event[]) => void;
  highlightEvent: (eventId: string | null) => void;

  // Calendars
  getCalendars: () => CalendarType[];
  setCalendars: (calendars: CalendarType[]) => void;
  setCalendarVisibility: (id: string, visible: boolean) => void;
  // ... other calendar methods
}

export function createCalendarStore(config: CalendarAppConfig) {
  return create<CalendarStoreState & CalendarStoreActions>()(
    subscribeWithSelector((set, get) => ({
      // Initial state from config
      currentView: config.defaultView || ViewType.WEEK,
      currentDate: config.initialDate || new Date(),
      events: config.events || [],
      // ... etc

      // Actions
      changeView: (view) => set({ currentView: view }),
      setCurrentDate: (date) => set({ currentDate: new Date(date) }),
      // ... etc
    }))
  );
}
```

**2. `/src/lib/calendar/core/calendarSelectors.ts`** - Selectors for derived state
```typescript
// Only re-computes when events or calendar visibility changes
export const selectVisibleEvents = (state: CalendarStore) => {
  const visible = new Set(state._calendarRegistry.getVisible().map(c => c.id));
  return state.events.filter(e => !e.calendarId || visible.has(e.calendarId));
};
```

### Files to Modify

**3. `/src/lib/calendar/core/useCalendarApp.ts`** - Refactor to use store
```typescript
export function useCalendarApp(config: CalendarAppConfig): UseCalendarAppReturn {
  const store = useMemo(() => createCalendarStore(config), []);

  // Selective subscriptions - only re-render when specific state changes
  const currentView = useStore(store, s => s.currentView);
  const currentDate = useStore(store, s => s.currentDate);
  const events = useStore(store, selectVisibleEvents);

  // Create backward-compatible app object
  const app = useMemo(() => createAppProxy(store), [store]);

  return { app, currentView, currentDate, events, /* ... */ };
}
```

**4. `/src/lib/calendar/core/CalendarApp.ts`** - Can be deprecated or kept as thin wrapper

## Migration Strategy (Incremental)

### Phase 1: Create Store (non-breaking)
- [ ] Create `calendarStore.ts` with full implementation
- [ ] Create `calendarSelectors.ts`
- [ ] Add Zustand dependencies if needed

### Phase 2: Compatibility Layer
- [ ] Refactor `useCalendarApp.ts` to use store internally
- [ ] Maintain exact same `UseCalendarAppReturn` interface
- [ ] Test that existing components work unchanged

### Phase 3: Optimize Components (optional)
- [ ] Update view components to use direct store subscriptions
- [ ] Remove unnecessary re-renders

## Key Benefits

1. **Selective subscriptions**: Components only re-render when their specific slice changes
2. **Backward compatible**: Same public API, existing code works unchanged
3. **Store factory**: Supports multiple calendar instances
4. **Cleaner code**: Remove 100+ lines of method wrapping boilerplate
5. **Better devtools**: Zustand devtools for debugging

## Files Summary

| File | Action |
|------|--------|
| `core/calendarStore.ts` | CREATE |
| `core/calendarSelectors.ts` | CREATE |
| `core/useCalendarApp.ts` | MODIFY (major refactor) |
| `core/CalendarApp.ts` | MODIFY (optional - deprecate or thin wrapper) |
| `core/index.ts` | MODIFY (export new store) |

## Verification

1. Run existing calendar tests (if any)
2. Manual test in Tauri app:
   - Navigate to calendar page
   - Switch between views (month/week/day)
   - Create/update/delete events
   - Toggle calendar visibility in sidebar
   - Verify no regressions in functionality
3. Check React DevTools for reduced re-renders

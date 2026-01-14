// EventKit FFI for Rust
// Provides calendar integration on macOS via EventKit framework

#import <Foundation/Foundation.h>
#import <EventKit/EventKit.h>
#import <AppKit/AppKit.h>

// Result codes
#define EK_SUCCESS 0
#define EK_ERROR_ACCESS_DENIED 1
#define EK_ERROR_NOT_FOUND 2
#define EK_ERROR_FAILED 3

// Authorization status codes
#define EK_AUTH_NOT_DETERMINED 0
#define EK_AUTH_RESTRICTED 1
#define EK_AUTH_DENIED 2
#define EK_AUTH_AUTHORIZED 3

// Shared event store instance
static EKEventStore *sharedEventStore = nil;
static BOOL accessGranted = NO;

// Helper: Get or create shared event store
static EKEventStore* getEventStore(void) {
    if (sharedEventStore == nil) {
        sharedEventStore = [[EKEventStore alloc] init];
    }
    return sharedEventStore;
}

// Helper: Copy NSString to malloc'd C string (caller must free)
static char* copyString(NSString *str) {
    if (str == nil) return NULL;
    const char *utf8 = [str UTF8String];
    if (utf8 == NULL) return NULL;
    size_t len = strlen(utf8) + 1;
    char *copy = (char*)malloc(len);
    if (copy) {
        memcpy(copy, utf8, len);
    }
    return copy;
}

// Helper: Convert CGColor to hex string
static NSString* colorToHex(CGColorRef cgColor) {
    if (cgColor == NULL) return @"#808080";

    NSColorSpace *srgbSpace = [NSColorSpace sRGBColorSpace];
    NSColor *color = [NSColor colorWithCGColor:cgColor];
    NSColor *srgbColor = [color colorUsingColorSpace:srgbSpace];

    if (srgbColor == nil) return @"#808080";

    CGFloat r, g, b, a;
    [srgbColor getRed:&r green:&g blue:&b alpha:&a];

    return [NSString stringWithFormat:@"#%02X%02X%02X",
            (int)(r * 255), (int)(g * 255), (int)(b * 255)];
}

// Get current authorization status without prompting
int ek_get_auth_status(void) {
    @autoreleasepool {
        EKAuthorizationStatus status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];

        switch (status) {
            case EKAuthorizationStatusNotDetermined:
                return EK_AUTH_NOT_DETERMINED;
            case EKAuthorizationStatusRestricted:
                return EK_AUTH_RESTRICTED;
            case EKAuthorizationStatusDenied:
                return EK_AUTH_DENIED;
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
            case EKAuthorizationStatusFullAccess:
                return EK_AUTH_AUTHORIZED;
            case EKAuthorizationStatusWriteOnly:
                return EK_AUTH_AUTHORIZED;
#else
            case EKAuthorizationStatusAuthorized:
                return EK_AUTH_AUTHORIZED;
#endif
            default:
                return EK_AUTH_DENIED;
        }
    }
}

// Request calendar access (blocks until user responds)
int ek_request_access(void) {
    @autoreleasepool {
        // Check if already authorized
        EKAuthorizationStatus status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
        if (status == EKAuthorizationStatusFullAccess) {
            accessGranted = YES;
            return EK_SUCCESS;
        }
#else
        if (status == EKAuthorizationStatusAuthorized) {
            accessGranted = YES;
            return EK_SUCCESS;
        }
#endif

        if (status == EKAuthorizationStatusDenied || status == EKAuthorizationStatusRestricted) {
            return EK_ERROR_ACCESS_DENIED;
        }

        EKEventStore *store = getEventStore();
        __block BOOL granted = NO;
        __block NSError *error = nil;

        dispatch_semaphore_t sem = dispatch_semaphore_create(0);

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 140000
        if (@available(macOS 14.0, *)) {
            [store requestFullAccessToEventsWithCompletion:^(BOOL g, NSError *e) {
                granted = g;
                error = e;
                dispatch_semaphore_signal(sem);
            }];
        } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            [store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL g, NSError *e) {
                granted = g;
                error = e;
                dispatch_semaphore_signal(sem);
            }];
#pragma clang diagnostic pop
        }
#else
        [store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL g, NSError *e) {
            granted = g;
            error = e;
            dispatch_semaphore_signal(sem);
        }];
#endif

        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

        if (!granted) {
            return EK_ERROR_ACCESS_DENIED;
        }

        // Warm-up: EventStore may need a moment after permission grant
        for (int i = 0; i < 3; i++) {
            NSArray *calendars = [store calendarsForEntityType:EKEntityTypeEvent];
            if (calendars != nil && calendars.count > 0) {
                accessGranted = YES;
                return EK_SUCCESS;
            }
            [NSThread sleepForTimeInterval:0.3];
        }

        accessGranted = YES;
        return EK_SUCCESS;
    }
}

// List all calendars (returns JSON string, caller must free)
char* ek_list_calendars(int *result_code) {
    @autoreleasepool {
        if (!accessGranted) {
            *result_code = EK_ERROR_ACCESS_DENIED;
            return NULL;
        }

        EKEventStore *store = getEventStore();
        NSArray<EKCalendar *> *calendars = [store calendarsForEntityType:EKEntityTypeEvent];

        if (calendars == nil) {
            *result_code = EK_ERROR_FAILED;
            return NULL;
        }

        NSMutableArray *jsonArray = [NSMutableArray array];

        for (EKCalendar *cal in calendars) {
            NSString *colorHex = colorToHex(cal.CGColor);

            NSDictionary *dict = @{
                @"id": cal.calendarIdentifier ?: @"",
                @"title": cal.title ?: @"",
                @"color": colorHex
            };
            [jsonArray addObject:dict];
        }

        NSError *jsonError = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:jsonArray
                                                          options:0
                                                            error:&jsonError];

        if (jsonError != nil || jsonData == nil) {
            *result_code = EK_ERROR_FAILED;
            return NULL;
        }

        NSString *jsonString = [[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding];
        *result_code = EK_SUCCESS;
        return copyString(jsonString);
    }
}

// List events in date range (returns JSON string, caller must free)
char* ek_list_events(double start_timestamp, double end_timestamp, int *result_code) {
    @autoreleasepool {
        if (!accessGranted) {
            *result_code = EK_ERROR_ACCESS_DENIED;
            return NULL;
        }

        EKEventStore *store = getEventStore();

        NSDate *startDate = [NSDate dateWithTimeIntervalSince1970:start_timestamp];
        NSDate *endDate = [NSDate dateWithTimeIntervalSince1970:end_timestamp];

        NSPredicate *predicate = [store predicateForEventsWithStartDate:startDate
                                                                endDate:endDate
                                                              calendars:nil];

        NSArray<EKEvent *> *events = [store eventsMatchingPredicate:predicate];

        if (events == nil) {
            *result_code = EK_ERROR_FAILED;
            return NULL;
        }

        NSMutableArray *jsonArray = [NSMutableArray array];

        for (EKEvent *event in events) {
            // Get alarm minutes before (first alarm only)
            NSInteger alarmMinutes = 0;
            if (event.alarms.count > 0) {
                EKAlarm *alarm = event.alarms[0];
                alarmMinutes = (NSInteger)(-alarm.relativeOffset / 60);
            }

            NSDictionary *dict = @{
                @"id": event.eventIdentifier ?: @"",
                @"title": event.title ?: @"",
                @"start_time": @([event.startDate timeIntervalSince1970]),
                @"end_time": @([event.endDate timeIntervalSince1970]),
                @"location": event.location ?: @"",
                @"notes": event.notes ?: @"",
                @"calendar": event.calendar.calendarIdentifier ?: @"",
                @"all_day": @(event.allDay),
                @"alarm_minutes_before": @(alarmMinutes)
            };
            [jsonArray addObject:dict];
        }

        NSError *jsonError = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:jsonArray
                                                          options:0
                                                            error:&jsonError];

        if (jsonError != nil || jsonData == nil) {
            *result_code = EK_ERROR_FAILED;
            return NULL;
        }

        NSString *jsonString = [[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding];
        *result_code = EK_SUCCESS;
        return copyString(jsonString);
    }
}

// Create event (returns event ID, caller must free)
char* ek_create_event(
    const char *title,
    double start_timestamp,
    double end_timestamp,
    const char *location,
    const char *notes,
    const char *calendar_id,
    int all_day,
    int alarm_minutes_before,
    int *result_code
) {
    @autoreleasepool {
        if (!accessGranted) {
            *result_code = EK_ERROR_ACCESS_DENIED;
            return NULL;
        }

        EKEventStore *store = getEventStore();
        EKEvent *event = [EKEvent eventWithEventStore:store];

        event.title = title ? [NSString stringWithUTF8String:title] : @"";
        event.startDate = [NSDate dateWithTimeIntervalSince1970:start_timestamp];
        event.endDate = [NSDate dateWithTimeIntervalSince1970:end_timestamp];
        event.allDay = (all_day != 0);

        if (location && strlen(location) > 0) {
            event.location = [NSString stringWithUTF8String:location];
        }

        if (notes && strlen(notes) > 0) {
            event.notes = [NSString stringWithUTF8String:notes];
        }

        // Set calendar
        if (calendar_id && strlen(calendar_id) > 0) {
            NSString *calId = [NSString stringWithUTF8String:calendar_id];
            EKCalendar *cal = [store calendarWithIdentifier:calId];
            if (cal != nil) {
                event.calendar = cal;
            } else {
                event.calendar = [store defaultCalendarForNewEvents];
            }
        } else {
            event.calendar = [store defaultCalendarForNewEvents];
        }

        // Add alarm if specified
        if (alarm_minutes_before > 0) {
            EKAlarm *alarm = [EKAlarm alarmWithRelativeOffset:-(alarm_minutes_before * 60)];
            [event addAlarm:alarm];
        }

        NSError *error = nil;
        BOOL success = [store saveEvent:event span:EKSpanThisEvent commit:YES error:&error];

        if (!success || error != nil) {
            *result_code = EK_ERROR_FAILED;
            return NULL;
        }

        *result_code = EK_SUCCESS;
        return copyString(event.eventIdentifier);
    }
}

// Delete event by ID
int ek_delete_event(const char *event_id) {
    @autoreleasepool {
        if (!accessGranted) {
            return EK_ERROR_ACCESS_DENIED;
        }

        if (event_id == NULL || strlen(event_id) == 0) {
            return EK_ERROR_NOT_FOUND;
        }

        EKEventStore *store = getEventStore();
        NSString *eventIdStr = [NSString stringWithUTF8String:event_id];
        EKEvent *event = [store eventWithIdentifier:eventIdStr];

        if (event == nil) {
            return EK_ERROR_NOT_FOUND;
        }

        NSError *error = nil;
        BOOL success = [store removeEvent:event span:EKSpanThisEvent commit:YES error:&error];

        if (!success || error != nil) {
            return EK_ERROR_FAILED;
        }

        return EK_SUCCESS;
    }
}

// Get default calendar ID (caller must free)
char* ek_get_default_calendar(int *result_code) {
    @autoreleasepool {
        if (!accessGranted) {
            *result_code = EK_ERROR_ACCESS_DENIED;
            return NULL;
        }

        EKEventStore *store = getEventStore();
        EKCalendar *cal = [store defaultCalendarForNewEvents];

        if (cal == nil) {
            *result_code = EK_ERROR_NOT_FOUND;
            return NULL;
        }

        *result_code = EK_SUCCESS;
        return copyString(cal.calendarIdentifier);
    }
}

// Free a string returned by other functions
void ek_free_string(char *str) {
    if (str != NULL) {
        free(str);
    }
}

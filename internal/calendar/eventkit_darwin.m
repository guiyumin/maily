#import <Foundation/Foundation.h>
#import <EventKit/EventKit.h>
#import <CoreGraphics/CoreGraphics.h>
#include "eventkit_darwin.h"
#include <stdlib.h>
#include <string.h>

// Shared event store instance
static EKEventStore *sharedEventStore = nil;
static BOOL accessGranted = NO;

static EKEventStore* getEventStore(void) {
    if (sharedEventStore == nil) {
        sharedEventStore = [[EKEventStore alloc] init];
    }
    return sharedEventStore;
}

static char* copyString(NSString *str) {
    if (str == nil) return NULL;
    const char *cstr = [str UTF8String];
    char *copy = (char*)malloc(strlen(cstr) + 1);
    strcpy(copy, cstr);
    return copy;
}

static NSString* colorToHex(CGColorRef cgColor) {
    if (cgColor == NULL) return @"#808080";

    size_t numComponents = CGColorGetNumberOfComponents(cgColor);
    const CGFloat *components = CGColorGetComponents(cgColor);

    if (components == NULL || numComponents < 3) {
        return @"#808080";
    }

    int r = (int)(components[0] * 255);
    int g = (int)(components[1] * 255);
    int b = (int)(components[2] * 255);

    return [NSString stringWithFormat:@"#%02X%02X%02X", r, g, b];
}

int GetAuthorizationStatus(void) {
    @autoreleasepool {
        EKAuthorizationStatus status;

        if (@available(macOS 14.0, *)) {
            status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];
            // On macOS 14+, check for full access
            if (status == EKAuthorizationStatusFullAccess) {
                return EK_AUTH_AUTHORIZED;
            }
        } else {
            status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeEvent];
        }

        switch (status) {
            case EKAuthorizationStatusNotDetermined:
                return EK_AUTH_NOT_DETERMINED;
            case EKAuthorizationStatusRestricted:
                return EK_AUTH_RESTRICTED;
            case EKAuthorizationStatusDenied:
                return EK_AUTH_DENIED;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            case EKAuthorizationStatusAuthorized:
                return EK_AUTH_AUTHORIZED;
#pragma clang diagnostic pop
            default:
                return EK_AUTH_DENIED;
        }
    }
}

int RequestCalendarAccess(void) {
    @autoreleasepool {
        int currentStatus = GetAuthorizationStatus();
        if (currentStatus == EK_AUTH_DENIED || currentStatus == EK_AUTH_RESTRICTED) {
            accessGranted = NO;
            return EK_ERROR_ACCESS_DENIED;
        }

        EKEventStore *store = getEventStore();

        // If not yet determined, request permission first
        if (currentStatus == EK_AUTH_NOT_DETERMINED) {
            dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
            __block BOOL granted = NO;

            if (@available(macOS 14.0, *)) {
                [store requestFullAccessToEventsWithCompletion:^(BOOL success, NSError *error) {
                    granted = success;
                    dispatch_semaphore_signal(semaphore);
                }];
            } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
                [store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL success, NSError *error) {
                    granted = success;
                    dispatch_semaphore_signal(semaphore);
                }];
#pragma clang diagnostic pop
            }

            dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);

            if (!granted) {
                accessGranted = NO;
                return EK_ERROR_ACCESS_DENIED;
            }
        }

        // Warm up EventStore with retry - handles transient failures after
        // permission grant or when accessing with a new binary after upgrade
        for (int i = 0; i < 3; i++) {
            NSArray<EKCalendar *> *calendars = [store calendarsForEntityType:EKEntityTypeEvent];
            if (calendars != nil) {
                accessGranted = YES;
                return EK_SUCCESS;
            }
            if (i < 2) {
                [NSThread sleepForTimeInterval:0.3];
            }
        }

        // Warm-up failed after retries
        accessGranted = NO;
        return EK_ERROR_ACCESS_DENIED;
    }
}

char* ListCalendars(void) {
    @autoreleasepool {
        if (!accessGranted) {
            if (RequestCalendarAccess() != EK_SUCCESS) {
                return copyString(@"[]");
            }
        }

        EKEventStore *store = getEventStore();
        NSArray<EKCalendar *> *calendars = [store calendarsForEntityType:EKEntityTypeEvent];

        NSMutableArray *calendarDicts = [NSMutableArray array];
        for (EKCalendar *cal in calendars) {
            NSDictionary *dict = @{
                @"id": cal.calendarIdentifier ?: @"",
                @"title": cal.title ?: @"",
                @"color": colorToHex(cal.CGColor)
            };
            [calendarDicts addObject:dict];
        }

        NSError *error = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:calendarDicts options:0 error:&error];
        if (error != nil) {
            return copyString(@"[]");
        }

        NSString *jsonString = [[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding];
        return copyString(jsonString);
    }
}

char* ListEvents(long long startTimestamp, long long endTimestamp) {
    @autoreleasepool {
        if (!accessGranted) {
            if (RequestCalendarAccess() != EK_SUCCESS) {
                return copyString(@"[]");
            }
        }

        EKEventStore *store = getEventStore();

        NSDate *startDate = [NSDate dateWithTimeIntervalSince1970:(NSTimeInterval)startTimestamp];
        NSDate *endDate = [NSDate dateWithTimeIntervalSince1970:(NSTimeInterval)endTimestamp];

        NSArray<EKCalendar *> *calendars = [store calendarsForEntityType:EKEntityTypeEvent];
        NSPredicate *predicate = [store predicateForEventsWithStartDate:startDate endDate:endDate calendars:calendars];
        NSArray<EKEvent *> *events = [store eventsMatchingPredicate:predicate];

        NSMutableArray *eventDicts = [NSMutableArray array];
        for (EKEvent *event in events) {
            NSDictionary *dict = @{
                @"id": event.eventIdentifier ?: @"",
                @"title": event.title ?: @"",
                @"startTime": @((long long)[event.startDate timeIntervalSince1970]),
                @"endTime": @((long long)[event.endDate timeIntervalSince1970]),
                @"location": event.location ?: @"",
                @"notes": event.notes ?: @"",
                @"calendar": event.calendar.title ?: @"",
                @"calendarID": event.calendar.calendarIdentifier ?: @"",
                @"allDay": @(event.allDay)
            };
            [eventDicts addObject:dict];
        }

        // Sort by start time
        [eventDicts sortUsingComparator:^NSComparisonResult(NSDictionary *a, NSDictionary *b) {
            return [a[@"startTime"] compare:b[@"startTime"]];
        }];

        NSError *error = nil;
        NSData *jsonData = [NSJSONSerialization dataWithJSONObject:eventDicts options:0 error:&error];
        if (error != nil) {
            return copyString(@"[]");
        }

        NSString *jsonString = [[NSString alloc] initWithData:jsonData encoding:NSUTF8StringEncoding];
        return copyString(jsonString);
    }
}

char* CreateEvent(const char* title, long long startTimestamp, long long endTimestamp,
                  const char* calendarID, const char* location, const char* notes, int allDay,
                  int alarmMinutesBefore) {
    @autoreleasepool {
        if (!accessGranted) {
            if (RequestCalendarAccess() != EK_SUCCESS) {
                return NULL;
            }
        }

        EKEventStore *store = getEventStore();
        EKEvent *event = [EKEvent eventWithEventStore:store];

        event.title = title ? [NSString stringWithUTF8String:title] : @"";
        event.startDate = [NSDate dateWithTimeIntervalSince1970:(NSTimeInterval)startTimestamp];
        event.endDate = [NSDate dateWithTimeIntervalSince1970:(NSTimeInterval)endTimestamp];
        event.allDay = (allDay != 0);

        if (location != NULL && strlen(location) > 0) {
            event.location = [NSString stringWithUTF8String:location];
        }

        if (notes != NULL && strlen(notes) > 0) {
            event.notes = [NSString stringWithUTF8String:notes];
        }

        // Add alarm if specified (negative offset = before event)
        if (alarmMinutesBefore > 0) {
            EKAlarm *alarm = [EKAlarm alarmWithRelativeOffset:-alarmMinutesBefore * 60];
            [event addAlarm:alarm];
        }

        // Find the calendar by ID, or use default
        EKCalendar *calendar = nil;
        if (calendarID != NULL && strlen(calendarID) > 0) {
            NSString *calID = [NSString stringWithUTF8String:calendarID];
            calendar = [store calendarWithIdentifier:calID];
        }

        if (calendar == nil) {
            calendar = [store defaultCalendarForNewEvents];
        }

        if (calendar == nil) {
            return NULL;
        }

        event.calendar = calendar;

        NSError *error = nil;
        BOOL success = [store saveEvent:event span:EKSpanThisEvent commit:YES error:&error];

        if (!success || error != nil) {
            return NULL;
        }

        return copyString(event.eventIdentifier);
    }
}

int DeleteEvent(const char* eventID) {
    @autoreleasepool {
        if (!accessGranted) {
            if (RequestCalendarAccess() != EK_SUCCESS) {
                return EK_ERROR_ACCESS_DENIED;
            }
        }

        if (eventID == NULL) {
            return EK_ERROR_NOT_FOUND;
        }

        EKEventStore *store = getEventStore();
        NSString *eventIDStr = [NSString stringWithUTF8String:eventID];
        EKEvent *event = [store eventWithIdentifier:eventIDStr];

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

void FreeString(char* str) {
    if (str != NULL) {
        free(str);
    }
}

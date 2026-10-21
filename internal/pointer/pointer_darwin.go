//go:build darwin && cgo

package pointer

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics

#import <Cocoa/Cocoa.h>

@interface LaserView : NSView
@property (assign) BOOL active;
@property (assign) CGFloat pointX;
@property (assign) CGFloat pointY;
@property (assign) int mode; // 0 = laser, 1 = spotlight
@end

@implementation LaserView
- (void)drawRect:(NSRect)dirtyRect {
    if (!self.active) return;
    NSRect bounds = [self bounds];
    if (bounds.size.width <= 0 || bounds.size.height <= 0) return;

    // Handle 16:9 widescreen presentation area within potentially different screen aspect ratios
    CGFloat slideAspect = 16.0 / 9.0;
    CGFloat screenAspect = bounds.size.width / bounds.size.height;
    CGFloat slideW, slideH, offX, offY;

    if (screenAspect > slideAspect) {
        // Pillarboxed (screen is wider than 16:9, e.g. ultrawide)
        slideH = bounds.size.height;
        slideW = slideH * slideAspect;
        offX = (bounds.size.width - slideW) / 2.0;
        offY = 0;
    } else {
        // Letterboxed (screen is taller than 16:9, e.g. 16:10 or 4:3)
        slideW = bounds.size.width;
        slideH = slideW / slideAspect;
        offX = 0;
        offY = (bounds.size.height - slideH) / 2.0;
    }

    CGFloat cx = offX + (self.pointX * slideW);
    CGFloat cy = offY + ((1.0 - self.pointY) * slideH);

    if (self.mode == 1) { // Spotlight mode
        [[NSColor colorWithCalibratedWhite:0.0 alpha:0.65] set];
        NSRectFillUsingOperation(bounds, NSCompositingOperationSourceOver);

        CGFloat r = 120.0;
        NSRect spotRect = NSMakeRect(cx - r, cy - r, r * 2.0, r * 2.0);
        NSBezierPath *circle = [NSBezierPath bezierPathWithOvalInRect:spotRect];
        CGContextRef ctx = [[NSGraphicsContext currentContext] CGContext];
        CGContextSaveGState(ctx);
        CGContextSetBlendMode(ctx, kCGBlendModeClear);
        [[NSColor clearColor] set];
        [circle fill];
        CGContextRestoreGState(ctx);
    } else { // Laser pointer mode (glowing red dot)
        CGFloat r = 12.0;
        NSRect dotRect = NSMakeRect(cx - r, cy - r, r * 2.0, r * 2.0);

        // Outer glow
        NSBezierPath *glow = [NSBezierPath bezierPathWithOvalInRect:NSInsetRect(dotRect, -6, -6)];
        [[NSColor colorWithCalibratedRed:1.0 green:0.2 blue:0.2 alpha:0.45] set];
        [glow fill];

        // Core bright dot
        NSBezierPath *dot = [NSBezierPath bezierPathWithOvalInRect:dotRect];
        [[NSColor colorWithCalibratedRed:1.0 green:0.0 blue:0.1 alpha:0.95] set];
        [dot fill];

        // Center specular glint
        NSBezierPath *glint = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(cx - 2.5, cy + 2.5, 5, 5)];
        [[NSColor colorWithCalibratedWhite:1.0 alpha:0.9] set];
        [glint fill];
    }
}
@end

static NSWindow *overlayWindow = nil;
static LaserView *overlayView = nil;
static NSScreen *currentScreen = nil;
static NSTimeInterval lastScreenQueryTime = 0.0;
static NSScreen *cachedTargetScreen = nil;

// querySlideShowScreen locates the presentation display when multiple screens are connected.
// It directly queries PowerPoint via AppleScript for the exact bounds of 'slide show window 1'.
// This strictly respects the user's configured presentation display and Presenter View settings
// without modifying any PowerPoint or system configurations.
static NSScreen *querySlideShowScreen(void) {
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if ([screens count] <= 1) {
        return [screens firstObject];
    }

    @autoreleasepool {
        static NSAppleScript *script = nil;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            script = [[NSAppleScript alloc] initWithSource:
                @"if application \"Microsoft PowerPoint\" is running then\n"
                @"  tell application \"Microsoft PowerPoint\"\n"
                @"    if (count of slide show windows) > 0 then\n"
                @"      set b to bounds of slide show window 1\n"
                @"      return (item 1 of b as string) & \",\" & (item 2 of b as string) & \",\" & (item 3 of b as string) & \",\" & (item 4 of b as string)\n"
                @"    end if\n"
                @"  end tell\n"
                @"end if\n"
                @"return \"\""
            ];
            [script compileAndReturnError:nil];
        });

        if (script != nil) {
            NSDictionary *err = nil;
            NSAppleEventDescriptor *desc = [script executeAndReturnError:&err];
            if (desc && [desc stringValue]) {
                NSString *str = [desc stringValue];
                if ([str length] > 0) {
                    NSArray *parts = [str componentsSeparatedByString:@","];
                    if ([parts count] == 4) {
                        CGFloat left = [[parts objectAtIndex:0] doubleValue];
                        CGFloat top = [[parts objectAtIndex:1] doubleValue];
                        CGFloat right = [[parts objectAtIndex:2] doubleValue];
                        CGFloat bottom = [[parts objectAtIndex:3] doubleValue];

                        CGFloat midX = (left + right) / 2.0;
                        CGFloat midY = (top + bottom) / 2.0;

                        // QuickDraw (top-left) to Cocoa (bottom-left of primary screen screens[0]) coordinate conversion:
                        CGFloat primaryH = screens[0].frame.size.height;
                        NSPoint cocoaMid = NSMakePoint(midX, primaryH - midY);

                        for (NSScreen *s in screens) {
                            if (NSPointInRect(cocoaMid, [s frame])) {
                                return s;
                            }
                        }
                    }
                }
            }
        }
    }

    // Fallback if no slide show window is detected: use mainScreen or primary screen
    NSScreen *main = [NSScreen mainScreen];
    if (main != nil) {
        return main;
    }
    return screens[0];
}

static NSScreen *getSlideShowScreen(BOOL forceRefresh) {
    if (cachedTargetScreen != nil && ![[NSScreen screens] containsObject:cachedTargetScreen]) {
        cachedTargetScreen = nil;
    }

    NSTimeInterval now = [NSDate timeIntervalSinceReferenceDate];
    if (forceRefresh || cachedTargetScreen == nil || (now - lastScreenQueryTime > 1.5)) {
        cachedTargetScreen = querySlideShowScreen();
        lastScreenQueryTime = now;
    }
    return cachedTargetScreen;
}

static void ensureWindowOnTargetScreen(NSScreen *targetScreen) {
    if (targetScreen == nil) {
        targetScreen = [NSScreen mainScreen];
    }
    NSRect screenRect = [targetScreen frame];

    if (overlayWindow == nil) {
        overlayWindow = [[NSWindow alloc] initWithContentRect:screenRect
                                                    styleMask:NSWindowStyleMaskBorderless
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO];
        [overlayWindow setOpaque:NO];
        [overlayWindow setBackgroundColor:[NSColor clearColor]];
        [overlayWindow setLevel:NSScreenSaverWindowLevel];
        [overlayWindow setIgnoresMouseEvents:YES];
        [overlayWindow setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary];

        overlayView = [[LaserView alloc] initWithFrame:NSMakeRect(0, 0, screenRect.size.width, screenRect.size.height)];
        [overlayWindow setContentView:overlayView];
        [overlayWindow orderFrontRegardless];
        currentScreen = targetScreen;
    } else {
        if (currentScreen != targetScreen || !NSEqualRects([overlayWindow frame], screenRect)) {
            [overlayWindow setFrame:screenRect display:YES];
            [overlayView setFrame:NSMakeRect(0, 0, screenRect.size.width, screenRect.size.height)];
            [overlayWindow orderFrontRegardless];
            currentScreen = targetScreen;
        }
    }
}

static void updatePointerNative(int active, double x, double y, int mode) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (active == 0) {
            if (overlayView != nil && overlayView.active) {
                overlayView.active = NO;
                [overlayView setNeedsDisplay:YES];
            }
            lastScreenQueryTime = 0.0;
            return;
        }

        BOOL isStarting = (overlayView == nil || !overlayView.active);
        NSScreen *targetScreen = getSlideShowScreen(isStarting);
        ensureWindowOnTargetScreen(targetScreen);
        if (overlayView == nil) return;

        overlayView.active = YES;
        overlayView.pointX = (CGFloat)x;
        overlayView.pointY = (CGFloat)y;
        overlayView.mode = mode;
        [overlayView setNeedsDisplay:YES];
    });
}

static void closePointerNative(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (overlayWindow != nil) {
            [overlayWindow orderOut:nil];
            overlayWindow = nil;
            overlayView = nil;
            currentScreen = nil;
            cachedTargetScreen = nil;
            lastScreenQueryTime = 0.0;
        }
    });
}
*/
import "C"

type darwinController struct{}

func newPlatform() Controller {
	return &darwinController{}
}

func (d *darwinController) Update(active bool, x, y float64, mode string) {
	cActive := C.int(0)
	if active {
		cActive = C.int(1)
	}
	cMode := C.int(0)
	if mode == "spotlight" {
		cMode = C.int(1)
	}
	C.updatePointerNative(cActive, C.double(x), C.double(y), cMode)
}

func (d *darwinController) Close() {
	C.closePointerNative()
}

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

// findSlideShowScreen locates the presentation display when multiple screens are connected.
// If PowerPoint has a Slide Show window running, it returns the screen containing that window.
// Otherwise, on multi-monitor setups, it targets the secondary display (projector/external screen)
// instead of the presenter's laptop screen.
static NSScreen *findSlideShowScreen(void) {
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if ([screens count] <= 1) {
        return [screens firstObject];
    }

    // 1. Look for PowerPoint's full-screen Slide Show window across on-screen windows
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (windowList) {
        NSArray *windows = (__bridge NSArray *)windowList;
        for (NSDictionary *info in windows) {
            NSString *owner = info[(id)kCGWindowOwnerName];
            NSString *name = info[(id)kCGWindowName];
            if ([owner isEqualToString:@"Microsoft PowerPoint"]) {
                if (name != nil && ([name containsString:@"Slide Show"] || [name containsString:@"Diaporama"] ||
                                    [name containsString:@"Presentación"] || [name containsString:@"Bildschirmpräsentation"])) {
                    NSDictionary *boundsDict = info[(id)kCGWindowBounds];
                    if (boundsDict) {
                        CGRect cgBounds;
                        if (CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)boundsDict, &cgBounds)) {
                            CGFloat primaryH = screens[0].frame.size.height;
                            NSPoint centerPt = NSMakePoint(CGRectGetMidX(cgBounds), primaryH - CGRectGetMidY(cgBounds));
                            for (NSScreen *s in screens) {
                                if (NSPointInRect(centerPt, s.frame)) {
                                    CFRelease(windowList);
                                    return s;
                                }
                            }
                        }
                    }
                }
            }
        }
        CFRelease(windowList);
    }

    // 2. Multi-screen presentation heuristic:
    // screens[0] is the primary laptop display (menu bar, Presenter View).
    // screens[1] is the external presentation screen (audience projector).
    return screens[1];
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
            return;
        }

        NSScreen *targetScreen = findSlideShowScreen();
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

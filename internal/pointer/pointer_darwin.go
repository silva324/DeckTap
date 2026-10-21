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
    CGFloat cx = self.pointX * bounds.size.width;
    CGFloat cy = (1.0 - self.pointY) * bounds.size.height;

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

static void ensureWindowOnMainThread(void) {
    if (overlayWindow != nil) return;
    NSRect screenRect = [[NSScreen mainScreen] frame];
    overlayWindow = [[NSWindow alloc] initWithContentRect:screenRect
                                                styleMask:NSWindowStyleMaskBorderless
                                                  backing:NSBackingStoreBuffered
                                                    defer:NO];
    [overlayWindow setOpaque:NO];
    [overlayWindow setBackgroundColor:[NSColor clearColor]];
    [overlayWindow setLevel:NSScreenSaverWindowLevel];
    [overlayWindow setIgnoresMouseEvents:YES];
    [overlayWindow setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary];

    overlayView = [[LaserView alloc] initWithFrame:screenRect];
    [overlayWindow setContentView:overlayView];
    [overlayWindow orderFrontRegardless];
}

static void updatePointerNative(int active, double x, double y, int mode) {
    dispatch_async(dispatch_get_main_queue(), ^{
        ensureWindowOnMainThread();
        if (overlayView == nil) return;

        // Resize overlay to current main screen if it changed
        NSRect screenRect = [[NSScreen mainScreen] frame];
        if (!NSEqualRects([overlayWindow frame], screenRect)) {
            [overlayWindow setFrame:screenRect display:NO];
            [overlayView setFrame:screenRect];
        }

        overlayView.active = (active != 0);
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

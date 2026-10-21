//go:build darwin && cgo

package nativewin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface DeckTapWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation DeckTapWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
    [sender orderOut:nil];
    return NO;
}
@end

static NSWindow *pairingWin = nil;
static WKWebView *pairingWeb = nil;
static DeckTapWindowDelegate *winDel = nil;
static id keyMonitor = nil;

static void openNativeWindow(const char *cUrl, const char *cTitle, int w, int h) {
    if (!cUrl) return;
    NSString *urlStr = [NSString stringWithUTF8String:cUrl];
    NSString *titleStr = cTitle ? [NSString stringWithUTF8String:cTitle] : @"DeckTap";
    NSURL *nsUrl = [NSURL URLWithString:urlStr];

    dispatch_async(dispatch_get_main_queue(), ^{
        if (keyMonitor == nil) {
            keyMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyDown handler:^NSEvent *(NSEvent *event) {
                if (([event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask) == NSEventModifierFlagCommand) {
                    NSString *chars = [event charactersIgnoringModifiers];
                    if ([chars isEqualToString:@"q"] || [chars isEqualToString:@"Q"]) {
                        [NSApp terminate:nil];
                        return nil;
                    }
                    if ([chars isEqualToString:@"w"] || [chars isEqualToString:@"W"]) {
                        if (pairingWin != nil) {
                            [pairingWin orderOut:nil];
                            return nil;
                        }
                    }
                }
                return event;
            }];
        }

        if (pairingWin == nil) {
            NSRect screenRect = [[NSScreen mainScreen] visibleFrame];
            CGFloat width = (CGFloat)w;
            CGFloat height = (CGFloat)h;
            CGFloat x = NSMidX(screenRect) - (width / 2.0);
            CGFloat y = NSMidY(screenRect) - (height / 2.0);
            NSRect frame = NSMakeRect(x, y, width, height);

            NSUInteger mask = NSWindowStyleMaskTitled |
                              NSWindowStyleMaskClosable |
                              NSWindowStyleMaskMiniaturizable |
                              NSWindowStyleMaskResizable;

            pairingWin = [[NSWindow alloc] initWithContentRect:frame
                                                     styleMask:mask
                                                       backing:NSBackingStoreBuffered
                                                         defer:NO];
            [pairingWin setReleasedWhenClosed:NO];
            [pairingWin setTitle:titleStr];
            [pairingWin setTitlebarAppearsTransparent:YES];
            [pairingWin setAppearance:[NSAppearance appearanceNamed:NSAppearanceNameDarkAqua]];
            [pairingWin setBackgroundColor:[NSColor colorWithCalibratedRed:0.06 green:0.09 blue:0.16 alpha:1.0]];

            winDel = [[DeckTapWindowDelegate alloc] init];
            [pairingWin setDelegate:winDel];

            WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
            pairingWeb = [[WKWebView alloc] initWithFrame:[[pairingWin contentView] bounds] configuration:config];
            [pairingWeb setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];

            [[pairingWin contentView] addSubview:pairingWeb];
        } else {
            [pairingWin setTitle:titleStr];
        }

        NSURLRequest *req = [NSURLRequest requestWithURL:nsUrl];
        [pairingWeb loadRequest:req];

        [pairingWin makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}
*/
import "C"
import "unsafe"

func openPlatform(cfg WindowConfig) {
	cURL := C.CString(cfg.URL)
	defer C.free(unsafe.Pointer(cURL))
	cTitle := C.CString(cfg.Title)
	defer C.free(unsafe.Pointer(cTitle))

	C.openNativeWindow(cURL, cTitle, C.int(cfg.Width), C.int(cfg.Height))
}

// Supported reports true when running on macOS with CGO.
func Supported() bool {
	return true
}

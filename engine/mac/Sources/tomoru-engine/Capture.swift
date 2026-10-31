// Hearing the system: a Core Audio process tap (macOS 14.2+), which needs only the System Audio
// Recording permission, not Screen Recording.
import AppKit
import AVFoundation
import CoreAudio

struct CaptureError: Error, CustomStringConvertible {
    let description: String
}

/// TapCapture hears what apps play: all of them but this process, or one app's. The tap lives in
/// a private aggregate device, which goes when the process does.
final class TapCapture {
    let queue = DispatchQueue(label: "capture.tap")
    let onBuffer: (AVAudioPCMBuffer) -> Void
    var tapID = AudioObjectID(kAudioObjectUnknown), aggID = AudioObjectID(kAudioObjectUnknown)
    var procID: AudioDeviceIOProcID?
    var format: AVAudioFormat!
    // What arrives, logged every 30 s: a denied permission gives silence, not an error.
    var frames = 0, peak: Float = 0, lastReport = Date()

    init(onBuffer: @escaping (AVAudioPCMBuffer) -> Void) { self.onBuffer = onBuffer }

    func start(app: String?) throws {
        func check(_ st: OSStatus, _ what: String) throws {
            if st != noErr { throw CaptureError(description: "\(what) failed (\(st))") }
        }
        let desc: CATapDescription
        if let app {
            // An app plays through helpers too (a browser's audio process), with bundle IDs of their own.
            let appPIDs = Set(NSRunningApplication.runningApplications(withBundleIdentifier: app).map(\.processIdentifier))
            var objs: [AudioObjectID] = [], bundles: Set<String> = [app]
            for obj in processObjects() {
                var pid: pid_t = 0
                guard getProp(obj, kAudioProcessPropertyPID, &pid) else { continue }
                let bundle = stringProp(obj, kAudioProcessPropertyBundleID) ?? ""
                if bundle == app || bundle.hasPrefix(app + ".") || appPIDs.contains(responsiblePID(pid)) {
                    objs.append(obj)
                    if !bundle.isEmpty { bundles.insert(bundle) }
                }
            }
            log("tap \(app): \(objs.count) processes, bundles \(bundles.sorted())")
            desc = CATapDescription(monoMixdownOfProcesses: objs)
            // Processes that start later, by bundle ID (macOS 26).
            desc.bundleIDs = Array(bundles)
            desc.isProcessRestoreEnabled = true
        } else {
            var me = AudioObjectID(kAudioObjectUnknown)
            var pid = getpid()
            var addr = AudioObjectPropertyAddress(mSelector: kAudioHardwarePropertyTranslatePIDToProcessObject,
                                                  mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
            var size = UInt32(MemoryLayout<AudioObjectID>.size)
            AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &addr, UInt32(MemoryLayout<pid_t>.size), &pid, &size, &me)
            desc = CATapDescription(monoGlobalTapButExcludeProcesses: me == kAudioObjectUnknown ? [] : [me])
        }
        desc.uuid = UUID()
        desc.isPrivate = true
        desc.muteBehavior = .unmuted
        try check(AudioHardwareCreateProcessTap(desc, &tapID), "create tap")

        var asbd = AudioStreamBasicDescription()
        try check(getProp(tapID, kAudioTapPropertyFormat, &asbd) ? noErr : -1, "tap format")
        guard let fmt = AVAudioFormat(streamDescription: &asbd) else { throw CaptureError(description: "tap format unusable") }
        format = fmt

        var out = AudioObjectID(kAudioObjectUnknown)
        try check(getProp(AudioObjectID(kAudioObjectSystemObject), kAudioHardwarePropertyDefaultSystemOutputDevice, &out) ? noErr : -1, "output device")
        let outUID = stringProp(out, kAudioDevicePropertyDeviceUID) ?? ""
        let agg: [String: Any] = [
            kAudioAggregateDeviceNameKey: "Tomoru tap",
            kAudioAggregateDeviceUIDKey: UUID().uuidString,
            kAudioAggregateDeviceMainSubDeviceKey: outUID,
            kAudioAggregateDeviceIsPrivateKey: true,
            kAudioAggregateDeviceIsStackedKey: false,
            kAudioAggregateDeviceTapAutoStartKey: true,
            kAudioAggregateDeviceSubDeviceListKey: [[kAudioSubDeviceUIDKey: outUID]],
            kAudioAggregateDeviceTapListKey: [[kAudioSubTapDriftCompensationKey: true, kAudioSubTapUIDKey: desc.uuid.uuidString]],
        ]
        try check(AudioHardwareCreateAggregateDevice(agg as CFDictionary, &aggID), "aggregate device")
        try check(AudioDeviceCreateIOProcIDWithBlock(&procID, aggID, queue) { [weak self] _, input, _, _, _ in
            guard let self, let pcm = AVAudioPCMBuffer(pcmFormat: self.format, bufferListNoCopy: input, deallocator: nil) else { return }
            self.report(pcm)
            self.onBuffer(pcm)
        }, "IO proc")
        // The permission prompt shows here, the first time; denied, the tap gives silence.
        try check(AudioDeviceStart(aggID, procID), "start")
        log("tap \(fmt.sampleRate) Hz, \(fmt.channelCount) ch, via \(outUID)")
    }

    func report(_ pcm: AVAudioPCMBuffer) {
        if let ch = pcm.floatChannelData?[0] {
            for i in 0..<Int(pcm.frameLength) { peak = max(peak, abs(ch[i])) }
        }
        frames += Int(pcm.frameLength)
        if Date().timeIntervalSince(lastReport) >= 30 {
            log(String(format: "tap: %.1f s of audio, peak %.4f", Double(frames) / format.sampleRate, peak))
            frames = 0; peak = 0; lastReport = Date()
        }
    }
}

func getProp<T: BitwiseCopyable>(_ obj: AudioObjectID, _ sel: AudioObjectPropertySelector, _ v: inout T) -> Bool {
    var addr = AudioObjectPropertyAddress(mSelector: sel, mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var size = UInt32(MemoryLayout<T>.size)
    return AudioObjectGetPropertyData(obj, &addr, 0, nil, &size, &v) == noErr
}

func stringProp(_ obj: AudioObjectID, _ sel: AudioObjectPropertySelector) -> String? {
    var s: Unmanaged<CFString>?
    return getProp(obj, sel, &s) ? s?.takeRetainedValue() as String? : nil
}

/// processObjects are Core Audio's process objects: the processes that use audio.
func processObjects() -> [AudioObjectID] {
    var addr = AudioObjectPropertyAddress(
        mSelector: kAudioHardwarePropertyProcessObjectList, mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
    var size: UInt32 = 0
    let sys = AudioObjectID(kAudioObjectSystemObject)
    guard AudioObjectGetPropertyDataSize(sys, &addr, 0, nil, &size) == noErr else { return [] }
    var ids = [AudioObjectID](repeating: 0, count: Int(size) / MemoryLayout<AudioObjectID>.size)
    guard AudioObjectGetPropertyData(sys, &addr, 0, nil, &size, &ids) == noErr else { return [] }
    return ids
}

/// responsiblePID is the process responsible for pid: a helper's app.
let responsibleFn = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "responsibility_get_pid_responsible_for_pid")
    .map { unsafeBitCast($0, to: (@convention(c) (pid_t) -> pid_t).self) }
func responsiblePID(_ pid: pid_t) -> pid_t { responsibleFn?(pid) ?? pid }

// MARK: - Permission

/// tccPreflight says whether TCC lets this process (or the one responsible for it) use a service:
/// "granted", "denied", or "unknown" when the user has not been asked. Private SPI: there is no
/// public API to check System Audio Recording.
func tccPreflight(_ service: String) -> String {
    guard let h = dlopen("/System/Library/PrivateFrameworks/TCC.framework/Versions/A/TCC", RTLD_NOW),
          let f = dlsym(h, "TCCAccessPreflight") else { return "unknown" }
    let preflight = unsafeBitCast(f, to: (@convention(c) (CFString, CFDictionary?) -> Int).self)
    switch preflight(service as CFString, nil) {
    case 0: return "granted"
    case 1: return "denied"
    default: return "unknown"
    }
}

/// audioAccess says whether a tap may hear the system. Screen Recording covers it too.
func audioAccess() -> String {
    CGPreflightScreenCaptureAccess() ? "granted" : tccPreflight("kTCCServiceAudioCapture")
}

/// screenAccess says whether the app may capture the screen (Screen Recording), which selecting a
/// part of it to translate takes: "granted", "denied", or "unknown" when the user has not been asked.
func screenAccess() -> String { tccPreflight("kTCCServiceScreenCapture") }

/// requestScreenAccess has the system ask the user for Screen Recording, once, and waits for the
/// answer, a minute at most. The system only offers to open System Settings, where the user turns
/// it on; it takes effect once the app is reopened, which System Settings offers.
func requestScreenAccess() -> String {
    if screenAccess() == "unknown" {
        _ = CGRequestScreenCaptureAccess()
        for _ in 0..<300 where screenAccess() == "unknown" { usleep(200_000) }
    }
    return screenAccess()
}

// MARK: - Apps

/// listApps sends the apps whose sound can be heard on their own: the running apps with a Dock
/// icon, and any app playing sound, those playing first.
func listApps() {
    let playing = playingPIDs()
    // Not the app that runs the engine: Tomoru itself.
    let own = NSRunningApplication(processIdentifier: responsiblePID(getpid()))?.bundleIdentifier
    var seen = Set<String>()
    var apps: [(name: String, id: String, playing: Bool)] = []
    for app in NSWorkspace.shared.runningApplications {
        guard let id = app.bundleIdentifier, let name = app.localizedName, !seen.contains(id),
              id != own
        else { continue }
        let on = playing.contains(app.processIdentifier)
        guard on || app.activationPolicy == .regular else { continue }
        seen.insert(id)
        apps.append((name, id, on))
    }
    apps.sort { $0.playing != $1.playing ? $0.playing : $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    for a in apps { send(.app(a.id, a.name, a.playing)) }
}

/// playingPIDs are the apps playing sound now. A helper that plays for an app, as a browser's
/// audio process, counts as the app responsible for it.
func playingPIDs() -> Set<pid_t> {
    var out = Set<pid_t>()
    for id in processObjects() {
        var running: UInt32 = 0
        var pid: pid_t = 0
        guard getProp(id, kAudioProcessPropertyIsRunningOutput, &running), running != 0, getProp(id, kAudioProcessPropertyPID, &pid) else { continue }
        out.insert(pid)
        out.insert(responsiblePID(pid))
    }
    return out
}

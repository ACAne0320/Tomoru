// tomoru-engine: Tomoru's macOS engine. It hears the system's audio with a Core Audio tap,
// recognizes speech with SpeechAnalyzer, and speaks docs/engine-protocol.md on stdin and stdout.
//
//	tomoru-engine listen --locale <bcp47> [--app <bundle id> | --file <audio>] [--vad <model.mlmodelc> | none] [--asr none]
//	tomoru-engine locales
//	tomoru-engine prepare --locale <bcp47>
//	tomoru-engine speak --locale <bcp47> -- <sentence>...
//	tomoru-engine access | request-access | screen-access | request-screen-access | apps
//	tomoru-engine select --out <png>
import Foundation

setvbuf(stdout, nil, _IOLBF, 0)

let args = Array(CommandLine.arguments.dropFirst())

func option(_ name: String) -> String? {
    guard let i = args.firstIndex(of: name), i + 1 < args.count else { return nil }
    return args[i + 1]
}

func usage() -> Never {
    FileHandle.standardError.write(Data("usage: tomoru-engine listen --locale <bcp47> [--app <bundle id>] | locales | prepare --locale <bcp47> | speak --locale <bcp47> -- <sentence>... | access | request-access | screen-access | request-screen-access | apps | select --out <png>\n".utf8))
    exit(2)
}

hello()
switch args.first {
case "access":
    send(.access(audioAccess()))
    exit(0)

case "request-access":
    // There is no call to ask: the system asks the first time a tap records. Start one and wait
    // for the answer, a minute at most.
    if audioAccess() == "unknown" {
        let tap = TapCapture { _ in }
        DispatchQueue.global().async { try? tap.start(app: nil) }
        for _ in 0..<300 where audioAccess() == "unknown" { usleep(200_000) }
    }
    send(.access(audioAccess()))
    exit(0)

case "screen-access":
    send(.access(screenAccess()))
    exit(0)

case "request-screen-access":
    send(.access(requestScreenAccess()))
    exit(0)

case "apps":
    listApps()
    exit(0)

case "select":
    guard let out = option("--out") else { usage() }
    selectArea(out)
    exit(0)

case "locales":
    Task {
        await listLocales()
        exit(0)
    }
    dispatchMain()

case "prepare":
    guard let tag = option("--locale") else { usage() }
    Task {
        let locale = await supportedLocale(tag)
        await installAssets(makeTranscriber(locale), locale)
        send(.state("ready"))
        exit(0)
    }
    dispatchMain()

case "speak":
    guard let tag = option("--locale"), let i = args.firstIndex(of: "--"), i + 1 < args.count else { usage() }
    signal(SIGINT) { _ in exit(0) }
    let speaker = Speaker()
    speaker.speak(Array(args[(i + 1)...]), tag: tag)
    dispatchMain()

case "listen":
    guard let tag = option("--locale") else { usage() }
    // Commands come on stdin; when it closes, the app has gone, and so does the engine.
    var listener: Listener?
    Thread.detachNewThread {
        do {
            try consumeCommands(nextLine: { readLine() }) { command in
                if command.type == "finalize", let t = command.through {
                    listener?.finalize(through: t)
                }
            }
        } catch { fail("internal", "stdin: \(error)") }
        log("stdin closed")
        exit(0)
    }
    signal(SIGINT) { _ in exit(0) }
    // Voice detection: the model beside the engine, or --vad's; --vad none turns it off.
    var vad = VoiceDetector.modelURL()
    if let v = option("--vad") { vad = v == "none" ? nil : URL(fileURLWithPath: v) }
    if option("--asr") == "none" {
        // Hearing only, for tomoru-asr or the cloud: finalize is their business.
        let h = Hearer(app: option("--app"), vad: vad, file: option("--file"))
        h.start()
        dispatchMain()
    }
    Task {
        let l = Listener(locale: await supportedLocale(tag), app: option("--app"), vad: vad, file: option("--file"))
        listener = l
        await l.start()
    }
    dispatchMain()

default:
    usage()
}

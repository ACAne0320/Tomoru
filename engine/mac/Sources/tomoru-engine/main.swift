import Foundation

setvbuf(stdout, nil, _IOLBF, 0)

let args = Array(CommandLine.arguments.dropFirst())

func option(_ name: String) -> String? {
    guard let i = args.firstIndex(of: name), i + 1 < args.count else { return nil }
    return args[i + 1]
}

func usage() -> Never {
    FileHandle.standardError.write(Data("usage: tomoru-engine listen --locale <bcp47> [--app <bundle id>] | locales | prepare --locale <bcp47> | speak --file <wav> | access | request-access | screen-access | request-screen-access | apps | select --out <png>\n".utf8))
    exit(2)
}

hello()
switch args.first {
case "access":
    send(.access(audioAccess()))
    exit(0)

case "request-access":
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
    guard let file = option("--file") else { usage() }
    signal(SIGINT) { _ in exit(0) }
    signal(SIGTERM) { _ in exit(0) }
    let speaker = MainActor.assumeIsolated {
        let s = Speaker()
        s.speak(file)
        return s
    }
    withExtendedLifetime(speaker) { dispatchMain() }

case "listen":
    guard let tag = option("--locale") else { usage() }
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
    var vad = VoiceDetector.modelURL()
    if let v = option("--vad") { vad = v == "none" ? nil : URL(fileURLWithPath: v) }
    if option("--asr") == "none" {
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

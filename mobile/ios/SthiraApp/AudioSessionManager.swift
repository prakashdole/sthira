import Foundation
import AVFoundation

/// AudioSessionManager: manages iOS audio session lifecycle, responding gracefully to
/// incoming phone calls, alarms, and headphone route changes.
class AudioSessionManager: ObservableObject {
    static let shared = AudioSessionManager()

    var onInterruptionBegan: (() -> Void)?
    var onInterruptionEnded: (() -> Void)?

    private init() {
        setupNotifications()
    }

    func configureSessionForPlaybackAndRecord() throws {
        let session = AVAudioSession.sharedInstance()
        try session.setCategory(.playAndRecord, mode: .voiceChat, options: [.defaultToSpeaker, .allowBluetooth])
        try session.setActive(true)
    }

    func deactivateSession() {
        do {
            try AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        } catch {}
    }

    private func setupNotifications() {
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(handleInterruption),
            name: AVAudioSession.interruptionNotification,
            object: AVAudioSession.sharedInstance()
        )
    }

    @objc private func handleInterruption(notification: Notification) {
        guard let userInfo = notification.userInfo,
              let typeValue = userInfo[AVAudioSessionInterruptionTypeKey] as? UInt,
              let type = AVAudioSession.InterruptionType(rawValue: typeValue) else {
            return
        }

        switch type {
        case .began:
            onInterruptionBegan?()
        case .ended:
            if let optionsValue = userInfo[AVAudioSessionInterruptionOptionKey] as? UInt {
                let options = AVAudioSession.InterruptionOptions(rawValue: optionsValue)
                if options.contains(.shouldResume) {
                    onInterruptionEnded?()
                }
            }
        @unknown default:
            break
        }
    }
}

/// Ephemeral in-memory audio recording for IndicConformer ASR.
/// Invariants (O10):
/// - 0 ms disk retention: writes solely into an in-memory Data buffer.
/// - Caps max utterance duration at 5000 ms.
/// - Never retains or logs raw audio bytes.
class EphemeralAudioRecorder {
    private var audioEngine: AVAudioEngine?
    private var audioData = Data()
    private(set) var isRecording = false

    func startRecording(completion: @escaping (Result<Data, Error>) -> Void) {
        audioData.removeAll()
        let engine = AVAudioEngine()
        self.audioEngine = engine

        let inputNode = engine.inputNode
        let recordingFormat = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true)

        guard let format = recordingFormat else {
            completion(.failure(NSError(domain: "SthiraAudio", code: -1, userInfo: [NSLocalizedDescriptionKey: "Invalid audio format"])))
            return
        }

        inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputNode.outputFormat(forBus: 0)) { [weak self] buffer, _ in
            guard let self = self else { return }
            // Convert to 16kHz mono 16-bit PCM in-memory
            let channelData = buffer.floatChannelData?[0]
            if let data = channelData {
                let frameLength = Int(buffer.frameLength)
                var pcmBytes = Data(capacity: frameLength * 2)
                for i in 0..<frameLength {
                    let floatSample = max(-1.0, min(1.0, data[i]))
                    var intSample = Int16(floatSample * 32767.0)
                    withUnsafeBytes(of: &intSample) { pcmBytes.append(contentsOf: $0) }
                }
                self.audioData.append(pcmBytes)
            }
        }

        do {
            try engine.start()
            isRecording = true

            // Automatically stop recording after 5 seconds max
            DispatchQueue.main.asyncAfter(deadline: .now() + 5.0) { [weak self] in
                if self?.isRecording == true {
                    self?.stopRecording(completion: completion)
                }
            }
        } catch {
            completion(.failure(error))
        }
    }

    func stopRecording(completion: @escaping (Result<Data, Error>) -> Void) {
        guard isRecording else { return }
        isRecording = false
        audioEngine?.inputNode.removeTap(onBus: 0)
        audioEngine?.stop()
        audioEngine = nil

        let wavData = createWavHeader(pcmData: audioData, sampleRate: 16000, channels: 1, bitDepth: 16)
        completion(.success(wavData))
    }

    private func createWavHeader(pcmData: Data, sampleRate: Int32, channels: Int16, bitDepth: Int16) -> Data {
        var header = Data()
        let totalDataLen = Int32(pcmData.count + 36)
        let byteRate = sampleRate * Int32(channels) * Int32(bitDepth) / 8
        let blockAlign = channels * bitDepth / 8

        header.append("RIFF".data(using: .ascii)!)
        withUnsafeBytes(of: totalDataLen.littleEndian) { header.append(contentsOf: $0) }
        header.append("WAVE".data(using: .ascii)!)
        header.append("fmt ".data(using: .ascii)!)
        var subchunk1Size: Int32 = 16
        withUnsafeBytes(of: subchunk1Size.littleEndian) { header.append(contentsOf: $0) }
        var audioFormat: Int16 = 1 // PCM
        withUnsafeBytes(of: audioFormat.littleEndian) { header.append(contentsOf: $0) }
        withUnsafeBytes(of: channels.littleEndian) { header.append(contentsOf: $0) }
        withUnsafeBytes(of: sampleRate.littleEndian) { header.append(contentsOf: $0) }
        withUnsafeBytes(of: byteRate.littleEndian) { header.append(contentsOf: $0) }
        withUnsafeBytes(of: blockAlign.littleEndian) { header.append(contentsOf: $0) }
        withUnsafeBytes(of: bitDepth.littleEndian) { header.append(contentsOf: $0) }
        header.append("data".data(using: .ascii)!)
        var dataLen = Int32(pcmData.count)
        withUnsafeBytes(of: dataLen.littleEndian) { header.append(contentsOf: $0) }

        return header + pcmData
    }
}

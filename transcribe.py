from sys import argv
from faster_whisper import WhisperModel

def format_time(time_in_seconds):
    hours, remainder = divmod(time_in_seconds, 3600)
    minutes, seconds = divmod(remainder, 60)
    milliseconds = int((seconds % 1) * 1000)
    return f"{int(hours):02}:{int(minutes):02}:{int(seconds):02},{milliseconds:03}"

def save_to_srt(segments, filename="transcription.srt"):
    with open(filename, "w") as file:
        for i, segment in enumerate(segments, start=1):
            # Convert time to hh:mm:ss,SSS
            start_time_str = format_time(segment.start)
            end_time_str = format_time(segment.end)

            # Save SRT segment
            file.write(f"{i}\n")
            file.write(f"{start_time_str} --> {end_time_str}\n")
            file.write(f"{segment.text}\n\n")

            # Print progress
            print(f" {start_time_str} --> {end_time_str} | {segment.text}")

# model_size = "large-v3"
# model_size = "small"
model_size = argv[2]
threads = int(argv[3])
app_dir = argv[4]
action = argv[5] # translate or transcribe
print(f"Using {model_size} on {threads} cpu threads.")

# Run on GPU with FP16
# model = WhisperModel(model_size, device="cuda", compute_type="float16")

# or run on GPU with INT8
# model = WhisperModel(model_size, device="cuda", compute_type="int8_float16")

# or run on CPU with INT8
model = WhisperModel(model_size, device="cpu", compute_type="int8", cpu_threads=threads, download_root=app_dir)

segments, info = model.transcribe("tmp/audio.mp3", beam_size=5, task=action)
print("Detected language '%s' with probability %f." % (info.language, info.language_probability))

save_to_srt(segments, f"tmp/{argv[1]} (transcription).srt")
print("Transcription saved to srt file.")

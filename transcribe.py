import argparse
from faster_whisper import WhisperModel

def format_time(time_in_seconds):
    hours, remainder = divmod(time_in_seconds, 3600)
    minutes, seconds = divmod(remainder, 60)
    milliseconds = int((seconds % 1) * 1000)
    return f"{int(hours):02}:{int(minutes):02}:{int(seconds):02},{milliseconds:03}"

def save_to_srt(segments, filename):
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

parser = argparse.ArgumentParser()
parser.add_argument('--input')
parser.add_argument('--model')
parser.add_argument('--threads')
parser.add_argument('--appdir')
parser.add_argument('--action') # translate or transcribe
parser.add_argument('--output')
args = parser.parse_args()
print(args)

# args.model = "large-v3"
# args.model = "small"
threads = int(args.threads)
print(f"Using {args.model} on {args.threads} cpu threads.")

# Run on GPU with FP16
# model = WhisperModel(args.model, device="cuda", compute_type="float16")

# or run on GPU with INT8
# model = WhisperModel(args.model, device="cuda", compute_type="int8_float16")

# or run on CPU with INT8
model = WhisperModel(args.model, device="cpu", compute_type="int8", cpu_threads=threads, download_root=args.appdir)

segments, info = model.transcribe(args.input, beam_size=5, task=args.action)
print("Detected language '%s' with probability %f." % (info.language, info.language_probability))

save_to_srt(segments, args.output)
print("Transcription saved to srt file.")

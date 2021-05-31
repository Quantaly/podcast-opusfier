# podcast-opusfier

Re-encodes podcast feeds into 64 kb/s Opus audio. Saves space without sacrificing quality, assuming your podcast client knows what to do with `.opus` files.

## Usage

Ensure the [`ffmpeg`](https://ffmpeg.org/) binaries are available to the service

Point your podcast client to `http://<this service>/rss/<link to original RSS feed>`
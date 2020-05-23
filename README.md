# AutoYT

A CLI tool for automating the editing and uploading process of YouTube music videos.

```
go get github.com/stillwwater/autoyt
```

##  Usage

```bash
# a minimal example (see autoyt --help for a full list of options):
# add new music and artwork to the buffer
autoyt add music "ohwell - blue by you.mp3"
autoyt add art -a snatti89 "https://i.imgur.com/jJkRk1u.jpg"

# preview video description, the desc command lets you change any information on videos before they are scheduled
autoyt desc

# encode videos using ffmpeg and schedule an upload time
# images and music will be picked from the buffer
autoyt schedule

# upload scheduled videos to youtube. Videos will be private at first and made public at the scheduled date (this is handled by YouTube)
autoyt upload
```

You may change the default behavior and video templates using a `config.json` file. A default will be written to `~/.autoyt/config.json`, you may edit this directly or place your own config file in `~/.config/autoyt/config.json`.

## Requirements

- AutoYT uses ffmpeg to encode videos, you may download it [here](https://ffmpeg.org/). On Windows you may need to manually add ffmpeg to `$PATH` or provide the full path to the ffmpeg binary in `config.json`.
- You will need access to the YouTube API and download your `client_secret.json` to `~/.autoyt`, you can do so using the [Google developer console](https://console.developers.google.com/). When you run `autoyt upload` for the first time you will be prompted to authorize the app.

## License

[MIT](./LICENSE)

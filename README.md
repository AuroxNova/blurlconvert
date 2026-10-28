# blurlconvert
Reads Decrypts and Converts blurl files & blurl json files.

# Requirements
- ffmpeg in path
- keys.bin in the same directory of the executable
- Install Golang to build code into a exe file

# Why this new update?
- Fortnite Changed how MPD Files are structured, instead of working with Segments, Now it's working uploading the full song but encrypted

- Example: instead of init_0.mp4 to init_203.mp4, now is main_0_dashinit.mp4 (for festival tracks)

main_0_0_dashinit.mp4 (for Preview tracks) [Those are not encrypted anymore, bc it doesn't make any sense encrypting the previews lol]

- This program is still working with old mpd files, downloading the segments, etc.

# How To Build EXE file
- Open CMD on the folder path and Type:
```yaml
go build ./
```
# How To run the file or exe
- Open CMD on the Folder path, if you wanna run it without making the build just run:
```yaml
go run ./ master.blurl
```
Or
```yaml
go run ./ C:\Users\PC\Documents\Festival Exporter\master.blurl
```
(THE Blurl have to be inside of the folder or copy the path of the blurl file too)

To run on the build code
```yaml
blurlconvert.exe master.blurl
```

# Optional output path
Pass a second argument to choose where the result goes. Without it, files are written to the current folder as `master_audio.mp4`, `master_video.mp4` or `<kid>_master.mp4`.
```yaml
blurlconvert.exe master.blurl "C:\Users\PC\Music\song.mp4"
```
With both an audio and a video track the name is used as a base (`song_audio.mp4`, `song_video.mp4`) and the merged result is written to `song.mp4`.


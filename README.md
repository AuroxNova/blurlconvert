# blurlconvert
Reads Decrypts and Converts blurl files & blurl json files.

# Requirements
- ffmpeg in path
- keys.bin in the same directory of the executable
- Install Golang to build code into a exe file

# Why this new update?
- Fortnite Changed how MPD Files are structured, instead of working with Segments, Now it's working upload the full song but encrypted

- Example: instead of init_0.mp4 to init_203.mp4, now is main_0_dashinit.mp4 (for festival tracks)
main_0_0_dashinit.mp4 (for Preview tracks) [Those are not encrypted anymore, bc it doesn't make any sense encrypting the previews lol]

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

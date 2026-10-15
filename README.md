
# 1、introduction

![首页](images/home.png "首页")
![阅读界面](images/read-content.png "阅读界面")
![阅读界面](images/read-content-catlogs.png "阅读界面")
![阅读界面设置](images/read-content-settings.png "阅读界面设置")
![阅读界面目录](images/read-content-mulu.png "阅读界面目录")
![阅读界面目录](images/book-download.png "阅读界面目录")

A mysterious e-book reader

You can use a computer in any setting, whether it's for work or school, as long as you have it. It's highly concealed and difficult to detect

Suitable for Windows, macOS, Linux

Supports e-book formats such as txt, epub, mobi, azw3, azw, docx, dox, pdf, html, htmlz, etc

The time required for non txt epub file formats may be relatively long (especially for mobi files around 10MB, which is particularly slow as the file size increases). If you spend a long time spinning around, you should find an online file format to convert to epub file and then operate again


ps:
    Slow is likely due to the author being too poor to afford the cloud server fees  o(╥﹏╥)o!

# 2、feature

- Transparent reading mode
- Hidden mode
- App Top mode
- Automatic directory extraction
- Many ways to Add file
- Delete files
- Custom setting And The functions that an e-book reader should have
- support epub,mobi,azw,azw3,txt file local parse!

# 3、download

[click to download app](https://github.com/lbk-ones/wd-reader/releases)


# 4、acknowledgments
thanks wails!

# 5、env
Go 1.25+ and Node.js 24 LTS. Wails is pinned to v3.0.0-beta.29 (including the frontend runtime).

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.29
wails3 dev
wails3 build
```

Build output: `bin/wdreader.exe` on Windows, `bin/wdreader` on macOS/Linux.
Run commands from the repository root. On Windows, the existing `books` directory is relative to the working directory; on macOS/Linux it remains `~/books`.

See [the Wails 3 migration notes](docs/wails3-migration.md) for feature mapping, packaging commands, compatibility and verification details.

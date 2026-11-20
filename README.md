# mylan

局域网设备菜单栏应用。扫描使用内嵌的 [lanmac](embed/lanmac)，界面是 MyGo 原生浮层。

点菜单栏图标打开面板：列表查看设备、搜索、按类型筛选、给设备写备注、监控某个 IP。刷新在顶栏，快捷键 `⌘R`。监控的 IP 在下次扫描里上线、离线、换成别的设备或改了名称时，会发一条系统通知。结果超过 2 分钟后再打开面板会自动扫一次。

A menu-bar app for devices on the local network. Scanning uses the embedded [lanmac](embed/lanmac). The panel is a native MyGo flyout.

Click the menu-bar icon to open the panel: a list, search, a type filter, a note on a device, and a watch on one IP. Refresh is in the header, or `⌘R`. A watched IP sends a notification when the next scan finds it online, offline, replaced by another device, or renamed. Opening the panel again after two minutes scans once more.

## 安装

发布页提供这些安装包：

| 系统 | 安装包 |
| --- | --- |
| macOS（Apple 芯片） | `mylan 0.1.0 arm64.dmg` |
| macOS（Intel） | `mylan 0.1.0 amd64.dmg` |
| Windows x64 | `mylan Setup 0.1.0 amd64.exe` |
| Windows ARM | `mylan Setup 0.1.0 arm64.exe` |
| Linux x64 | `mylan-0.1.0-linux-amd64.tar.gz`、`mylan_0.1.0_amd64.deb` |
| Linux ARM | `mylan-0.1.0-linux-arm64.tar.gz`、`mylan_0.1.0_arm64.deb` |

### macOS

- 按芯片选择 dmg，打开后把 `mylan.app` 拖进「应用程序」。
- 这是菜单栏应用，程序坞里没有图标。点菜单栏图标打开面板，右键打开菜单。
- 第一次运行请允许「本地网络」。不允许的话，Bonjour 发现不到设备。
- 扫描使用系统自带的 `python3`，还要用到 `ping`、`arp`、`dns-sd` 和 `smbutil`。
- 完整扫描会 ping 当前网段，大约几秒。设置里可以改成快速模式、包含休眠设备、用 SMB 补名称，或指定网段。
- 安装包目前没有 Apple 公证。换一台 Mac 打开时，若系统拦截，到「系统设置 → 隐私与安全性」里允许。

### Windows

- x64 电脑用 `amd64` 安装包，ARM 电脑（如部分 Surface）用 `arm64` 安装包。
- 安装程序装到当前用户的 `%LOCALAPPDATA%\Programs`，不需要管理员权限。开始菜单里可以打开，设置里可以卸载。
- 安装包没有 Windows 代码签名。SmartScreen 提示时，选择仍要运行。
- 扫描脚本按 macOS 编写，调用 `route`、`ifconfig`、`arp`、`dns-sd` 和 macOS 版 `ping`。Windows 上可以打开界面，目前不能扫描局域网。

### Linux

- 需要图形桌面。托盘在 Linux 上是菜单，桌面需要能显示 AppIndicator（Debian、Ubuntu 上是 `libayatana-appindicator3-1`）。
- 用压缩包旁的 `install.sh` 安装时，应用在 `~/.local/mylan.app`，不需要 root。`.deb` 安装到 `/opt`，之后用系统的包管理器更新。
- 需要已安装的 `python3`。
- 扫描脚本按 macOS 编写。Linux 上可以打开界面，目前不能扫描局域网。

## 开发

需要 Go 1.27 或更新版本。在 Mac 上开发时还要系统自带的 `python3`。

```sh
go tool mygo doctor
go tool mygo dev
```

## 打包

当前系统打一个包：

```sh
go tool mygo build
```

一次打出上表里的各个系统：

```sh
go tool mygo build -platform darwin/arm64,darwin/amd64,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

在 Mac 上打 Windows 安装包需要 [NSIS](https://nsis.sourceforge.io)（`brew install makensis`）。没有它时，Windows 目录里只有程序，没有安装包。磁盘映像只在 macOS 上生成。

给更新签名时带上私钥：

```sh
export MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")"
go tool mygo build -platform darwin/arm64,darwin/amd64,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

私钥只留在本机，以及仓库的 `MYGO_UPDATER_PRIVATE_KEY` secret。不要放进仓库。

## 发布

版本写在 `mygo.json` 的 `version`，说明写在 `CHANGELOG.md` 里同号的 `## 版本`。推送同号的 tag 会生成草稿 Release，并上传各系统的安装包：

```sh
git tag v0.1.0
git push origin v0.1.0
```

---

## Install

The release page has these packages:

| System | Package |
| --- | --- |
| macOS (Apple silicon) | `mylan 0.1.0 arm64.dmg` |
| macOS (Intel) | `mylan 0.1.0 amd64.dmg` |
| Windows x64 | `mylan Setup 0.1.0 amd64.exe` |
| Windows ARM | `mylan Setup 0.1.0 arm64.exe` |
| Linux x64 | `mylan-0.1.0-linux-amd64.tar.gz`, `mylan_0.1.0_amd64.deb` |
| Linux ARM | `mylan-0.1.0-linux-arm64.tar.gz`, `mylan_0.1.0_arm64.deb` |

### macOS

- Pick the disk image for the Mac's chip, then drag `mylan.app` to Applications.
- The app lives in the menu bar and has no Dock icon. Click the icon for the panel. Right-click for the menu.
- Allow Local Network the first time it asks. Without that, Bonjour cannot find devices.
- Scanning uses the system `python3`, plus `ping`, `arp`, `dns-sd`, and `smbutil`.
- A full scan pings the current subnet and takes a few seconds. Settings can switch to fast mode, include sleeping devices, fill in names over SMB, or set a subnet.
- The package is not notarized by Apple. If another Mac blocks it, allow it in System Settings → Privacy & Security.

### Windows

- Use the `amd64` installer on x64 PCs, and the `arm64` installer on ARM PCs such as some Surface models.
- The installer puts the app in `%LOCALAPPDATA%\Programs` for the current user. It does not need an administrator. Open it from the Start menu, and uninstall it from Settings.
- The installer is not code-signed for Windows. If SmartScreen warns, choose to run it anyway.
- The scanner is written for macOS and calls `route`, `ifconfig`, `arp`, `dns-sd`, and the macOS `ping`. The window opens on Windows. Scanning the LAN does not work there yet.

### Linux

- A desktop session is required. The tray is a menu, so the desktop needs an AppIndicator (`libayatana-appindicator3-1` on Debian and Ubuntu).
- `install.sh`, next to the archive, installs into `~/.local/mylan.app` without root. The `.deb` installs into `/opt` and is updated by the package manager.
- `python3` must already be installed.
- The scanner is written for macOS. The window opens on Linux. Scanning the LAN does not work there yet.

## Develop

Go 1.27 or newer. On a Mac, the system `python3` as well.

```sh
go tool mygo doctor
go tool mygo dev
```

## Build

One package for the machine you are on:

```sh
go tool mygo build
```

Every package in the table:

```sh
go tool mygo build -platform darwin/arm64,darwin/amd64,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

Building the Windows installers on a Mac needs [NSIS](https://nsis.sourceforge.io) (`brew install makensis`). Without it, the Windows directory has the program and no installer. Disk images are created only on macOS.

Sign updates with the private key:

```sh
export MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")"
go tool mygo build -platform darwin/arm64,darwin/amd64,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

Keep that key on this machine and in the `MYGO_UPDATER_PRIVATE_KEY` repository secret. Do not commit it.

## Release

`version` in `mygo.json` is the version. The matching `## version` section in `CHANGELOG.md` is the note. Pushing the tag of that version creates a draft release and uploads every system's package:

```sh
git tag v0.1.0
git push origin v0.1.0
```

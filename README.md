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

扫描使用 macOS 的 `dns-sd`、`arp` 和 `ping`。Windows 和 Linux 目前不能发现设备，所以不发布这两个系统的安装包。

### macOS

- 按芯片选择 dmg，打开后把 `mylan.app` 拖进「应用程序」。
- 这是菜单栏应用，程序坞里没有图标。点菜单栏图标打开面板，右键打开菜单。
- 第一次运行请允许「本地网络」。不允许的话，Bonjour 发现不到设备。
- 扫描使用系统自带的 `python3`，还要用到 `ping`、`arp`、`dns-sd` 和 `smbutil`。
- 完整扫描会 ping 当前网段，大约几秒。设置里可以改成快速模式、包含休眠设备、用 SMB 补名称，或指定网段。
- 安装包目前没有 Apple 公证。换一台 Mac 打开时，若系统拦截，到「系统设置 → 隐私与安全性」里允许。

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

一次打出 Apple 芯片和 Intel 两份：

```sh
go tool mygo build -platform darwin/arm64,darwin/amd64
```

给更新签名时带上私钥：

```sh
export MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")"
go tool mygo build -platform darwin/arm64,darwin/amd64
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

Scanning uses the macOS `dns-sd`, `arp`, and `ping`. Windows and Linux cannot discover devices yet, so those installers are not published.

### macOS

- Pick the disk image for the Mac's chip, then drag `mylan.app` to Applications.
- The app lives in the menu bar and has no Dock icon. Click the icon for the panel. Right-click for the menu.
- Allow Local Network the first time it asks. Without that, Bonjour cannot find devices.
- Scanning uses the system `python3`, plus `ping`, `arp`, `dns-sd`, and `smbutil`.
- A full scan pings the current subnet and takes a few seconds. Settings can switch to fast mode, include sleeping devices, fill in names over SMB, or set a subnet.
- The package is not notarized by Apple. If another Mac blocks it, allow it in System Settings → Privacy & Security.

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

Both Mac packages:

```sh
go tool mygo build -platform darwin/arm64,darwin/amd64
```

Sign updates with the private key:

```sh
export MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")"
go tool mygo build -platform darwin/arm64,darwin/amd64
```

Keep that key on this machine and in the `MYGO_UPDATER_PRIVATE_KEY` repository secret. Do not commit it.

## Release

`version` in `mygo.json` is the version. The matching `## version` section in `CHANGELOG.md` is the note. Pushing the tag of that version creates a draft release and uploads every system's package:

```sh
git tag v0.1.0
git push origin v0.1.0
```

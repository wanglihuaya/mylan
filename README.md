# mylan

局域网设备菜单栏应用。扫描使用内嵌的 [lanmac](embed/lanmac)，界面是 MyGo 原生浮层。

点菜单栏图标打开面板：列表查看设备、搜索、按类型筛选、给设备写备注、监控某个 IP。刷新在顶栏，快捷键 `⌘R`。监控的 IP 在下次扫描里上线、离线、换成别的设备或改了名称时，会发一条系统通知。结果超过 2 分钟后再打开面板会自动扫一次。

## 开发

需要 Go 1.27 或更新版本，以及 macOS 自带的 `python3`。

```sh
go tool mygo doctor
go tool mygo dev
```

首次运行如果系统询问「本地网络」，需要允许，否则 Bonjour 发现不到设备。

完整扫描大约数秒，会 ping 当前网段。设置里可以改成快速模式、包含休眠设备、用 SMB 补名称，或指定网段。

## 打包

```sh
go tool mygo build
```

产物在 `build/darwin-arm64/`：签过名的 `mylan.app`，以及拖到「应用程序」安装的磁盘映像。给更新签名时带上私钥，会同时写出更新包和 `update-darwin-arm64.json`：

```sh
export MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")"
go tool mygo build
```

私钥只在本机和仓库的 `MYGO_UPDATER_PRIVATE_KEY` secret 里。装好的应用只接受这把私钥签过的更新。

## 发布和更新

版本在 `mygo.json` 的 `version`，发布说明是 `CHANGELOG.md` 里对应的 `## 版本`。推送相同的 tag 会打草稿 release，并上传安装包和签名更新：

```sh
git tag v0.1.0
git push origin v0.1.0
```

发布草稿之后，应用每天检查 `wanglihuaya/mylan` 的最新 release。有新版本时弹出更新窗口，可以安装、稍后提醒或跳过。右键菜单和设置页底部的「检查更新」会马上查一次。开发版不能替换自己，窗口会说明原因。

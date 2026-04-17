# mihomo-cli

mihomo 命令行管理工具

## 配置

编辑 `config.json`:

```json
{
  "subUrl": "https://your-subscription-url",
  "basePath": "base.yml",
  "overridePath": "override.yml"
}
```

- `base.yml` - 基础配置（DNS、端口等）
- `override.yml` - 用户覆盖配置（优先级最高）

## 命令

### download
下载最新 mihomo 二进制到 mihomo 目录

```bash
mihomo-cli download
```

### sub
更新订阅，与 base.yml、override.yml 合并后写入 mihomo/config.yaml

```bash
mihomo-cli sub
```

### start / stop
后台启动或停止 mihomo

```bash
mihomo-cli start
mihomo-cli stop
```

### proxy list
查看所有代理组和节点延迟

```bash
mihomo-cli proxy list
```

```
ProxyGroup (Selector)
  - Node1 (120ms)
  - Node2 (89ms) *
  - Node3 (150ms)
```

`*` 表示当前选中的节点

### proxy set
切换代理组选中的节点

```bash
mihomo-cli proxy set ProxyGroup Node2
```

### service install / uninstall
安装或卸载 systemd 服务

```bash
sudo mihomo-cli service install
sudo mihomo-cli service uninstall
```

服务以 root 身份运行，开启 TUN 模式。

## 注意事项

- 测速会跳过 DIRECT 和 REJECT 类型节点
- `sub` 成功后会自动重启 mihomo 服务（如已安装）
- API 请求会自动使用 config.yaml 中的 secret 认证
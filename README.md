# Tar2OCI

轻量级命令行工具，将 tar 包直接转换为符合 OCI 标准的容器镜像，无需 Docker Daemon。

## 特性

- **无依赖构建** — 无需安装 Docker 或 containerd，无需 Docker Daemon 权限
- **多层支持** — 支持多个 tar 包叠加为多层镜像
- **基础镜像扩展** — 基于已有镜像叠加新层
- **双格式输出** — Docker tar（可 `docker load`）或 OCI layout
- **Registry 推送** — 直接推送到远程镜像仓库
- **多架构支持** — 指定目标平台（amd64/arm64 等）
- **流式处理** — 低内存占用，适合大型 tar 包

## 安装

```bash
go install github.com/TDnorthgarden/Tar2OCI/cmd/tar2oci@latest
```

或从源码构建：

```bash
git clone https://github.com/TDnorthgarden/Tar2OCI.git
cd Tar2OCI
go build -o tar2oci ./cmd/tar2oci
```

## 快速开始

### 单层镜像

```bash
# 创建应用 tar 包
tar cf app.tar -C /path/to/app .

# 构建镜像
tar2oci build \
  --input app.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server" \
  --cmd "--port 8080" \
  --env "MODE=production"

# 加载到 Docker
docker load -i my-app:v1.0.tar
docker run --rm my-app:v1.0
```

### 多层镜像

```bash
tar2oci build \
  --input base-deps.tar \
  --input runtime-libs.tar \
  --input app-binary.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server"
```

### 基于基础镜像扩展

```bash
tar2oci build \
  --base alpine.tar \
  --input app.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server"
```

### OCI Layout 输出

```bash
tar2oci build \
  --input app.tar \
  --output ./my-app-oci \
  --format oci-layout \
  --image my-app:v1.0
```

### 推送到 Registry

```bash
tar2oci push \
  --input app.tar \
  --image registry.example.com/team/my-app:v1.0 \
  --entrypoint "/app/server" \
  --username admin \
  --password secret
```

从 stdin 读取密码：

```bash
echo "secret" | tar2oci push \
  --input app.tar \
  --image registry.example.com/team/my-app:v1.0 \
  --username admin \
  --password-stdin
```

## 命令参考

### 全局选项

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--config` | `-C` | 配置文件路径 | `./tar2oci.yaml` |
| `--verbose` | `-v` | 详细日志输出 | `false` |
| `--quiet` | `-q` | 静默模式（仅输出错误） | `false` |
| `--help` | `-h` | 显示帮助信息 | - |

### build 命令

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--input` | `-i` | 输入 tar 文件路径（可重复） | 必填 |
| `--output` | `-o` | 输出路径 | 当前目录 |
| `--image` | - | 镜像名称（含 tag） | `image:latest` |
| `--format` | `-f` | 输出格式：`docker-tar` 或 `oci-layout` | `docker-tar` |
| `--base` | `-b` | 基础镜像路径 | - |
| `--platform` | `-p` | 目标平台 `OS/ARCH` | 宿主机架构 |
| `--entrypoint` | `-e` | 容器入口点 | - |
| `--cmd` | `-c` | 默认命令 | - |
| `--env` | - | 环境变量 `KEY=VALUE`（可重复） | - |
| `--workdir` | `-w` | 工作目录 | `/` |
| `--user` | `-u` | 运行用户 `UID:GID` | `root` |
| `--exposed-port` | - | 暴露端口（可重复） | - |
| `--label` | `-l` | 镜像标签 `KEY=VALUE`（可重复） | - |
| `--compression` | - | 压缩算法：`gzip` 或 `zstd` | `gzip` |
| `--stop-signal` | - | 停止信号 | `SIGTERM` |
| `--dry-run` | - | 预览模式，不实际写入 | `false` |

### push 命令

push 命令包含 build 的所有参数，以及：

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--image` | - | 远程镜像地址 | 必填 |
| `--username` | - | 仓库用户名 | - |
| `--password` | - | 仓库密码 | - |
| `--password-stdin` | - | 从 stdin 读取密码 | `false` |

## 配置文件

支持 YAML 格式配置文件，默认查找顺序：

1. `./tar2oci.yaml`（当前目录）
2. `~/.tar2oci/config.yaml`（用户目录）
3. `/etc/tar2oci/config.yaml`（系统目录）

### 配置示例

```yaml
image: registry.example.com/team/my-app:v1.0
entrypoint: ["/app/server"]
cmd: ["--port", "8080"]
workdir: /app
env:
  MODE: production
  LOG_LEVEL: info
exposed_ports:
  - "8080/tcp"
user: "1000:1000"
labels:
  maintainer: "team@example.com"
  version: "1.0.0"
compression: gzip
format: docker-tar
platform: linux/amd64
registries:
  registry.example.com:
    username: admin
    password_file: /secrets/registry-password
```

### 优先级

CLI 参数 > 环境变量 > 配置文件 > 默认值

## 认证

认证凭据按以下优先级查找：

1. 命令行参数 `--username` / `--password`
2. 环境变量 `TAR2OCI_USERNAME` / `TAR2OCI_PASSWORD`
3. Docker 配置文件 `~/.docker/config.json`

## 错误码

| 错误码 | 说明 |
|--------|------|
| E001 | 输入文件不存在 |
| E002 | 无效的 tar 格式 |
| E003 | 权限不足 |
| E004 | 仓库认证失败 |
| E005 | 缺少镜像仓库地址 |
| E006 | 缺少输入文件 |
| E007 | 无效的平台格式 |
| E008 | 基础镜像不存在 |
| E009 | 磁盘空间不足 |
| E010 | 网络超时 |
| E011 | Registry API 错误 |
| E012 | 不支持的压缩算法 |
| E013 | 配置文件解析失败 |
| E014 | 参数冲突 |
| E015 | 层 digest 校验失败 |

## 示例场景

### CI/CD 流水线集成

```yaml
# GitLab CI 示例
build-image:
  stage: package
  script:
    - go build -o app ./cmd/server
    - tar cf app.tar app config/
    - tar2oci push
        --input app.tar
        --image $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG
        --entrypoint "/app"
        --username gitlab-ci-token
        --password $CI_JOB_TOKEN
```

### 多架构镜像构建

```bash
# 构建 amd64 版本
GOARCH=amd64 go build -o app-amd64 ./cmd/server
tar cf app-amd64.tar app-amd64

# 构建 arm64 版本
GOARCH=arm64 go build -o app-arm64 ./cmd/server
tar cf app-arm64.tar app-arm64

# 构建多架构镜像
tar2oci build \
  --input app-amd64.tar --platform linux/amd64 \
  --input app-arm64.tar --platform linux/arm64 \
  --output my-app:v1.0.tar
```

## 输出格式

### Docker Tar

```
my-app_v1.0.tar
├── <config-sha256>.json    # OCI 配置
├── <layer-sha256>.tar.gz   # 层数据
├── manifest.json           # Docker manifest
└── repositories            # 仓库映射
```

### OCI Layout

```
my-app-oci/
├── oci-layout              # OCI 版本声明
├── index.json              # 镜像索引
└── blobs/
    └── sha256/
        ├── <config-digest>    # 配置
        ├── <layer-digest>     # 层数据
        └── <manifest-digest>  # 清单
```

## 依赖

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [image-spec](https://github.com/opencontainers/image-spec) — OCI 镜像规范
- [go-digest](https://github.com/opencontainers/go-digest) — 内容寻址哈希
- [compress](https://github.com/klauspost/compress) — Zstd 压缩支持
- [yaml.v3](https://gopkg.in/yaml.v3) — YAML 配置解析

## 许可证

MIT

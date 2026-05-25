Tar2OCI 项目需求文档 (PRD)
. 项目概述
.1 项目背景
在当前的微服务与云原生开发中，许多开发者习惯将应用程序编译为静态二进制文件，并配合最小化的运行时依赖（如动态链接库、配置文件等）打包成一个 tar 包。传统的容器镜像构建方式（如编写 Dockerfile 并执行 docker build）往往需要依赖 Docker Daemon，流程相对繁琐且存在安全隐患。
.2 项目目标
开发一款名为 Tar2OCI 的轻量级命令行工具。该工具能够读取用户提供的"二进制文件及最小运行环境 tar 包"，结合用户指定的元数据（如启动命令、环境变量等），直接生成符合 OCI（Open Container Initiative）标准的容器镜像 Tar 包，或直接推送到镜像仓库。
.3 核心价值
无依赖构建：无需安装 Docker 或 containerd，无需 Docker Daemon 权限。
高效快速：直接在本地文件系统操作，跳过传统构建上下文传输等耗时步骤。
标准化：产出完全符合 OCI Image Spec 的标准镜像。
. 用户角色与使用场景
.1 目标用户
后端开发工程师（Go/Rust/C++ 等编译型语言开发者）。
DevOps/运维工程师。
CI/CD 流水线架构师。
.2 典型使用场景
场景一（本地打包）：开发者在本地编译好程序并打包成 app.tar，使用 Tar2OCI 将其转换为 app:latest.tar，然后手动分发给测试环境导入。
场景二（CI/CD集成）：在 GitLab CI 或 Jenkins 的无特权容器中，编译程序并打包，直接调用 Tar2OCI 将镜像推送到 Harbor/ACR 等私有仓库。
场景三（多层构建）：开发者将应用依赖（如 glibc、ca-certificates）打包为 deps.tar，应用二进制打包为 app.tar，使用 Tar2OCI 将两个 tar 合并为双层镜像。
场景四（基于基础镜像扩展）：开发者基于已有的 alpine:latest 镜像，叠加自己的应用 tar 包，生成精简镜像。
. 功能需求
.1 核心功能
F1: 基础镜像层导入
工具必须接受一个或多个 .tar 文件作为输入（通过 --input 参数，可重复指定）。
每个 .tar 文件解压后的内容将作为镜像的一个独立文件系统层。
层的顺序由 --input 参数的顺序决定，第一个输入为最底层。
F2: OCI 镜像元数据配置
用户需能通过命令行参数或配置文件指定以下 OCI 配置：
Entrypoint (入口点)
Cmd (默认命令)
WorkDir (工作目录)
Env (环境变量，支持键值对)
ExposedPorts (暴露端口)
User (运行用户)
StopSignal (停止信号，默认 SIGTERM)
Labels (镜像标签，支持键值对)
F3: 镜像导出
支持两种输出格式，通过 --format 参数选择：
docker-tar（默认）：输出 Docker 兼容的 tar 包，可使用 docker load -i 加载。
oci-layout：输出 OCI Image Layout 标准目录结构，可用于 skopeo copy 或直接挂载。
Docker tar 格式结构：
<image-name>/
  manifest.json
  <layer-sha256>.tar.gz
  <config-sha256>.json
  repositories

OCI layout 格式结构：
<output-dir>/
  oci-layout
  index.json
  blobs/sha256/<digest>

F4: 镜像推送
支持直接将生成的镜像推送到远程镜像仓库（支持 HTTP/HTTPS）。
认证方式（按优先级）：
命令行参数 --username / --password
环境变量 TAR2OCI_USERNAME / TAR2OCI_PASSWORD
已有 Docker 配置文件 ~/.docker/config.json
Bearer Token（匿名拉取场景）
敏感参数保护：密码不应出现在进程列表中，支持 --password-stdin 从标准输入读取。
推送支持断点续传：当网络中断时，已上传的 blob 层可复用，无需重新上传。
.2 辅助功能
F5: 镜像标签管理
支持指定镜像的 Repository 和 Tag（例如 my-app:v1.0）。
默认标签策略：
若未指定 --output 且未指定 --image，默认使用 image:latest。
若指定了 --output 但未包含 tag，默认追加 :latest。
若指定了 --image 但未包含 tag，默认追加 :latest。
F6: 压缩算法支持
默认使用 Gzip 压缩层数据。
可选支持 Zstd 压缩，通过 --compression zstd 参数切换。
Zstd 模式生成的镜像需符合 OCI mediaType 规范（application/vnd.oci.image.layer.v1.tar+zstd）。
F7: 多层镜像构建
支持通过多个 --input 参数指定多个 tar 文件，每个 tar 作为独立层叠加。
层顺序：第一个 --input 为最底层，后续依次叠加。
示例：
tar2oci build \
  --input base-deps.tar \
  --input runtime-libs.tar \
  --input app-binary.tar \
  --output my-app:v1.0.tar

F8: 基础镜像支持
支持通过 --base 参数指定已有镜像作为基础镜像。
基础镜像来源：
本地 OCI layout 目录
本地 Docker tar 文件
远程镜像仓库地址（需配合认证）
在基础镜像上叠加的层通过 --input 指定。
示例：
tar2oci build \
  --base alpine:latest \
  --input app.tar \
  --output my-app:v1.0.tar

F9: 平台与架构
支持通过 --platform 参数指定目标平台，格式为 OS/ARCH（如 linux/amd64、linux/arm64）。
默认取宿主机架构（通过 runtime.GOARCH 获取）。
生成的镜像 config 中必须正确设置 architecture 和 os 字段。
支持生成多架构镜像清单（manifest list）：
tar2oci build \
  --input app-amd64.tar --platform linux/amd64 \
  --input app-arm64.tar --platform linux/arm64 \
  --output my-app:v1.0.tar

F10: 输出格式选择
通过 --format 参数控制输出格式：
docker-tar（默认）：兼容 docker load -i
oci-layout：符合 OCI Image Layout 规范
若未指定 --output，默认输出文件名为 <image-name>.tar（docker-tar）或 <image-name>/（oci-layout）。
. 非功能性需求
.1 性能要求
对于 100MB 以内的 tar 包，转换生成时间应控制在秒级。
内存占用应保持低水平，避免将整个 tar 包一次性加载到内存中（应采用流式处理）。
层 digest 计算应采用边写入边计算的方式，避免二次读取。
多层构建时，各层可并行处理以提升效率。
.2 兼容性与标准
生成的镜像必须符合 OCI Image Format Specification v1.0+。
生成的镜像必须能被 Docker、Containerd、Podman、CRI-O 等主流容器运行时加载和运行。
Docker tar 格式必须兼容 docker load -i 命令。
OCI layout 格式必须兼容 skopeo copy 命令。
.3 易用性
提供清晰的命令行帮助信息（--help）。
提供详细的执行日志（支持 --verbose 模式）。
错误提示需准确，例如当 tar 包损坏或仓库认证失败时，应给出明确报错。
支持 --dry-run 模式，预览将要生成的镜像结构但不实际写入。
.4 安全性
密码等敏感信息不得出现在命令行参数列表中（/proc 可见）。
支持从环境变量或文件读取凭证。
临时文件应设置适当的权限（0600），处理完毕后清理。
.5 可靠性
推送过程中网络中断应支持断点续传。
磁盘空间不足时应提前检测并给出明确提示。
层去重：相同内容的层应复用已有 blob，避免重复存储。
. 配置文件规范
.1 文件格式
支持 YAML 格式的配置文件，默认查找顺序：
./tar2oci.yaml（当前目录）
~/.tar2oci/config.yaml（用户目录）
/etc/tar2oci/config.yaml（系统目录）
可通过 --config 参数指定自定义路径。
.2 配置文件示例
# tar2oci.yaml
image: registry.example.com/team/my-app:v1.0
entrypoint: ["/app/server"]
cmd: ["--port", "8080"]
workdir: /app
env:
  MODE: prod
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

.3 优先级规则
CLI 参数 > 环境变量 > 配置文件 > 默认值
相同参数在 CLI 和配置文件中同时存在时，CLI 优先。
. 命令行接口设计
工具名称：tar2oci
基本语法：
tar2oci [global options] command [command options] [arguments...]
全局选项：
--config, -C    指定配置文件路径
--verbose, -v   输出详细日志
--quiet, -q     静默模式，仅输出错误
--help, -h      显示帮助信息
--version       显示版本号
核心命令示例：
# 示例1：将单个 tar 包转换为本地 docker 可加载的 tar 镜像
tar2oci build \
  --input app-env.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server" \
  --cmd "--port 8080" \
  --env "MODE=prod" \
  --env "LOG_LEVEL=info" \
  --workdir "/app"

# 示例2：多层构建
tar2oci build \
  --input base-deps.tar \
  --input app-binary.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server"

# 示例3：基于基础镜像扩展
tar2oci build \
  --base alpine:latest \
  --input app.tar \
  --output my-app:v1.0.tar \
  --entrypoint "/app/server"

# 示例4：输出 OCI layout 格式
tar2oci build \
  --input app.tar \
  --output ./my-app-oci \
  --format oci-layout \
  --image my-app:v1.0

# 示例5：直接推送到远程仓库
tar2oci push \
  --input app-env.tar \
  --image registry.example.com/team/my-app:v1.0 \
  --entrypoint "/app/server" \
  --username admin \
  --password secret

# 示例6：从 stdin 读取密码
echo "secret" | tar2oci push \
  --input app.tar \
  --image registry.example.com/team/my-app:v1.0 \
  --username admin \
  --password-stdin

# 示例7：多架构镜像构建
tar2oci build \
  --input app-amd64.tar --platform linux/amd64 \
  --input app-arm64.tar --platform linux/arm64 \
  --output my-app:v1.0.tar

参数说明：
参数	简写	说明	是否必填	默认值
--input	-i	输入的 tar 包路径（可重复指定）	是	-
--output	-o	输出路径（tar 文件或目录）	否	当前目录
--image	--img	镜像名称（含 tag）	否	image:latest
--format	-f	输出格式：docker-tar 或 oci-layout	否	docker-tar
--base	-b	基础镜像路径或地址	否	-
--platform	-p	目标平台 OS/ARCH	否	宿主机架构
--entrypoint	-e	容器启动入口点	否	-
--cmd	-c	容器默认执行命令	否	-
--env	--env	环境变量 KEY=VALUE（可重复）	否	-
--workdir	-w	容器内工作目录	否	/
--user	-u	运行用户 UID:GID	否	root
--exposed-port	--port	暴露端口（可重复）	否	-
--label	-l	镜像标签 KEY=VALUE（可重复）	否	-
--compression	--comp	压缩算法：gzip 或 zstd	否	gzip
--username	--user	镜像仓库用户名	否	-
--password	--pass	镜像仓库密码	否	-
--password-stdin	-	从 stdin 读取密码	否	-
--stop-signal	-	容器停止信号	否	SIGTERM
--dry-run	-	预览模式，不实际写入	否	false
. 异常处理与错误码
.1 错误码定义
错误码	错误信息	说明
E001	Error: input file not found: <path>	输入文件不存在
E002	Error: invalid tar format: <path>	输入文件非有效 tar 格式
E003	Error: permission denied: <path>	文件写入权限不足
E004	Error: auth failed: <registry>	仓库认证失败
E005	Error: missing image repository	缺少镜像仓库地址
E006	Error: missing input files	未指定任何输入文件
E007	Error: invalid platform format: <platform>	平台格式错误（应为 OS/ARCH）
E008	Error: base image not found: <path>	基础镜像不存在
E009	Error: disk space insufficient	磁盘空间不足
E010	Error: network timeout: <registry>	网络连接超时
E011	Error: registry API error: <status>	仓库 API 返回错误
E012	Error: unsupported compression: <algo>	不支持的压缩算法
E013	Error: config file parse error: <path>	配置文件解析失败
E014	Error: conflicting parameters	参数冲突（如同时指定 --base 和多个 --input 作为底层）
E015	Error: layer digest mismatch	层 digest 校验失败

.2 错误处理原则
所有错误必须输出到 stderr。
错误信息必须包含具体的上下文（如文件路径、仓库地址）。
非零退出码：成功退出码为 0，错误退出码为 1。
网络错误支持自动重试（最多 3 次，指数退避）。
. 验收标准
.1 功能验收
使用 Tar2OCI 生成的镜像，可以通过 docker load -i <image>.tar 成功加载。
加载后的镜像，运行容器（docker run）能正确执行预设的 Entrypoint 和 Cmd。
容器内的环境变量、工作目录与设定一致。
工具可以在无 Docker 环境的纯净 Linux 虚拟机中成功运行并产出镜像文件。
.2 多层验收
使用多个 --input 生成的镜像，各层内容独立且顺序正确。
相同内容的层在多次构建中产生相同的 digest。
.3 基础镜像验收
基于 alpine:latest 生成的镜像，运行时可使用 alpine 的工具（如 sh、ls）。
.4 输出格式验收
docker-tar 格式可被 docker load -i 正确加载。
oci-layout 格式可被 skopeo copy 正确复制到 registry。
.5 推送验收
推送到 Harbor/ACR 等私有仓库成功。
网络中断后重新推送，已上传的层可复用。
.6 平台验收
指定 --platform linux/arm64 生成的镜像，config 中 architecture 为 arm64。
. 附录
.1 术语表
OCI: Open Container Initiative，容器镜像格式标准。
Layer: 镜像层，容器文件系统的一个增量修改。
Digest: 内容寻址的哈希值，用于唯一标识一个 blob。
Blob: OCI 存储中的不可变数据对象。
Manifest: 描述镜像结构的 JSON 文档。
Config: 镜像配置，包含入口点、环境变量等元数据。
.2 参考文档
OCI Image Specification: https://github.com/opencontainers/image-spec
OCI Distribution Specification: https://github.com/opencontainers/distribution-spec
Docker Image Format: https://docs.docker.com/registry/spec/manifest-v2-2/

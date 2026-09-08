# macOS 编译说明（CGO 绑定）

> 说明：当前版本不再从源码编译静态库，而是直接使用 alibaba/zvec 官方
> release 提供的**自包含 C API 动态库**（`libzvec_c_api.dylib`），
> 里面已经包含完整的 C++ 核心和全部第三方依赖。macOS 上**无需 CMake、
> 无需编译 zvec 源码**。

## 目录结构

```
lib/
├── linux-x86_64/libzvec_c_api.so     # Linux x86_64
├── linux-arm64/libzvec_c_api.so      # Linux ARM64
├── macos-arm64/libzvec_c_api.dylib   # macOS Apple Silicon
└── data/jieba_dict/                  # FTS 分词数据（jieba）
    ├── jieba.dict.utf8
    └── hmm_model.utf8
```

文件名必须保持 `libzvec_c_api.{so,dylib}`（与 SONAME 一致），
cgo 链接规则直接链接该文件并把 rpath 指到所在子目录，
运行时无需设置 `DYLD_LIBRARY_PATH`。

## 使用 / 验证（本地）

```bash
# 构建 CGO 绑定（需要 clang，macOS 自带）
CGO_ENABLED=1 go build -tags 'cgo zvec_cgo' ./cgo/

# 运行 CGO 测试
CGO_ENABLED=1 go test -tags 'cgo zvec_cgo' -v ./cgo/
```

检查 dylib 架构：

```bash
file lib/macos-arm64/libzvec_c_api.dylib
# 应显示: Mach-O 64-bit arm64 dynamically linked shared library
```

## 更新库文件（跟随 zvec 子模块版本）

当前仓库把 zvec 子模块固定到某个版本（目前 **v0.7.0**）。
当子模块升级到新版本时，需要下载对应 release 的预编译 SDK：

1. 确认固定版本：
   ```bash
   cd zvec && git describe --tags --exact-match HEAD
   ```
2. 从 `https://github.com/alibaba/zvec/releases` 下载对应 tag 的
   `zvec-sdk-osx-arm64.tar.gz`（Apple Silicon）。
3. 解压后把 `libzvec_c_api.dylib` 放到
   `lib/macos-arm64/libzvec_c_api.dylib`（如 FTS 数据有更新，
   同步 `lib/data/jieba_dict/`）。
4. 运行上面的 CGO 测试验证后提交。

仓库中的
[`build-zvec-native-libraries.yml`](../.github/workflows/build-zvec-native-libraries.yml)
GitHub Actions 工作流会自动完成上述步骤（下载预编译 SDK → 提交到
`lib/`），当 `zvec/` 子模块或工作流文件变化时自动触发。

## 故障排除

### 链接错误：找不到 `libzvec_c_api`

确认 `lib/macos-arm64/libzvec_c_api.dylib` 存在，且文件名没有
平台后缀：

```bash
ls lib/macos-arm64/
grep -A 3 "#cgo darwin" cgo/collection_cgo.go
```

### 运行时报 dyld 加载失败

cgo 已经把 rpath 写进了产物；若仍失败，可临时设置
`DYLD_LIBRARY_PATH=$(pwd)/lib/macos-arm64` 排查。

### 使用了 FTS 时提示找不到 jieba 词典

```bash
export ZVEC_JIEBA_DICT_DIR=$(pwd)/lib/data/jieba_dict
```

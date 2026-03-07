# macOS 编译说明

## 使用 GitHub Actions (推荐)

最简单的方式是通过 GitHub Actions 自动编译:

1. 推送代码到 main 分支
2. 或手动在 GitHub Actions 中触发 "Build zvec Static Libraries"
3. 编译完成后会自动提交到 lib/ 目录

## 本地编译

### 前置条件

```bash
# 安装 CMake
brew install cmake
```

### 编译步骤

```bash
# 1. 初始化子模块 (首次需要，耗时较长)
git submodule update --init --recursive

# 2. 运行构建脚本
./build-macos.sh
```

### 手动编译

如果构建脚本失败，可以手动编译:

```bash
# 1. 进入 zvec 目录
cd zvec

# 2. 创建构建目录
mkdir -p build && cd build

# 3. 配置 CMake (Apple Silicon)
cmake -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
      -DCMAKE_OSX_ARCHITECTURES=arm64 \
      ..

# 对于 Intel Mac，使用:
# cmake -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
#       -DCMAKE_OSX_ARCHITECTURES=x86_64 \
#       ..

# 4. 编译
make -j$(sysctl -n hw.ncpu)

# 5. 复制库文件到 lib/ 目录
cp lib/libzvec_core.a ../../lib/libzvec_core-macos-arm64.a
cp lib/libzvec_ailego.a ../../lib/libzvec_ailego-macos-arm64.a
```

## 库文件命名

编译完成后，lib/ 目录应该包含:

```
lib/
├── libzvec_core-linux-x86_64.a     # Linux x86_64
├── libzvec_ailego-linux-x86_64.a   # Linux x86_64
├── libzvec_core-macos-arm64.a      # macOS Apple Silicon
├── libzvec_ailego-macos-arm64.a    # macOS Apple Silicon
├── libzvec_core-macos-x86_64.a     # macOS Intel (可选)
└── libzvec_ailego-macos-x86_64.a   # macOS Intel (可选)
```

## 验证编译

```bash
# 检查库文件架构
file lib/libzvec_core-macos-*.a

# 应该显示:
# libzvec_core-macos-arm64.a: Mach-O 64-bit arm64
# libzvec_core-macos-x86_64.a: Mach-O 64-bit x86_64
```

## 使用 Go 测试

```bash
# 测试 CGO 链接
go build ./...

# 运行测试
go test ./...
```

## 故障排除

### 问题：CMake 版本太低

```bash
brew upgrade cmake
```

### 问题：子模块初始化失败

```bash
# 尝试使用更浅的克隆深度
git submodule update --init --recursive --depth 1
```

### 问题：编译时找不到头文件

确保子模块已正确初始化:
```bash
ls zvec/src/include
```

### 问题：链接错误

确保库文件命名正确，并且 CGO 配置正确:
```bash
# 检查 cgo/collection_cgo.go 中的链接配置
grep -A 5 "#cgo darwin" cgo/collection_cgo.go
```

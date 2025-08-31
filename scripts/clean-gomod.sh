#!/bin/bash

# 清理 go.mod 中的私有包引用
# 用于在提交代码前确保 go.mod 不包含私有依赖

cd `dirname $0`
cd ../

echo "Cleaning private dependencies from go.mod..."

# 移除私有包引用
sed -i.bak '/github.com\/quka-ai\/commercial/d' go.mod
sed -i.bak '/github.com\/quka-ai\/commercial/d' go.sum

# 删除备份文件
rm -f go.mod.bak go.sum.bak

echo "Cleaned go.mod and go.sum files"
echo "Private packages removed:"
echo "  - github.com/quka-ai/commercial"
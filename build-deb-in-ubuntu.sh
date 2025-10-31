#!/bin/bash
version=1.5
BAK_TARGET_DIR=build/linux/wdreader_0.0.0_amd64_bak
INIT_DIR=build/linux/wdreader_0.0.0_amd64
INIT_VERSION=$(jq -r '.info.productVersion' wails.json)
rm -rf "$BAK_TARGET_DIR"
rm -rf build/linux/wdreader_*_amd64.deb
cp -r $INIT_DIR $BAK_TARGET_DIR
content=$(cat "${INIT_DIR}/DEBIAN/control")
#echo $content
content=$(echo "$content" | sed -e "s/{{.Name}}/$(jq -r '.name' wails.json)/g")
content=$(echo "$content" | sed -e "s/{{.Info.ProductVersion}}/$(jq -r '.info.productVersion' wails.json)/g")
content=$(echo "$content" | sed -e "s/{{.Author.Name}}/$(jq -r '.author.name' wails.json)/g")
content=$(echo "$content" | sed -e "s/{{.Author.Email}}/$(jq -r '.author.email' wails.json)/g")
content=$(echo "$content" | sed -e "s/{{.Info.Comments}}/$(jq -r '.info.comments' wails.json)/g")
echo $content
echo "$content" > "${INIT_DIR}/DEBIAN/control"
content=$(cat "${INIT_DIR}/usr/share/applications/wdreader.desktop")
content=$(echo "$content" | sed -e "s/{{.Info.ProductName}}/$(jq -r '.info.productName' wails.json)/g")
content=$(echo "$content" | sed -e "s/{{.Info.Comments}}/$(jq -r '.info.comments' wails.json)/g")
echo $content
echo "$content" > "${INIT_DIR}/usr/share/applications/wdreader.desktop"
if [ -a "build/bin/wdreader" ]; then
  echo "文件存在"
else
  echo "文件不存在开始执行构建"
  CGO_ENABLED=1 wails build -clean -platform linux/amd64 -ldflags "-X main.version=${version}" -o wdreader
fi
mv build/bin/wdreader "${INIT_DIR}/usr/local/bin/"
cd build/linux
mv wdreader_0.0.0_amd64 "wdreader_${version}_amd64"
sed -i 's/'$INIT_VERSION'/'$version'/g' "wdreader_${version}_amd64/DEBIAN/control"
dpkg-deb --build -Zxz "wdreader_${version}_amd64"
cd ../../
rm -rf "build/linux/wdreader_${version}_amd64"
mv $BAK_TARGET_DIR $INIT_DIR
echo "构建成功产物在：build/linux/wdreader_${version}_amd64.deb"
mkdir -p "$out/app"
ln -s "$fakeBrowser" "$out/app/fake-browser"
cp "$fakeDesktop" "$out/app/fake-browser.desktop"
cp "$fakeIcon" "$out/app/product_logo_256.png"

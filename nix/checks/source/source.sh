cp -R "$src" source
chmod -R u+w source

unformatted="$(find source -name '*.go' -type f -print0 | xargs -0 gofmt -l)"
if [ -n "$unformatted" ]; then
  echo "Go files need formatting:" >&2
  echo "$unformatted" >&2
  exit 1
fi

(
  cd source
  biome format .
  treefmt --ci --walk filesystem --tree-root "$PWD"
  deadnix --fail .

  statixOutput="$(statix check . 2>&1)"
  if [ -n "$statixOutput" ]; then
    echo "$statixOutput" >&2
    exit 1
  fi
)
touch "$out"

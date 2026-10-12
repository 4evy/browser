check-jsonschema \
  --schemafile "$src/schema/storage/storage.json" \
  "$src/testdata/storage/storage.json"
touch "$out"

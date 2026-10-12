"$verificationExecutable" \
  "$defaultFile" \
  "$namedFile" \
  "$expectedDefaultSettings" \
  "$expectedNamedSettings" \
  "$managedPolicyFile" \
  "$expectedManagedPolicy"

for configFile in "$defaultFile" "$namedFile"; do
  profile="$TMPDIR/profile-$(basename "$configFile" .toml)"
  "$tweezleExecutable" profile apply "$configFile" \
    --profile-dir "$profile"
  if [ "$(basename "$configFile")" = "tweezle.toml" ]; then
    test -d "$profile/Local Extension Settings/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  fi
done

touch "$out"

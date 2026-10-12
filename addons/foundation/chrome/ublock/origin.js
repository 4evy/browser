// The dashboard uses vAPI.messaging to reach the background settings API
// https://github.com/gorhill/uBlock/blob/master/src/js/messaging.js
const send = async (what, args = {}) => {
  if (!globalThis.vAPI?.messaging?.send) {
    throw new Error("uBlock Origin dashboard messaging is unavailable");
  }
  return vAPI.messaging.send("dashboard", { what, ...args });
};
while (!(await send("readyToFilter"))) {
  await new Promise((resolve) => setTimeout(resolve, 100));
}
const currentUser = await send("userSettings");
const currentHidden = await send("readHiddenSettings");
for (const key of Object.keys(settings.user_settings ?? {})) {
  if (!Object.hasOwn(currentUser, key)) {
    throw new Error(`Unknown uBlock Origin user setting: ${key}`);
  }
  if (
    typeof settings.user_settings[key] !== typeof currentUser[key] ||
    Array.isArray(settings.user_settings[key]) !==
      Array.isArray(currentUser[key])
  ) {
    throw new Error(`Invalid type for uBlock Origin user setting: ${key}`);
  }
}
for (const key of Object.keys(settings.hidden_settings ?? {})) {
  if (!Object.hasOwn(currentHidden.default, key)) {
    throw new Error(`Unknown uBlock Origin hidden setting: ${key}`);
  }
  if (
    typeof settings.hidden_settings[key] !== typeof currentHidden.default[key]
  ) {
    throw new Error(`Invalid type for uBlock Origin hidden setting: ${key}`);
  }
}
for (const [name, value] of Object.entries(settings.user_settings ?? {})) {
  await send("userSettings", { name, value });
}
if (settings.hidden_settings) {
  const values = { ...currentHidden.current, ...settings.hidden_settings };
  const content = Object.entries(values)
    .map(([name, value]) => `${name} ${value}`)
    .join("\n");
  await send("writeHiddenSettings", { content });
}
if (settings.filter_lists) {
  const imported = settings.filter_lists.filter((key) =>
    /^https?:\/\//.test(key),
  );
  await send("applyFilterListSelection", {
    toSelect: settings.filter_lists,
    toImport: imported.join("\n"),
    merge: false,
  });
}
if (settings.trusted_sites) {
  await send("setWhitelist", { whitelist: settings.trusted_sites.join("\n") });
}
if (settings.rules !== undefined) {
  const current = await send("getRules");
  await send("modifyRuleset", {
    permanent: true,
    toRemove: current.permanentRules.join("\n"),
    toAdd: settings.rules,
  });
}
if (settings.user_filters !== undefined) {
  const current = await send("readUserFilters");
  const result = await send("writeUserFilters", {
    content: settings.user_filters,
    enabled: settings.filter_lists
      ? settings.filter_lists.includes("user-filters")
      : current.enabled,
    trusted: current.trusted,
  });
  if (result?.error) throw new Error(result.error);
}
await send("reloadAllFilters");
// Read storage after the settings API calls so asynchronous writes are flushed
await chrome.storage.local.get(null);

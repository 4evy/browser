import path from "node:path";
import vm from "node:vm";
import { declarations, parseSource } from "./literals.mts";

type Values = Record<string, any>;

export async function ghosteryConfiguration(repository: string) {
  const files: string[] = [];
  const recordSchemas: Values = {};
  const context = {
    __CHROMIUM__: true,
    __FIREFOX__: false,
    isWebkit: () => false,
    MODE_DEFAULT: "default",
    DEFAULT_REGIONS: ["bg"],
    String: "__STRING_ARRAY_TYPE__",
    store: {
      connect: Symbol("connect"),
      record: (value: unknown) => ({ $record: value }),
    },
  };
  function normalize(value: any, key: string): any {
    if (Array.isArray(value)) {
      if (value.length === 1 && value[0] === context.String) return [];
      return value.map((member, index) => normalize(member, `${key}/${index}`));
    }
    if (value !== null && typeof value === "object") {
      if (Object.hasOwn(value, "$record")) {
        recordSchemas[key] = normalize(value.$record, `${key}/*`);
        return {};
      }
      return Object.fromEntries(
        Object.entries(value).map(([name, member]) => [
          name,
          normalize(member, `${key}/${name}`),
        ]),
      );
    }
    return value;
  }
  async function declaration(file: string, name: string): Promise<any> {
    files.push(file);
    const source = await Bun.file(path.join(repository, file)).text();
    const node = declarations(parseSource(source)).get(name);
    if (!node) throw new Error(`${file}: missing ${name}`);
    // Only instantiate the reviewed data declaration; storage callbacks stay idle
    const value = vm.runInNewContext(
      `JSON.stringify((${source.slice(node.start, node.end)}))`,
      context,
      { timeout: 1000 },
    );
    return normalize(JSON.parse(value), name);
  }
  const options = await declaration("src/store/options.js", "Options");
  const version = await declaration("src/store/options.js", "OPTIONS_VERSION");
  const local: Values = {
    options,
    optionsVersion: version,
    customFilters: await declaration(
      "src/store/custom-filters.js",
      "CustomFilters",
    ),
    elementPickerSelectors: await declaration(
      "src/store/element-picker-selectors.js",
      "ElementPickerSelectors",
    ),
    disabledFilters: await declaration(
      "src/store/disabled-filters.js",
      "DisabledFilters",
    ),
  };
  const defaults = structuredClone(local);
  options.regionalFilters = { enabled: true, regions: ["bg", "de"] };
  options.theme = "dark";
  options.panel.statsType = "list";
  options.customFilters.enabled = true;
  options.customFilters.filterLists["https://example.com/filters.txt"] = {
    ...recordSchemas["Options/customFilters/filterLists"],
    enabled: false,
  };
  options.redirectProtection.exceptions["example.com"] = true;
  options.exceptions["sample-tracker-id"] = {
    ...recordSchemas["Options/exceptions"],
    domains: ["example.com"],
  };
  options.paused["example.org"] = structuredClone(
    recordSchemas["Options/paused"],
  );
  options.zapped["example.net"] = false;
  const distractionsFile = "src/background/rule_resources/distractions.json";
  files.push(distractionsFile);
  const distractions = await Bun.file(
    path.join(repository, distractionsFile),
  ).json();
  options.distractions = Object.fromEntries(
    Object.keys(distractions)
      .sort()
      .map((id) => [id, false]),
  );
  local.customFilters.text =
    "! Hypothetical sample filters\n||ads.example.com^\nexample.com##.sample-ad\n";
  local.customFilters.filterLists["https://example.com/filters.txt"] = {
    ...recordSchemas["CustomFilters/filterLists"],
    name: "Disabled sample list",
  };
  local.elementPickerSelectors.hostnames["example.com"] = [
    ".sample-banner",
    "#sample-popup",
  ];
  local.disabledFilters.ids["sample-filter-id"] = false;
  // Source record templates add paths that empty dictionary defaults cannot expose
  defaults.options = structuredClone(options);
  defaults.customFilters.filterLists = structuredClone(
    local.customFilters.filterLists,
  );
  defaults.elementPickerSelectors = structuredClone(
    local.elementPickerSelectors,
  );
  defaults.disabledFilters = structuredClone(local.disabledFilters);
  const localOnly: string[] = await declaration(
    "src/store/options.js",
    "LOCAL_OPTIONS",
  );
  const sync = {
    options: Object.fromEntries(
      Object.entries(options).filter(
        ([key]) => key === "revision" || !localOnly.includes(key),
      ),
    ),
  };
  files.push(
    "src/background/sync.js",
    "src/utils/regions.js",
    "src/utils/storage.js",
    "src/store/managed-config.js",
    "src/managed_storage.json",
  );
  const managedSchema = await Bun.file(
    path.join(repository, "src/managed_storage.json"),
  ).json();
  const managed = {
    disableOnboarding: false,
    disableUserControl: false,
    disableUserAccount: true,
    disableTrackersPreview: false,
    trustedDomains: ["example.org"],
    customFilters: ["! Hypothetical managed filter", "||ads.example.org^"],
  };
  if (
    JSON.stringify(Object.keys(managed).sort()) !==
    JSON.stringify(Object.keys(managedSchema.properties).sort())
  )
    throw new Error("Ghostery managed policy coverage changed");
  return {
    local,
    sync,
    defaults,
    files: [...new Set(files)],
    recordSchemas,
    managed,
  };
}

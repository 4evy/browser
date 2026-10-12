#!/usr/bin/env bun
// Evaluate only reviewed default declarations, never extension entry points
import path from "node:path";
import vm from "node:vm";
import LZString from "lz-string";
import { resConfiguration } from "./res.mts";
import { ghosteryConfiguration } from "./ghostery.mts";
import { stylusStyles, stylusScript } from "./stylus.mts";
import { checkSchemas } from "./schemas.mts";

// Reviewed upstream declarations have extension-specific, evolving schemas
type Settings = Record<string, any>;
type Areas = Record<string, Settings>;
type Coverage = {
  declared_keys: string[];
  configured_keys: string[];
  declared_paths: string[];
  configured_paths: string[];
  container?: string;
};
type Review = {
  slug: string;
  id: string;
  sample: string;
  repository: string;
  commit: string;
  source_urls: string[];
  coverage: Record<string, Coverage>;
  edge_cases: string[];
  limitations: string[];
  positional_tables?: { module: string; option: string; fields: string[] }[];
  filter_cases?: Record<string, Settings[]>;
  text_formats?: Record<string, string[]>;
  field_schemas?: Record<
    string,
    { declared_fields: string[]; configured_fields: string[] }
  >;
  record_schemas?: Settings;
  managed_policy_sample?: string;
  split_values?: { plain_sample: string; area: string };
  runtime_sample?: {
    config: string;
    schemas: Record<string, string[]>;
    excluded_fields: Record<string, string[]>;
  };
  compressed_templates?: {
    plain_sample: string;
    encoding: string;
    keys: string[];
  };
};

const root = import.meta.dir;
const repos = path.resolve(root, "../../../.sources/extension-research/repos");
const transpiler = new Bun.Transpiler({ loader: "ts" });
const reviews: Review[] = [];
const read = (repo: string, file: string): Promise<string> =>
  Bun.file(path.join(repos, repo, file)).text();

function paths(value: unknown, prefix = ""): string[] {
  const result = [prefix];
  if (Array.isArray(value)) {
    for (const member of value) result.push(...paths(member, `${prefix}/*`));
  } else if (value !== null && typeof value === "object") {
    for (const [key, member] of Object.entries(value)) {
      const token = key.replaceAll("~", "~0").replaceAll("/", "~1");
      result.push(...paths(member, `${prefix}/${token}`));
    }
  }
  return [...new Set(result)].sort();
}

function evaluate<T = Settings>(
  source: string,
  expression: string,
  context: Record<string, unknown> = {},
): T {
  const code = transpiler.transformSync(source.replaceAll("export ", ""));
  return JSON.parse(
    vm.runInNewContext(`${code}\nJSON.stringify(${expression})`, context, {
      timeout: 1000,
    }),
  );
}

function snippet(source: string, start: string, end: string): string {
  const begin = source.indexOf(start);
  const finish = source.indexOf(end, begin);
  if (begin < 0 || finish < 0)
    throw new Error(`Missing reviewed anchors: ${start}`);
  return source.slice(begin, finish);
}

async function save(
  slug: string,
  repo: string,
  repository: string,
  id: string,
  areas: Areas,
  defaults: Areas,
  files: string[],
  edges: string[],
  limitations: string[],
) {
  const dir = path.resolve(root, "..", slug);
  const document: Record<string, unknown> = {
    schema_version: 1,
    name: `${slug}: all source-declared defaults with representative custom values`,
    description:
      "Offline profile sample; close the browser and use a disposable profile",
  };
  for (const [area, values] of Object.entries(areas))
    document[area] = [{ id, values }];
  await Bun.write(
    path.join(dir, `${slug}.json`),
    JSON.stringify(document, null, 2) + "\n",
  );
  const revision = Bun.spawnSync({
    cmd: ["git", "-C", path.join(repos, repo), "rev-parse", "HEAD"],
    stdout: "pipe",
    stderr: "pipe",
  });
  if (revision.exitCode !== 0)
    throw new Error(
      `Cannot read ${repo} revision: ${revision.stderr.toString()}`,
    );
  const commit = revision.stdout.toString().trim();
  const coverage: Record<string, Coverage> = {};
  for (const [area, values] of Object.entries(defaults)) {
    const declared = Object.keys(values).sort();
    const configured = Object.keys(areas[area]).sort();
    const missing = declared.filter((key) => !configured.includes(key));
    if (missing.length)
      throw new Error(`${slug}: missing ${missing.join(", ")}`);
    const declaredPaths = paths(values);
    const configuredPaths = paths(areas[area]);
    const missingPaths = declaredPaths.filter(
      (key) => !configuredPaths.includes(key),
    );
    if (missingPaths.length)
      throw new Error(
        `${slug}: missing nested fields ${missingPaths.join(", ")}`,
      );
    coverage[area] = {
      declared_keys: declared,
      configured_keys: configured,
      declared_paths: declaredPaths,
      configured_paths: configuredPaths,
    };
  }
  reviews.push({
    slug,
    id,
    sample: `${slug}/${slug}.json`,
    repository,
    commit,
    source_urls: files.map((file) => `${repository}/blob/${commit}/${file}`),
    coverage,
    edge_cases: edges,
    limitations,
  });
}

let source = await read("darkreader", "src/defaults.ts");
const dark = evaluate(
  source.slice(source.indexOf("export const DEFAULT_COLORS")),
  "DEFAULT_SETTINGS",
  {
    isMacOS: true,
    isWindows: false,
    isCSSColorSchemePropSupported: true,
    isEdge: false,
    isMobile: false,
    __CHROMIUM_MV3__: true,
    __PLUS__: false,
    ThemeEngine: { dynamicTheme: "dynamicTheme", cssFilter: "cssFilter" },
    AutomationMode: { SYSTEM: "system", NONE: "" },
  },
);
const darkCustom = structuredClone(dark);
const darkFixes: Settings = {};
const darkTextFormats: Record<string, string[]> = {};
const parseHelpers = snippet(
  await read("darkreader", "src/generators/utils/parse.ts"),
  "export function parseSitesFixesConfig",
  "// URL patterns",
);
const textHelpers = snippet(
  await read("darkreader", "src/utils/text.ts"),
  "export function parseArray",
  "export function formatArray",
);
for (const [key, file, declaration, parser, formatter] of [
  [
    "dev_dynamic_theme_fixes",
    "dynamic-theme",
    "dynamicThemeFixesCommands",
    "parseDynamicThemeFixes",
    "formatDynamicThemeFixes",
  ],
  [
    "dev_inversion_fixes",
    "css-filter",
    "inversionFixesCommands",
    "parseInversionFixes",
    "formatInversionFixes",
  ],
  [
    "dev_static_themes",
    "static-theme",
    "staticThemeCommands",
    "parseStaticThemes",
    "formatStaticThemes",
  ],
  [
    "dev_detector_hints",
    "detector-hints",
    "detectorHintsCommands",
    "parseDetectorHints",
    "formatDetectorHints",
  ],
]) {
  const source = await read("darkreader", `src/generators/${file}.ts`);
  const declarations = snippet(
    source,
    `const ${declaration}`,
    `export function ${formatter}`,
  );
  const commands = evaluate(declarations, declaration);
  const text = [
    "example.com",
    "",
    ...Object.keys(commands).flatMap((command) => [
      command,
      command === "CSS"
        ? ".sample { color: #eeeeee !important; }"
        : ["NO COMMON", "NO DARK THEME", "SYSTEM THEME"].includes(command)
          ? ""
          : ".sample",
      "",
    ]),
  ].join("\n");
  const parsed = evaluate<Settings[]>(
    `${textHelpers}\n${parseHelpers}\n${declarations}`,
    `${parser}(text)`,
    { text },
  );
  const fields = Object.values(commands) as string[];
  if (
    parsed.length !== 1 ||
    fields.some((key) => !Object.hasOwn(parsed[0], key))
  )
    throw new Error(`Incomplete Dark Reader ${file} fixes`);
  darkFixes[key] = text;
  darkTextFormats[key] = fields;
}
Object.assign(darkCustom, {
  schemeVersion: 2,
  syncSettings: true,
  enabledFor: ["example.com"],
  disabledFor: ["mail.example.org"],
  automation: { enabled: true, mode: "time", behavior: "OnOff" },
  time: { activation: "18:30", deactivation: "07:15" },
  location: { latitude: 42.6977, longitude: 23.3219 },
});
darkCustom.theme.stylesheet =
  'a[href*="example"] { text-decoration: underline; }\n';
darkCustom.presets = [
  {
    id: "sample-preset",
    name: "Sofia evening",
    urls: ["example.com"],
    theme: structuredClone(darkCustom.theme),
  },
];
darkCustom.customThemes.push({
  url: ["example.org"],
  theme: { ...darkCustom.theme, brightness: 95 },
  builtIn: false,
});
const darkSyncPlain = structuredClone(darkCustom);
darkSyncPlain.theme.stylesheet = `/* ${"sample ".repeat(1800)} */`;
const syncProtocol = await read(
  "darkreader",
  "src/background/utils/extension-api.ts",
);
const darkSync = evaluate(
  snippet(
    syncProtocol,
    "function prepareSyncStorage",
    "export async function writeSyncStorage",
  ),
  "prepareSyncStorage(sample)",
  {
    sample: structuredClone(darkSyncPlain),
    // Reserve room for JSON escaping that upstream's character count omits
    chrome: { storage: { sync: { QUOTA_BYTES_PER_ITEM: 7900 } } },
  },
);
for (const [key, value] of Object.entries(darkSync)) {
  if (Buffer.byteLength(key + JSON.stringify(value), "utf8") > 8192)
    throw new Error(`Dark Reader sync item exceeds 8192 bytes: ${key}`);
}
const validator = (await read("darkreader", "src/utils/validation.ts")).replace(
  /^import .*\n/gm,
  "",
);
const validation = evaluate<{ errors: string[] }>(
  validator,
  "validateSettings(sample)",
  {
    DEFAULT_SETTINGS: dark,
    DEFAULT_THEME: dark.theme,
    sample: structuredClone(darkCustom),
    AutomationMode: {
      NONE: "",
      TIME: "time",
      SYSTEM: "system",
      LOCATION: "location",
    },
  },
);
if (validation.errors.length)
  throw new Error(
    `Dark Reader rejects sample: ${validation.errors.join(", ")}`,
  );
await save(
  "darkreader",
  "darkreader",
  "https://github.com/darkreader/darkreader",
  "eimadpbcbfnmbkopoojfekhnkhdbieeh",
  { local: { ...darkCustom, ...darkFixes }, sync: darkSync },
  { local: { ...dark, ...darkFixes }, sync: darkSync },
  [
    "src/defaults.ts",
    "src/background/user-storage.ts",
    "src/background/utils/extension-api.ts",
    "src/utils/validation.ts",
    "src/background/devtools.ts",
    "src/generators/dynamic-theme.ts",
    "src/generators/css-filter.ts",
    "src/generators/static-theme.ts",
    "src/generators/detector-hints.ts",
    "src/generators/utils/parse.ts",
    "src/utils/text.ts",
  ],
  [
    "Schema version 2 prevents migration of site lists",
    "Local syncSettings=true selects the sync snapshot",
    "All theme fields repeated in presets and custom themes",
    "Sync splits large values into base-36 chunks with __meta_split_count",
    "All commands in the four developer override text formats are checked by upstream parsers",
  ],
  [
    "Free MV3 source defaults evaluated for macOS; Plus-only fields excluded",
    "Sync encoder uses a conservative 7900-character budget to allow JSON escaping within Chromium's 8192-byte quota",
    "Source revision may differ from the store package",
  ],
);
reviews.at(-1)!.text_formats = darkTextFormats;
reviews.at(-1)!.field_schemas = checkSchemas(
  await read("darkreader", "src/definitions.d.ts"),
  {
    UserSettings: [darkCustom],
    Theme: [darkCustom.theme],
    ThemePreset: darkCustom.presets,
    CustomSiteConfig: darkCustom.customThemes,
    Automation: [darkCustom.automation],
    TimeSettings: [darkCustom.time],
    LocationSettings: [darkCustom.location],
  },
);
reviews
  .at(-1)!
  .source_urls.push(
    `${reviews.at(-1)!.repository}/blob/${reviews.at(-1)!.commit}/src/definitions.d.ts`,
  );
await Bun.write(
  path.resolve(root, "../darkreader/sync-settings.json"),
  JSON.stringify(darkSyncPlain, null, 2) + "\n",
);
reviews.at(-1)!.split_values = {
  plain_sample: "darkreader/sync-settings.json",
  area: "sync",
};

source = await read("stylus", "src/js/prefs.js");
const constants = (await read("stylus", "src/js/consts.js")).match(
  /^export const .*;.*$/gm,
);
if (!constants) throw new Error("Missing reviewed Stylus declarations");
const declarations = constants.join("\n");
const stylus = evaluate(
  declarations +
    "\n" +
    snippet(source, "const defaults =", "const warnUnknown"),
  "defaults",
);
const stylusCustom = structuredClone(stylus);
stylusCustom["editor.beautify"].newline_between_rules = true;
stylusCustom["editor.colorpicker.color"] = "#a6e3a1";
stylusCustom["editor.options"] = {
  cursorBlinkRate: 530,
  showCursorWhenSelecting: true,
};
stylusCustom.windowPosition = { left: 40, top: 50, width: 1100, height: 800 };
await save(
  "stylus",
  "stylus",
  "https://github.com/openstyles/stylus",
  "clngdbkpkpeebahjckkjfobafhncgmne",
  { sync: { settings: stylusCustom } },
  { sync: { settings: stylus } },
  [
    "src/js/prefs.js",
    "src/js/consts.js",
    "src/background/prefs-api.js",
    "src/.types.d.ts",
    "src/background/db.js",
    "src/background/style-manager/index.js",
    "src/background/usercss-manager.js",
    "src/js/msg-api.js",
  ],
  [
    "Dotted preference names are literal object keys under sync.settings",
    "Computed names resolved before coverage",
    "Nested editor options and beautifier configuration",
    "8 KiB per-item sync quota covers the entire settings object",
  ],
  [
    "User styles use the separate runtime.toml recipe and require a matching unpacked extension",
    "Drafts, OAuth credentials, remote publishing tokens, and generated engine caches are application state",
    "Firefox-only hotkeys are explicit inert defaults on Chromium",
    "Source revision may differ from the store package",
  ],
);
// Record inner preferences rather than only the wrapping settings key
reviews.at(-1)!.coverage.sync.declared_keys = Object.keys(stylus).sort();
reviews.at(-1)!.coverage.sync.configured_keys =
  Object.keys(stylusCustom).sort();
reviews.at(-1)!.coverage.sync.container = "settings";
const styleSamples = await stylusStyles(path.join(repos, "stylus"));
await Bun.write(
  path.resolve(root, "../stylus/styles.json"),
  JSON.stringify(styleSamples.styles, null, 2) + "\n",
);
await Bun.write(
  path.resolve(root, "../stylus/stylus.js"),
  stylusScript.trim() + "\n",
);
await Bun.write(
  path.resolve(root, "../stylus/runtime.toml"),
  [
    "# Set extension_dir to a matching unpacked Stylus package before applying",
    "# These disabled styles use the extension API to populate IndexedDB",
    "[browser]",
    'executable_name = "stylus-runtime-sample"',
    "",
    "[extension_settings]",
    'files = ["stylus.json"]',
    "",
    "[[extension_settings.runtime]]",
    'id = "clngdbkpkpeebahjckkjfobafhncgmne"',
    'page = "manage.html"',
    'script = "stylus.js"',
    "timeout_seconds = 120",
    "[extension_settings.runtime.files]",
    'styles = "styles.json"',
    "",
  ].join("\n"),
);
reviews.at(-1)!.runtime_sample = {
  config: "stylus/runtime.toml",
  schemas: styleSamples.schemas,
  excluded_fields: styleSamples.excluded,
};

source = await read("SponsorBlock", "src/config.ts");
const sponsor = evaluate(
  snippet(source, "const syncDefaults =", "const Config ="),
  "({sync: syncDefaults, local: localDefaults})",
  {
    CompileConfig: JSON.parse(
      await read("SponsorBlock", "config.json.example"),
    ),
    SegmentListDefaultTab: { Segments: 0 },
    NoticeVisibilityMode: { FadedForAutoSkip: 3 },
    CategorySkipOption: { AutoSkip: 2, ManualSkip: 1, ShowOverlay: 0 },
  },
);
const sponsorCustom = structuredClone(sponsor);
for (const [name, value] of Object.entries(sponsorCustom.sync)) {
  if (name.endsWith("Keybind") && value && typeof value === "object") {
    sponsorCustom.sync[name] = {
      code: null,
      ctrl: false,
      alt: false,
      shift: false,
      ...value,
    };
  }
}
sponsorCustom.sync.categorySelections = [
  "sponsor",
  "selfpromo",
  "exclusive_access",
  "interaction",
  "intro",
  "outro",
  "preview",
  "hook",
  "music_offtopic",
  "poi_highlight",
  "filler",
  "chapter",
].map((name, index) => ({ name, option: index % 3 }));
sponsorCustom.sync.invidiousInstances = ["invidious.example.org"];
const profileId = "sample-profile";
sponsorCustom.local.skipProfiles[profileId] = {
  name: "Sample channel overrides",
  categorySelections: structuredClone(sponsorCustom.sync.categorySelections),
  showAutogeneratedChapters: false,
  showCreatorChapters: true,
  autoSkipOnMusicVideos: false,
  skipNonMusicOnlyOnYoutubeMusic: null,
  muteSegments: true,
  fullVideoSegments: null,
  manualSkipOnFullVideo: false,
  minDuration: 1.5,
};
sponsorCustom.local.channelSkipProfileIDs["UC0000000000000000000000"] =
  profileId;
sponsorCustom.local.skipRules = [
  {
    predicate: {
      kind: "operator",
      operator: "and",
      displayInverted: false,
      left: {
        kind: "check",
        attribute: "category",
        operator: "==",
        value: "sponsor",
      },
      right: {
        kind: "operator",
        operator: "or",
        displayInverted: false,
        left: {
          kind: "check",
          attribute: "time.duration",
          operator: "<=",
          value: 45,
        },
        right: {
          kind: "check",
          attribute: "video.title",
          operator: "~i=",
          value: "intro|overview",
        },
      },
    },
    skipOption: 1,
    comments: [
      "Hypothetical sample: manually skip short sponsor segments or overview videos",
    ],
  },
];
await save(
  "sponsorblock",
  "SponsorBlock",
  "https://github.com/ajayyy/SponsorBlock",
  "mnjggcdmjocbbbhaepdhchncahnbgone",
  sponsorCustom,
  sponsor,
  [
    "src/config.ts",
    "src/types.ts",
    "src/utils/skipRule.type.ts",
    "config.json.example",
  ],
  [
    "Separate sync and local defaults",
    "Null identity, payment, and navigation fields",
    "Category numeric enums and string opacity values",
    "Migration moves submissions from sync to local",
    "Shortcut objects contain optional modifier fields",
  ],
  [
    "Identity, license, counters, and submission records are application-owned state",
    "Sample channel identifier is hypothetical; runtime caches and submissions use empty defaults",
    "Source revision may differ from the store package",
  ],
);
reviews
  .at(-1)!
  .source_urls.push(
    "https://github.com/ajayyy/maze-utils/blob/6b5b5be38f9db60d9bab70c1d3c196504578e552/src/config.ts",
  );

reviews.at(-1)!.field_schemas = {
  ...checkSchemas(await read("SponsorBlock", "src/config.ts"), {
    CustomConfiguration: Object.values(sponsorCustom.local.skipProfiles),
  }),
  ...checkSchemas(await read("SponsorBlock", "maze-utils/src/config.ts"), {
    Keybind: Object.entries(sponsorCustom.sync)
      .filter(
        ([key, value]) =>
          key.endsWith("Keybind") && value && typeof value === "object",
      )
      .map(([, value]) => value as Settings),
  }),
};

// Obsidian's serializer groups the in-memory settings into native sync keys
source = await read("obsidian-clipper", "src/utils/storage-utils.ts");
const clipper = evaluate(
  snippet(
    source,
    "const defaultSettings: Settings =",
    "// Update migration version",
  ),
  "defaultSettings",
);
const serializer = snippet(
  source,
  "export async function saveSettings",
  "export async function setLegacyMode",
);
function serializeClipper(settings: Settings): Settings {
  const record: { data?: Settings } = {};
  return evaluate(
    `let generalSettings = ${JSON.stringify(settings)};\n${serializer}\nsaveSettings();`,
    "record.data",
    {
      record,
      browser: {
        storage: {
          sync: {
            set(value: Settings) {
              record.data = value;
              return Promise.resolve();
            },
          },
        },
      },
    },
  );
}
const clipperCustom = structuredClone(clipper);
clipperCustom.vaults = ["Sample vault"];
clipperCustom.readerSettings.fonts = ["Georgia", "system-ui"];
clipperCustom.readerSettings.defaultFont = "Georgia";
clipperCustom.readerSettings.customCss = ".reader-content { hyphens: auto; }\n";
clipperCustom.propertyTypes = [
  { name: "source", type: "text", defaultValue: "{{url}}" },
];
clipperCustom.providers = [
  {
    id: "sample-provider",
    name: "Sample local provider",
    baseUrl: "http://127.0.0.1:11434",
    apiKey: "",
    apiKeyRequired: false,
    presetId: "ollama",
  },
];
clipperCustom.models = [
  {
    id: "sample-model",
    providerId: "sample-provider",
    providerModelId: "sample-model",
    name: "Sample model",
    enabled: false,
  },
];
clipperCustom.interpreterModel = "sample-model";
clipperCustom.defaultPromptContext = "{{content}}";
const clipperSync = serializeClipper(clipperCustom);
clipperSync.migrationVersion = 1;
const templates = [
  "create",
  "append-specific",
  "append-daily",
  "prepend-specific",
  "prepend-daily",
  "overwrite",
].map((behavior, index) => ({
  id: `sample-${index}`,
  name: `Sample ${behavior}`,
  behavior,
  noteNameFormat: "{{title}}",
  path: "Clippings",
  vault: "Sample vault",
  context: "{{content}}",
  noteContentFormat: "# {{title}}\n\n{{content}}\n",
  properties: [
    { id: `property-${index}`, name: "source", value: "{{url}}", type: "text" },
  ],
  triggers: ["https://example.com/"],
}));
clipperSync.template_list = templates.map((template) => template.id);
for (const template of templates) {
  const compressed = LZString.compressToUTF16(JSON.stringify(template));
  const chunks = [];
  for (let offset = 0; offset < compressed.length; offset += 8000)
    chunks.push(compressed.slice(offset, offset + 8000));
  if (
    JSON.stringify(template) !== LZString.decompressFromUTF16(chunks.join(""))
  )
    throw new Error("Template compression is not reversible");
  clipperSync[`template_${template.id}`] = chunks;
}
const clipperLocal = {
  history: [],
  ratings: [],
  language: "en",
  propertiesCollapsed: false,
  lastSelectedVault: "Sample vault",
  clipperIframeWidth: 480,
  clipperIframeHeight: 700,
  debugMode: false,
  isHighlighterMode: false,
  highlights: {},
  domains: {},
  provider_presets: { version: "sample" },
};
await save(
  "obsidian-clipper",
  "obsidian-clipper",
  "https://github.com/obsidianmd/obsidian-clipper",
  "cnjifjpddelmedmihgijeibhnjfabmlf",
  { sync: clipperSync, local: clipperLocal },
  { sync: serializeClipper(clipper) },
  [
    "src/utils/storage-utils.ts",
    "src/managers/template-manager.ts",
    "src/types/types.ts",
    "src/utils/i18n.ts",
    "src/utils/iframe-resize.ts",
  ],
  [
    "In-memory settings grouped into multiple sync keys by upstream serializer",
    "Templates use LZ-String UTF-16 chunks inside native JSON arrays",
    "Six template behaviors, vault names, property objects, and multiline formats",
    "Model/provider references must share IDs",
    "Sync quotas measure UTF-8 JSON bytes, not JavaScript string length",
    "History, language, window size, and UI state live in local storage",
  ],
  [
    "Interpreter is disabled and credentials are empty",
    "Chunk templates are preencoded; project nested mutation cannot decompress UTF-16 chunk arrays",
    "Large templates can exceed Chromium sync item quotas before the extension's character-count warning",
    "Runtime history and highlight contents use empty initial values",
    "Source revision may differ from the store package",
  ],
);
const templatePath = path.resolve(root, "../obsidian-clipper/templates.json");
await Bun.write(templatePath, JSON.stringify(templates, null, 2) + "\n");
reviews.at(-1)!.compressed_templates = {
  plain_sample: "obsidian-clipper/templates.json",
  encoding: "lz-string-utf16-chunks",
  keys: (clipperSync.template_list as string[]).map((id) => `template_${id}`),
};

reviews.at(-1)!.field_schemas = checkSchemas(
  await read("obsidian-clipper", "src/types/types.ts"),
  {
    Settings: [clipperCustom],
    ReaderSettings: [clipperCustom.readerSettings],
    Template: templates,
    Property: templates.flatMap((template) => template.properties),
    Provider: clipperCustom.providers,
    ModelConfig: clipperCustom.models,
    PropertyType: clipperCustom.propertyTypes,
  },
);

const res = await resConfiguration(path.join(repos, "RES"));
await save(
  "res",
  "RES",
  "https://github.com/honestbleeps/Reddit-Enhancement-Suite",
  "kbmfpngjjgdllneeigpgjifpgocmfgmb",
  { local: res.local },
  { local: res.defaults },
  res.files,
  [
    `${res.optionCount} persistent options including ${res.hostCount} generated host switches`,
    "Every registered module has explicit enablement in a separate blob",
    "Option values use nested value wrappers under module-prefixed keys",
    "25 positional tables include complete hypothetical rows and five-member key arrays",
    "Nested post/comment filter builders carry version, IDs, and optional propagation",
    "Every registered post, comment, and browsing filter case includes its source-declared condition fields",
    "Undefined cells in upstream macro arrays serialize as null",
  ],
  [
    `${res.skipped.length} buttons perform actions and have no persistent values`,
    "Runtime caches, saved posts, and vote histories are application data",
    "Sample accounts use hypothetical usernames and empty passwords",
    "Legacy modules and hosts may be inert on modern Reddit or defunct sites",
    "Source revision may differ from the store package",
  ],
);
reviews.at(-1)!.positional_tables = res.tables;
reviews.at(-1)!.filter_cases = res.filterCases;

const ghostery = await ghosteryConfiguration(path.join(repos, "ghostery"));
await save(
  "ghostery",
  "ghostery",
  "https://github.com/ghostery/ghostery-extension",
  "mlomiejdfkolichcflejclcbmpeaniij",
  { local: ghostery.local, sync: ghostery.sync },
  { local: ghostery.defaults, sync: ghostery.sync },
  ghostery.files,
  [
    "Migration version 6 accompanies the nested options object",
    "Hybrids record declarations become dictionaries, not schema objects",
    "Tracker exceptions, pause metadata, custom lists, selectors, and all bundled distraction IDs",
    "Local-only keys excluded from the sync snapshot; revision remains in both areas",
    "Managed policy has six fields and overrides editable options through browser policy",
  ],
  [
    "Hypothetical tracker/filter IDs demonstrate schemas without matching real tracker data",
    "Custom remote list is disabled and uses example.com",
    "Managed policy is a separate TOML recipe; writing its local cache does not enforce policy",
    "Reporting databases, downloaded rules, server config, counters, and caches are runtime state",
    "Source revision may differ from the store package",
  ],
);
reviews.at(-1)!.record_schemas = ghostery.recordSchemas;
reviews.at(-1)!.managed_policy_sample = "ghostery/managed.toml";
const managedDir = path.resolve(root, "../ghostery");
await Bun.write(
  path.join(managedDir, "managed.json"),
  JSON.stringify(
    {
      "3rdparty": {
        extensions: { mlomiejdfkolichcflejclcbmpeaniij: ghostery.managed },
      },
    },
    null,
    2,
  ) + "\n",
);
await Bun.write(
  path.join(managedDir, "managed.toml"),
  [
    "# Render with tweezle policy render examples/extensions/ghostery/managed.toml",
    "# Install the rendered policy through the browser's managed policy mechanism",
    "[browser]",
    'executable_name = "ghostery-policy-sample"',
    "",
    '[browser.chrome.managed_policies."3rdparty".extensions.mlomiejdfkolichcflejclcbmpeaniij]',
    ...Object.entries(ghostery.managed).map(
      ([key, value]) => `${key} = ${JSON.stringify(value)}`,
    ),
    "",
  ].join("\n"),
);

await Bun.write(
  path.join(root, "reviews.json"),
  JSON.stringify({ reviews }, null, 2) + "\n",
);
console.log(`Generated ${reviews.length} source-pinned samples`);

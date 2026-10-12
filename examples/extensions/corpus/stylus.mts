import path from "node:path";
import { parse } from "@babel/parser";

type Values = Record<string, any>;

export async function stylusStyles(repository: string) {
  const styles: Values[] = [
    {
      enabled: false,
      id: 1,
      name: "Hypothetical sectioned style",
      customName: "Sample sectioned style",
      _id: "b9cde177-0d1e-4cd3-8b7a-5fe25be934e8",
      _rev: 1,
      author: "Sample author",
      preferScheme: "dark",
      overridden: false,
      exclusions: ["*://example.com/private/*"],
      inclusions: ["*://example.net/sample/*"],
      updatable: false,
      url: "https://example.com/style",
      installationUrl: "https://example.com/style",
      updateUrl: "https://example.com/style.css",
      md5Url: "https://example.com/style.md5",
      installDate: 1,
      updateDate: 1,
      originalDigest: "",
      originalMd5: "",
      sections: [
        {
          code: "body { --sample-color: #663399; }\n",
          domains: ["example.com"],
          urls: ["https://example.org/sample"],
          urlPrefixes: ["https://example.net/sample/"],
          regexps: ["https://example\\.com/.*"],
        },
      ],
    },
    {
      enabled: false,
      id: 2,
      name: "Hypothetical UserCSS style",
      _id: "3856fbe8-b4cb-4709-a8ac-b29c8855b79c",
      _rev: 1,
      updatable: false,
      sections: [],
      sourceCode:
        '/* ==UserStyle==\n@name Hypothetical UserCSS style\n@namespace https://example.org/sample\n@version 1.0.0\n==/UserStyle== */\n@-moz-document domain("example.org") { body { color: var(--accent); padding: var(--spacing); opacity: var(--opacity); } }\n',
      usercssData: {
        name: "Hypothetical UserCSS style",
        namespace: "https://example.org/sample",
        version: "1.0.0",
        author: "Sample author",
        description: "Hypothetical disabled style",
        homepageURL: "https://example.org/sample",
        license: "CC0-1.0",
        preprocessor: "default",
        supportURL: "https://example.org/support",
        updateURL: "https://example.org/sample.user.css",
        vars: {
          accent: {
            type: "color",
            label: "Accent",
            name: "accent",
            value: "#663399",
            default: "#663399",
          },
          spacing: {
            type: "range",
            label: "Spacing",
            name: "spacing",
            value: 12,
            default: 8,
            min: 0,
            max: 24,
            step: 1,
            units: "px",
          },
          opacity: {
            type: "select",
            label: "Opacity",
            name: "opacity",
            value: "soft",
            default: "soft",
            options: [
              { name: "soft", label: "Soft", value: "0.8", isDefault: true },
              { name: "full", label: "Full", value: "1", isDefault: false },
            ],
          },
          label: {
            type: "text",
            label: "Label",
            name: "label",
            value: "Sample",
            default: "Sample",
          },
          enabled: {
            type: "checkbox",
            label: "Enable extra",
            name: "enabled",
            value: "0",
            default: "0",
          },
          count: {
            type: "number",
            label: "Count",
            name: "count",
            value: 3,
            default: 3,
            min: 0,
            max: 10,
            step: 1,
            units: "",
          },
        },
      },
    },
  ];
  const source = await Bun.file(
    path.join(repository, "src/.types.d.ts"),
  ).text();
  const ast = parse(source, { sourceType: "module", plugins: ["typescript"] });
  const schemas: Record<string, string[]> = {};
  for (const node of ast.program.body) {
    if (node.type === "TSInterfaceDeclaration")
      schemas[node.id.name] = node.body.body
        .map((member: any) => member.key?.name)
        .filter(Boolean);
  }
  const examples: Record<string, Values[]> = {
    StyleObj: styles,
    StyleSection: styles.flatMap((style) => style.sections),
    UsercssData: [styles[1].usercssData],
    UsercssVarBase: Object.values(styles[1].usercssData.vars),
    UsercssVarRange: [styles[1].usercssData.vars.spacing],
    UsercssVarSelect: [styles[1].usercssData.vars.opacity],
  };
  const excluded: Record<string, string[]> = {
    StyleObj: ["_usw"],
    StyleSection: ["_empty"],
  };
  for (const [schema, values] of Object.entries(examples)) {
    if (!schemas[schema]) throw new Error(`Missing Stylus schema ${schema}`);
    for (const key of schemas[schema]) {
      if (
        !values.some((value) => Object.hasOwn(value, key)) &&
        !excluded[schema]?.includes(key)
      )
        throw new Error(`Unconfigured Stylus ${schema}.${key}`);
    }
  }
  return {
    styles,
    schemas: Object.fromEntries(
      Object.keys(examples).map((key) => [key, schemas[key]]),
    ),
    excluded,
  };
}

// This payload runs inside the real extension page, where API owns IndexedDB
export const stylusScript = `
const call = async (path, ...args) => {
  const response = await chrome.runtime.sendMessage({data: {method: "invokeAPI", path, args}, TDM: 1});
  if (!response || response.error) throw new Error(response?.error?.message || "Stylus API returned no response");
  return response.data;
};
const result = await call("styles.importMany", input.files.styles);
if (result.length !== input.files.styles.length || result.some(item => item.err)) throw new Error("Stylus could not import all styles");
await call("styles.setOrder", {main: input.files.styles.map(style => style._id), prio: []});
const actual = await call("styles.getAll");
for (const wanted of input.files.styles) {
  const found = actual.find(style => style._id === wanted._id);
  if (!found || found.enabled !== wanted.enabled || !found.sections?.length) throw new Error("Stylus did not persist sample " + wanted._id);
}
`;

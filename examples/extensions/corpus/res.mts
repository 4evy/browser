import path from "node:path";
import {
  declarations,
  literal,
  parseSource,
  property,
  visit,
  type Node,
} from "./literals.mts";

type Values = Record<string, any>;

export async function resConfiguration(repository: string) {
  const files = ["lib/modules/index.js", "lib/modules/hosts/index.js"];
  const read = async (file: string) => {
    if (!files.includes(file)) files.push(file);
    return parseSource(await Bun.file(path.join(repository, file)).text());
  };
  const local: Values = { "RES.modulePrefs": {} };
  const tables: { module: string; option: string; fields: string[] }[] = [];
  const skipped: string[] = [];
  const descriptors = new Map<
    string,
    { node: Node; bindings: Map<string, Node> }
  >();
  let optionCount = 0;
  let hostCount = 0;
  function options(module: string, node: Node, bindings: Map<string, Node>) {
    const values = (local[`RESoptions.${module}`] ??= {});
    for (const member of node.properties) {
      if (member.type === "SpreadElement" && module === "showImages") continue;
      if (member.type !== "ObjectProperty")
        throw new Error(`${module}: unexpected options member`);
      const name = member.key.name ?? member.key.value;
      const value = property(member.value, "value");
      if (!value) {
        const type = property(member.value, "type");
        if (
          (!type || literal(type, bindings) !== "button") &&
          `${module}.${name}` !== "presets.resetToFactory"
        )
          throw new Error(
            `${module}.${name}: unexplained nonpersistent option`,
          );
        skipped.push(`${module}.${name}`);
        continue;
      }
      try {
        values[name] = { value: literal(value, bindings) };
      } catch (error) {
        throw new Error(`${module}.${name}: ${error}`);
      }
      optionCount++;
      descriptors.set(`${module}.${name}`, { node: member.value, bindings });
      const fields = property(member.value, "fields");
      if (fields) {
        tables.push({
          module,
          option: name,
          fields: fields.elements.map((field: Node) =>
            literal(property(field, "key")!, bindings),
          ),
        });
      }
    }
  }
  const registry = await read("lib/modules/index.js");
  for (const entry of registry.program.body) {
    if (entry.type !== "ImportDeclaration") continue;
    const file = `lib/modules/${entry.source.value.slice(2)}.js`;
    const ast = await read(file);
    const bindings = declarations(ast);
    const module = bindings.get("module");
    if (module?.type !== "NewExpression" || module.callee.name !== "Module")
      throw new Error(`${file}: missing Module declaration`);
    const id: string = literal(module.arguments[0], bindings);
    let disabled = false;
    let always = false;
    visit(ast, (node) => {
      if (
        node.type !== "AssignmentExpression" ||
        node.left.type !== "MemberExpression" ||
        node.left.object.name !== "module"
      )
        return;
      const name = node.left.property.name;
      if (name === "disabledByDefault")
        disabled = literal(node.right, bindings);
      if (name === "alwaysEnabled") always = literal(node.right, bindings);
      if (name === "options") options(id, node.right, bindings);
    });
    local["RES.modulePrefs"][id] = always || !disabled;
  }
  const hosts = await read("lib/modules/hosts/index.js");
  for (const entry of hosts.program.body) {
    if (entry.type !== "ImportDeclaration") continue;
    const ast = await read(
      `lib/modules/hosts/${entry.source.value.slice(2)}.js`,
    );
    const bindings = declarations(ast);
    const host = ast.program.body.find(
      (node: Node) => node.type === "ExportDefaultDeclaration",
    )?.declaration;
    if (host?.type !== "NewExpression" || host.callee.name !== "Host")
      throw new Error(`Invalid host ${entry.source.value}`);
    const id: string = literal(host.arguments[0], bindings);
    if (["defaultImage", "defaultVideo", "defaultAudio"].includes(id)) continue;
    local["RESoptions.showImages"][`display_${id}`] = { value: true };
    optionCount++;
    hostCount++;
    const extra = property(host.arguments[1], "options");
    if (extra) options("showImages", extra, bindings);
  }
  const defaults = structuredClone(local);
  // Hypothetical values exercise every positional table field without credentials
  const rows: Record<string, any[]> = {
    "accountSwitcher.accounts": ["sample-user", "", false],
    "commentDepth.subredditCommentDepths": ["testing", "4", "50"],
    "commentTools.macros": [
      "Sample macro",
      "Line one\nLine two: café",
      "sample",
      [77, true, false, false, false],
    ],
    "customToggles.toggle": ["sample-toggle", false, "Sample toggle"],
    "filteReddit.allowNSFW": ["testing", "everywhere"],
    "filteReddit.keywords": [
      "sample phrase",
      "include",
      "testing",
      "exception",
    ],
    "filteReddit.subreddits": ["sample_subreddit"],
    "filteReddit.comments": ["sample phrase", "include", "testing"],
    "filteReddit.users": ["sample-user"],
    "filteReddit.domains": ["example.com", "include", "testing"],
    "filteReddit.flair": ["sample flair", "include", "testing"],
    "hover.instances": ["sample-hover", false],
    "messageMenu.links": ["Sample", "https://example.com/"],
    "multiredditNavbar.sectionLinks": ["Sample", "https://example.com/"],
    "notifications.notificationTypes": ["sample-notification", false, false],
    "penaltyBox.features": ["showImages", "showViewImagesTab", false, "0"],
    "profileNavigator.sectionLinks": ["Sample", "https://example.com/"],
    "stylesheet.loadStylesheets": [
      "https://example.com/sample.css",
      "include",
      "testing",
      "sample-toggle",
    ],
    "stylesheet.snippets": [
      "body { --sample-color: #663399; }\n",
      "include",
      "testing",
      "sample-toggle",
    ],
    "stylesheet.bodyClasses": [
      "sample-class",
      "include",
      "testing",
      "sample-toggle",
    ],
    "subRedditTagger.subReddits": ["testing", "", "Sample tag"],
    "userTagger.presetTags": ["Sample tag", "none"],
    "usernameHider.perAccountDisplayText": [
      "sample-user",
      "Sample display name",
    ],
    "voteEnhancements.userDefinedLinkColoration": ["100", "#663399"],
    "voteEnhancements.userDefinedCommentColoration": ["100", "#663399"],
  };
  for (const table of tables) {
    const key = `${table.module}.${table.option}`;
    const row = rows[key];
    if (!row || row.length !== table.fields.length)
      throw new Error(`Incomplete table sample ${key}`);
    const { node, bindings } = descriptors.get(key)!;
    for (let index = 0; index < row.length; index++) {
      const field = property(node, "fields")!.elements[index];
      const allowed = property(field, "values");
      if (
        allowed?.type === "ArrayExpression" &&
        !literal(allowed, bindings).some(
          (item: Values) => item.value === row[index],
        )
      )
        throw new Error(`${key}: invalid ${table.fields[index]} value`);
    }
    local[`RESoptions.${table.module}`][table.option].value.push(row);
    defaults[`RESoptions.${table.module}`][table.option].value.push(row);
  }
  const cases = "lib/modules/filteReddit/cases.js";
  await read(cases);
  const filterCases: Record<string, Values[]> = {};
  for (const context of ["post", "comment", "browse"]) {
    const base = `lib/modules/filteReddit/${context}Cases`;
    const registry = await read(`${base}/index.js`);
    filterCases[context] = [];
    for (const entry of registry.program.body) {
      if (entry.type !== "ExportNamedDeclaration" || !entry.source) continue;
      const ast = await read(
        path.posix.normalize(`${base}/${entry.source.value}.js`),
      );
      const bindings = declarations(ast);
      for (const specifier of entry.specifiers) {
        const type = specifier.exported.name;
        const definition = ast.program.body.find(
          (node: Node) =>
            node.type === "ExportNamedDeclaration" &&
            node.declaration?.id?.name === specifier.local.name,
        )?.declaration;
        if (definition?.type !== "ClassDeclaration")
          throw new Error(`Missing filter case ${type}`);
        const defaults = definition.body.body.find(
          (node: Node) => node.static && node.key?.name === "defaultConditions",
        )?.value;
        let conditions: Values = {};
        if (type === "postAfter")
          conditions = { patt: "2026-01-01T00:00:00.000Z" };
        else if (type === "currentLocation")
          conditions = { patt: "/r/testing" };
        else if (type === "toggle")
          conditions = { toggleName: "sample-toggle" };
        else if (defaults) conditions = literal(defaults, bindings);
        else if (definition.superClass?.name === "PatternCase")
          conditions = { patt: "sample", fullMatch: false };
        if (Object.hasOwn(conditions, "patt") && conditions.patt === "")
          conditions.patt = "sample";
        if (type === "currentSub") conditions.patt = "testing";
        if (type === "currentMulti")
          conditions = { user: "sample-user", name: "sample-multi" };
        if (type === "loggedInAs") conditions.loggedInAs = "sample-user";
        if (type === "dow") conditions.days = ["Mon", "Fri"];
        if (type === "hasExpando") conditions.types = ["image", "video"];
        const fields = definition.body.body.find(
          (node: Node) => node.static && node.key?.name === "fields",
        )?.value;
        visit(fields, (node) => {
          if (
            node.type !== "ObjectProperty" ||
            (node.key.name ?? node.key.value) !== "id"
          )
            return;
          const key = literal(node.value, bindings);
          if (!Object.hasOwn(conditions, key))
            throw new Error(`${type}: missing condition field ${key}`);
        });
        filterCases[context].push({ type, ...conditions });
      }
    }
  }
  for (const suffix of ["P", "C"]) {
    local["RESoptions.filteReddit"][`customFilters${suffix}`].value = [
      {
        note: "Hypothetical nested filter",
        ver: 3,
        body: {
          type: "group",
          op: "all",
          of: [
            {
              type: "group",
              op: "any",
              of: [
                ...filterCases[suffix === "P" ? "post" : "comment"],
                ...filterCases.browse,
              ],
            },
          ],
        },
        id: `customFilter-sample-${suffix}`,
        opts: {
          ondemand: true,
          name: `Sample ${suffix}`,
          ...(suffix === "C" ? { propagate: false } : {}),
        },
      },
    ];
    defaults["RESoptions.filteReddit"][`customFilters${suffix}`] =
      structuredClone(
        local["RESoptions.filteReddit"][`customFilters${suffix}`],
      );
  }
  files.push(
    "lib/core/options/storage.js",
    "lib/core/modules/storage.js",
    "lib/core/options/table.js",
    "lib/modules/filteReddit/browseCases/CurrentSub.js",
    "lib/modules/filteReddit/Case.js",
  );
  return {
    local,
    defaults,
    files,
    tables,
    skipped,
    optionCount,
    hostCount,
    filterCases,
  };
}
